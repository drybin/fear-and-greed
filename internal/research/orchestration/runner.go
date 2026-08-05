package orchestration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/infrastructure/csvdata"
	"github.com/drybin/fear-and-greed/internal/research/candidates"
	"github.com/drybin/fear-and-greed/internal/research/controls"
	"github.com/drybin/fear-and-greed/internal/research/eligibility"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/metrics"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const stressSlippageBPS = 15

// CandleStore loads primary-source OHLCV candles for a symbol.
type CandleStore interface {
	Path(protocolv2.Symbol) string
	Load(protocolv2.Symbol) ([]model.Candle, error)
}

// DirCandleStore reads SYMBOL.csv files from a directory (fetch-data layout).
type DirCandleStore struct {
	Dir string
}

func (s DirCandleStore) Path(symbol protocolv2.Symbol) string {
	return filepath.Join(s.Dir, string(symbol)+".csv")
}

func (s DirCandleStore) Load(symbol protocolv2.Symbol) ([]model.Candle, error) {
	return csvdata.LoadKlines(s.Path(symbol))
}

// UnitArtifact is the retained JSON payload for one symbol inside a unit.
type UnitArtifact struct {
	Unit           Unit                        `json:"unit"`
	Symbol         protocolv2.Symbol           `json:"symbol,omitempty"`
	Survivorship   string                      `json:"survivorship_warning,omitempty"`
	Metrics        metrics.Summary             `json:"metrics"`
	TradeCount     int                         `json:"trade_count"`
	RejectionCount int                         `json:"rejection_count"`
	RealizedCash   float64                     `json:"realized_cash_diagnostic"`
	FinalEquity    float64                     `json:"final_equity"`
	Control        string                      `json:"control,omitempty"`
	EngineAuditLen int                         `json:"engine_audit_events"`
	Trades         []execution.TradeState      `json:"trades"`
	Equity         []execution.EquitySnapshot  `json:"equity"`
	Rejections     []execution.SignalRejection `json:"rejections"`
	Audit          []execution.AuditEvent      `json:"audit"`
}

// UnitResult is the typed, machine-readable artifact consumed by train-only
// selection, reporting, freeze validation, and final decision gates.
type UnitResult struct {
	Unit                Unit           `json:"unit"`
	SurvivorshipWarning string         `json:"survivorship_warning"`
	Symbols             []UnitArtifact `json:"symbols"`
	AggregateTradeCount int            `json:"aggregate_trade_count"`
	AggregateRejections int            `json:"aggregate_rejections"`
	SymbolsEvaluated    int            `json:"symbols_evaluated"`
}

// InProcessRunner evaluates candidates and controls with the protocol-v2 engine.
type InProcessRunner struct {
	Manifest manifest.Manifest
	Candles  CandleStore
	Adapters map[string]candidates.Adapter
}

// NewInProcessRunner builds adapters from the closed core candidate set.
func NewInProcessRunner(m manifest.Manifest, candles CandleStore) (*InProcessRunner, error) {
	if candles == nil {
		return nil, fmt.Errorf("orchestration: candle store is required")
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if err := manifest.ValidateCoreStrategyCodes(m.Strategies); err != nil {
		return nil, err
	}
	byCode := map[protocolv2.StrategyCode]candidates.Adapter{}
	for _, adapter := range candidates.Core() {
		byCode[adapter.Metadata().Ref.Code] = adapter
	}
	adapters := map[string]candidates.Adapter{}
	for _, strategy := range m.Strategies {
		adapter, ok := byCode[strategy.Ref.Code]
		if !ok {
			return nil, fmt.Errorf("orchestration: no adapter for %s", strategy.Ref)
		}
		if err := validateAdapterManifest(strategy, adapter); err != nil {
			return nil, err
		}
		adapters[strategy.Ref.String()] = adapter
	}
	return &InProcessRunner{Manifest: m, Candles: candles, Adapters: adapters}, nil
}

// Run evaluates one planned unit. Development never passes a holdout range.
func (r *InProcessRunner) Run(ctx context.Context, unit Unit) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := unit.Range.Validate(); err != nil {
		return nil, err
	}
	cfg, err := r.engineConfig(unit)
	if err != nil {
		return nil, err
	}
	if unit.Control != "" {
		return r.runControl(unit, cfg)
	}
	return r.runCandidate(unit, cfg)
}

func (r *InProcessRunner) engineConfig(unit Unit) (execution.Config, error) {
	slippage := r.Manifest.Execution.SlippageBPS
	if unit.Cost == "stress" {
		slippage = stressSlippageBPS
	}
	tf := r.Manifest.Strategies[0].Timeframe
	lookup := unit.Strategy
	if unit.Control != "" && unit.ReferenceStrategy.Code != "" {
		lookup = unit.ReferenceStrategy
	}
	for _, strategy := range r.Manifest.Strategies {
		if strategy.Ref.String() == lookup.String() {
			tf = strategy.Timeframe
			break
		}
	}
	interval, err := timeframeDuration(tf)
	if err != nil {
		return execution.Config{}, err
	}
	return execution.Config{
		InitialEquity:       r.Manifest.Risk.InitialEquity,
		Interval:            interval,
		CommissionBPS:       r.Manifest.Execution.CommissionBPS,
		SlippageBPS:         slippage,
		RiskPerTradePercent: r.Manifest.Risk.RiskPerTradePercent,
		MaxNotionalPercent:  r.Manifest.Risk.MaxNotionalPercent,
		CostProfile:         unit.Cost,
		GapPolicy:           execution.GapPolicy(r.Manifest.Execution.GapPolicy),
		CloseAtFoldEnd:      true,
	}, nil
}

func (r *InProcessRunner) runCandidate(unit Unit, cfg execution.Config) ([]byte, error) {
	adapter, ok := r.Adapters[unit.Strategy.String()]
	if !ok {
		return nil, fmt.Errorf("orchestration: missing adapter %s", unit.Strategy)
	}
	warmupBars := 0
	for _, strategy := range r.Manifest.Strategies {
		if strategy.Ref.String() == unit.Strategy.String() {
			warmupBars = strategy.WarmupBars
			break
		}
	}
	symbols := r.symbolsForUnit(unit)
	symbolResults := make([]UnitArtifact, 0, len(symbols))
	trades, rejects := 0, 0
	for _, symbol := range symbols {
		modelCandles, err := r.Candles.Load(symbol)
		if err != nil {
			return nil, fmt.Errorf("orchestration: load %s: %w", symbol, err)
		}
		window := windowCandles(modelCandles, unit.Range, warmupBars, cfg.Interval)
		scored := rangeCandles(modelCandles, unit.Range)
		if len(window) < 2 || len(scored) < 2 {
			return nil, fmt.Errorf("orchestration: insufficient eligible candles for %s in %s", symbol, unit.Key())
		}
		signals, err := adapter.Signals(symbol, window, unit.Candidate)
		if err != nil {
			return nil, err
		}
		engine, err := execution.NewEngine(cfg)
		if err != nil {
			return nil, err
		}
		filteredSignals := filterSignals(signals, unit.Range)
		result, err := engine.Run(toEngineCandles(scored), filteredSignals)
		if err != nil {
			return nil, err
		}
		summary, err := metrics.Calculate(metrics.Input{
			InitialEquity: cfg.InitialEquity,
			Equity:        result.Equity,
			Trades:        result.Trades,
			RiskByTrade:   riskByTrade(result.Trades, filteredSignals),
		})
		if err != nil {
			return nil, fmt.Errorf("orchestration: metrics for %s in %s: %w", symbol, unit.Key(), err)
		}
		trades += summary.TradeCount
		rejects += len(result.Rejections)
		symbolResults = append(symbolResults, UnitArtifact{
			Unit: unit, Symbol: symbol, Survivorship: eligibility.SurvivorshipWarning,
			Metrics: summary, TradeCount: summary.TradeCount, RejectionCount: len(result.Rejections),
			RealizedCash: result.RealizedCash, FinalEquity: lastEquity(result), EngineAuditLen: len(result.Audit),
			Trades: result.Trades, Equity: result.Equity, Rejections: result.Rejections, Audit: result.Audit,
		})
	}
	if len(symbolResults) == 0 {
		return nil, fmt.Errorf("orchestration: no eligible candles for %s", unit.Key())
	}
	payload := UnitResult{Unit: unit, SurvivorshipWarning: eligibility.SurvivorshipWarning, Symbols: symbolResults, AggregateTradeCount: trades, AggregateRejections: rejects, SymbolsEvaluated: len(symbolResults)}
	return json.Marshal(payload)
}

func (r *InProcessRunner) runControl(unit Unit, cfg execution.Config) ([]byte, error) {
	symbols := r.symbolsForUnit(unit)
	if len(symbols) == 0 {
		return nil, fmt.Errorf("orchestration: universe is empty")
	}
	if unit.Control == string(controls.BTCBuyAndHoldCode) {
		symbols = onlyBTC(symbols)
		if len(symbols) == 0 {
			return nil, fmt.Errorf("orchestration: BTCUSDT is required for the BTC benchmark")
		}
	}

	results := make([]UnitArtifact, 0, len(symbols))
	totalTrades, totalRejects := 0, 0
	for _, symbol := range symbols {
		modelCandles, err := r.Candles.Load(symbol)
		if err != nil {
			return nil, err
		}
		warmupBars := 0
		if unit.Control == string(controls.EMA200Code) {
			warmupBars = 200
		}
		window := windowCandles(modelCandles, unit.Range, warmupBars, cfg.Interval)
		aggregated := aggregateCandles(window, cfg.Interval)
		if len(aggregated) < 2 {
			return nil, fmt.Errorf("orchestration: insufficient control candles for %s", symbol)
		}
		tf, err := timeframeForDuration(cfg.Interval)
		if err != nil {
			return nil, err
		}
		input := controls.Input{Symbol: symbol, Timeframe: tf, Candles: aggregated, ScoreStart: unit.Range.Start}
		var result controls.Result
		switch unit.Control {
		case string(controls.CashCode):
			result, err = controls.Cash(cfg, input)
		case string(controls.BuyAndHoldCode):
			result, err = controls.BuyAndHold(cfg, input)
		case string(controls.BTCBuyAndHoldCode):
			result, err = controls.BTCBuyAndHold(cfg, input)
		case string(controls.EMA200Code):
			result, err = controls.EMA200(cfg, input)
		case string(controls.RandomCode):
			activity, activityErr := r.referenceActivity(unit, symbol, modelCandles, aggregated)
			if activityErr != nil {
				return nil, activityErr
			}
			result, err = controls.Random(cfg, input, activity, int64(r.Manifest.Seed))
		default:
			return nil, fmt.Errorf("orchestration: unknown control %q", unit.Control)
		}
		if err != nil {
			return nil, err
		}
		summary, err := metrics.Calculate(metrics.Input{InitialEquity: cfg.InitialEquity, Equity: result.Engine.Equity, Trades: result.Engine.Trades})
		if err != nil {
			return nil, fmt.Errorf("orchestration: control metrics for %s: %w", symbol, err)
		}
		totalTrades += summary.TradeCount
		totalRejects += len(result.Engine.Rejections)
		results = append(results, UnitArtifact{
			Unit: unit, Symbol: symbol, Survivorship: eligibility.SurvivorshipWarning, Metrics: summary,
			TradeCount: summary.TradeCount, RejectionCount: len(result.Engine.Rejections), RealizedCash: result.Engine.RealizedCash,
			FinalEquity: lastEquity(result.Engine), Control: unit.Control, EngineAuditLen: len(result.Engine.Audit),
			Trades: result.Engine.Trades, Equity: result.Engine.Equity, Rejections: result.Engine.Rejections, Audit: result.Engine.Audit,
		})
	}
	return json.Marshal(UnitResult{Unit: unit, SurvivorshipWarning: eligibility.SurvivorshipWarning, Symbols: results, AggregateTradeCount: totalTrades, AggregateRejections: totalRejects, SymbolsEvaluated: len(results)})
}

// PreflightDevelopment validates fingerprints and writes eligibility output.
func PreflightDevelopment(m manifest.Manifest, outputDir string, candles CandleStore) (protocolv2.SHA256Hex, error) {
	if err := m.Validate(); err != nil {
		return "", err
	}
	if err := manifest.ValidateCoreStrategyCodes(m.Strategies); err != nil {
		return "", err
	}
	if candles == nil {
		return "", fmt.Errorf("orchestration: candle store is required for preflight")
	}
	root := experimentRoot(outputDir, m)
	if err := os.MkdirAll(protocolv2.ReportDir(root), 0o755); err != nil {
		return "", err
	}
	type row struct {
		Symbol    protocolv2.Symbol         `json:"symbol"`
		Inventory eligibility.FileInventory `json:"inventory"`
		HashMatch bool                      `json:"hash_match"`
		Expected  protocolv2.SHA256Hex      `json:"expected_sha256"`
	}
	rows := make([]row, 0, len(m.Universe.Symbols))
	var hashMaterial string
	for _, snap := range m.Universe.Symbols {
		path := candles.Path(snap.Symbol)
		inv, err := eligibility.InventoryFile(path)
		if err != nil {
			return "", err
		}
		if inv.SHA256 != snap.CandleSHA256 {
			return "", fmt.Errorf("orchestration: candle fingerprint mismatch for %s: got %s want %s", snap.Symbol, inv.SHA256, snap.CandleSHA256)
		}
		if !inv.CoreUsable() {
			return "", fmt.Errorf("orchestration: candle file for %s failed core quality checks", snap.Symbol)
		}
		rows = append(rows, row{Symbol: snap.Symbol, Inventory: inv, HashMatch: true, Expected: snap.CandleSHA256})
		hashMaterial += string(inv.SHA256) + "\n"
	}
	report := map[string]any{
		"survivorship_warning": eligibility.SurvivorshipWarning,
		"universe":             m.Universe.Name,
		"provenance":           m.Universe.Provenance,
		"files":                rows,
	}
	if _, err := writeJSONAtomic(filepath.Join(protocolv2.ReportDir(root), "eligibility.json"), report); err != nil {
		return "", err
	}
	return digest([]byte(hashMaterial)), nil
}

func timeframeDuration(tf protocolv2.Timeframe) (time.Duration, error) {
	switch tf {
	case "1m":
		return time.Minute, nil
	case "3m":
		return 3 * time.Minute, nil
	case "5m":
		return 5 * time.Minute, nil
	case "15m":
		return 15 * time.Minute, nil
	case "30m":
		return 30 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "2h":
		return 2 * time.Hour, nil
	case "4h":
		return 4 * time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("orchestration: unsupported timeframe %q", tf)
	}
}

func windowCandles(all []model.Candle, rang protocolv2.TimeRange, warmupBars int, interval time.Duration) []model.Candle {
	start := rang.Start
	if warmupBars > 0 && interval > 0 {
		start = rang.Start.Add(-time.Duration(warmupBars) * interval)
	}
	out := make([]model.Candle, 0, len(all))
	for _, c := range all {
		t := c.OpenTime.UTC()
		if t.Before(start) || !t.Before(rang.End) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func toEngineCandles(in []model.Candle) []execution.Candle {
	out := make([]execution.Candle, len(in))
	for i, c := range in {
		out[i] = execution.Candle{Time: c.OpenTime.UTC(), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close}
	}
	return out
}

func filterSignals(signals []execution.CloseConfirmedSignal, rang protocolv2.TimeRange) []execution.CloseConfirmedSignal {
	out := make([]execution.CloseConfirmedSignal, 0, len(signals))
	for _, s := range signals {
		if rang.ContainsInstant(s.SourceCandleTime) {
			out = append(out, s)
		}
	}
	return out
}

func rangeCandles(all []model.Candle, rang protocolv2.TimeRange) []model.Candle {
	out := make([]model.Candle, 0, len(all))
	for _, candle := range all {
		if rang.ContainsInstant(candle.OpenTime.UTC()) {
			out = append(out, candle)
		}
	}
	return out
}

func (r *InProcessRunner) symbolsForUnit(unit Unit) []protocolv2.Symbol {
	if len(unit.Symbols) > 0 {
		return append([]protocolv2.Symbol(nil), unit.Symbols...)
	}
	out := make([]protocolv2.Symbol, 0, len(r.Manifest.Universe.Symbols))
	for _, snapshot := range r.Manifest.Universe.Symbols {
		out = append(out, snapshot.Symbol)
	}
	return out
}

func onlyBTC(symbols []protocolv2.Symbol) []protocolv2.Symbol {
	for _, symbol := range symbols {
		if symbol == "BTCUSDT" {
			return []protocolv2.Symbol{symbol}
		}
	}
	return nil
}

func (r *InProcessRunner) referenceActivity(unit Unit, symbol protocolv2.Symbol, all []model.Candle, controlCandles []execution.Candle) (controls.Activity, error) {
	adapter, ok := r.Adapters[unit.ReferenceStrategy.String()]
	if !ok || unit.ReferenceCandidate == "" {
		return controls.Activity{}, fmt.Errorf("orchestration: random control requires a selected reference strategy and candidate")
	}
	warmup := adapter.Metadata().WarmupBars
	interval, err := timeframeDuration(adapter.Metadata().Timeframe)
	if err != nil {
		return controls.Activity{}, err
	}
	window := windowCandles(all, unit.Range, warmup, interval)
	signals, err := adapter.Signals(symbol, window, unit.ReferenceCandidate)
	if err != nil {
		return controls.Activity{}, err
	}
	return controls.MatchActivity(scoredExecutionCandles(controlCandles, unit.Range.Start), filterSignals(signals, unit.Range))
}

func scoredExecutionCandles(candles []execution.Candle, start time.Time) []execution.Candle {
	index := 0
	for index < len(candles) && candles[index].Time.Before(start) {
		index++
	}
	return candles[index:]
}

func aggregateCandles(candles []model.Candle, interval time.Duration) []execution.Candle {
	if len(candles) == 0 {
		return nil
	}
	out := make([]execution.Candle, 0, len(candles))
	for _, source := range candles {
		bucket := source.OpenTime.UTC().Truncate(interval)
		if len(out) == 0 || !out[len(out)-1].Time.Equal(bucket) {
			out = append(out, execution.Candle{Time: bucket, Open: source.Open, High: source.High, Low: source.Low, Close: source.Close})
			continue
		}
		current := &out[len(out)-1]
		if source.High > current.High {
			current.High = source.High
		}
		if source.Low < current.Low {
			current.Low = source.Low
		}
		current.Close = source.Close
	}
	return out
}

func timeframeForDuration(interval time.Duration) (protocolv2.Timeframe, error) {
	switch interval {
	case time.Minute:
		return "1m", nil
	case 3 * time.Minute:
		return "3m", nil
	case 5 * time.Minute:
		return "5m", nil
	case 15 * time.Minute:
		return "15m", nil
	case 30 * time.Minute:
		return "30m", nil
	case time.Hour:
		return "1h", nil
	case 2 * time.Hour:
		return "2h", nil
	case 4 * time.Hour:
		return "4h", nil
	case 24 * time.Hour:
		return "1d", nil
	default:
		return "", fmt.Errorf("orchestration: unsupported interval %s", interval)
	}
}

func riskByTrade(trades []execution.TradeState, signals []execution.CloseConfirmedSignal) map[string]float64 {
	bySignal := make(map[string]execution.CloseConfirmedSignal, len(signals))
	for _, signal := range signals {
		bySignal[signal.SignalID] = signal
	}
	out := make(map[string]float64, len(trades))
	for _, trade := range trades {
		signal, ok := bySignal[trade.Entry.SignalID]
		if !ok {
			continue
		}
		risk := (trade.Entry.Price - signal.Stop) * trade.Entry.Quantity
		if risk > 0 {
			out[trade.TradeID] = risk
		}
	}
	return out
}

func validateAdapterManifest(config manifest.Strategy, adapter candidates.Adapter) error {
	metadata := adapter.Metadata()
	if config.Ref != metadata.Ref || config.Timeframe != metadata.Timeframe || config.WarmupBars != metadata.WarmupBars {
		return fmt.Errorf("orchestration: manifest strategy %s metadata does not match adapter %s", config.Ref, metadata.Ref)
	}
	grid := adapter.Grid()
	if len(config.Grid) != len(grid) {
		return fmt.Errorf("orchestration: manifest strategy %s grid size does not match adapter", config.Ref)
	}
	byID := make(map[protocolv2.ParameterCandidateID]map[string]any, len(grid))
	for _, candidate := range grid {
		byID[candidate.ID] = candidate.Values
	}
	for _, candidate := range config.Grid {
		expected, ok := byID[candidate.ID]
		if !ok || !sameJSON(expected, candidate.Values) {
			return fmt.Errorf("orchestration: manifest candidate %s/%s does not match adapter values", config.Ref, candidate.ID)
		}
	}
	return nil
}

func sameJSON(left, right any) bool {
	l, lerr := json.Marshal(left)
	r, rerr := json.Marshal(right)
	return lerr == nil && rerr == nil && string(l) == string(r)
}

func lastEquity(result execution.Result) float64 {
	if len(result.Equity) == 0 {
		return 0
	}
	return result.Equity[len(result.Equity)-1].TotalEquity
}
