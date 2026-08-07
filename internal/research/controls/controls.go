// Package controls provides deterministic, protocol-v2-compatible benchmark
// simulations. Controls are intentionally separate from the candidate strategy
// registry: they are comparison baselines, not selectable hypotheses.
package controls

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const (
	CashCode          protocolv2.StrategyCode    = "cash-control"
	BuyAndHoldCode    protocolv2.StrategyCode    = "buy-and-hold"
	BTCBuyAndHoldCode protocolv2.StrategyCode    = "btc-buy-and-hold"
	EMA200Code        protocolv2.StrategyCode    = "ema200-long-cash"
	RandomCode        protocolv2.StrategyCode    = "random-frequency-matched"
	Version           protocolv2.StrategyVersion = "v1"
)

// Input identifies one independently accounted control series.
type Input struct {
	Symbol     protocolv2.Symbol
	Timeframe  protocolv2.Timeframe
	Candles    []execution.Candle
	ScoreStart time.Time
}

func (in Input) validate() error {
	if err := in.Symbol.Validate(); err != nil {
		return err
	}
	if err := in.Timeframe.Validate(); err != nil {
		return err
	}
	if len(in.Candles) < 2 {
		return fmt.Errorf("controls: at least two candles are required")
	}
	if in.ScoreStart.IsZero() {
		in.ScoreStart = in.Candles[0].Time
	}
	if in.ScoreStart.Before(in.Candles[0].Time) || in.ScoreStart.After(in.Candles[len(in.Candles)-1].Time) {
		return fmt.Errorf("controls: score start must be inside the candle range")
	}
	if len(in.scoredCandles()) < 2 {
		return fmt.Errorf("controls: scored range requires at least two candles")
	}
	return nil
}

// Result keeps the exact engine evidence and control identity needed by a
// report writer. Its timestamps are always those supplied in Input.
type Result struct {
	Control protocolv2.StrategyRef
	Input   Input
	Entries []execution.CloseConfirmedSignal
	Exits   []execution.CloseConfirmedExitSignal
	Engine  execution.Result
}

func ref(code protocolv2.StrategyCode) protocolv2.StrategyRef {
	return protocolv2.StrategyRef{Code: code, Version: Version}
}

// Cash produces an equity curve with no trade attempts.
func Cash(config execution.Config, input Input) (Result, error) {
	return run(config, ref(CashCode), input, nil, nil)
}

// BuyAndHold opens on the next available open after the aligned range starts
// and remains long until the engine's fold-end policy. The engine retains its
// normal cost, sizing, and mark-to-market accounting.
func BuyAndHold(config execution.Config, input Input) (Result, error) {
	entry, err := holdEntry(ref(BuyAndHoldCode), input, "buy-and-hold-entry")
	if err != nil {
		return Result{}, err
	}
	return run(config, ref(BuyAndHoldCode), input, []execution.CloseConfirmedSignal{entry}, nil)
}

// BTCBuyAndHold is BuyAndHold over explicitly supplied BTC candles. Supplying
// BTC input rather than deriving it from the asset under test prevents an
// accidental proxy benchmark.
func BTCBuyAndHold(config execution.Config, btc Input) (Result, error) {
	entry, err := holdEntry(ref(BTCBuyAndHoldCode), btc, "btc-buy-and-hold-entry")
	if err != nil {
		return Result{}, err
	}
	return run(config, ref(BTCBuyAndHoldCode), btc, []execution.CloseConfirmedSignal{entry}, nil)
}

// EMA200 is a causal long/cash regime control: a close above its EMA enters on
// the following open; a close at or below it exits on the following open.
func EMA200(config execution.Config, input Input) (Result, error) {
	if err := input.validate(); err != nil {
		return Result{}, err
	}
	if len(input.Candles) <= 200 {
		return Result{}, fmt.Errorf("controls: EMA200 requires more than 200 candles")
	}
	entries := make([]execution.CloseConfirmedSignal, 0)
	exits := make([]execution.CloseConfirmedExitSignal, 0)
	long := false
	const alpha = 2.0 / 201.0
	ema := 0.0
	for i, candle := range input.Candles {
		if i < 199 {
			continue
		}
		if i == 199 {
			for _, seed := range input.Candles[:200] {
				ema += seed.Close
			}
			ema /= 200
		} else {
			ema = alpha*candle.Close + (1-alpha)*ema
		}
		above := candle.Close > ema
		if candle.Time.Before(input.scoreStart()) {
			continue
		}
		if above && !long {
			stop := causalProtectiveStop(candle.Close)
			entries = append(entries, execution.CloseConfirmedSignal{
				SignalID: fmt.Sprintf("ema200-entry-%06d", i), Strategy: ref(EMA200Code), Symbol: input.Symbol,
				Timeframe: input.Timeframe, SourceCandleTime: candle.Time, Side: execution.SideLong, Stop: stop,
				Diagnostics: map[string]float64{"ema200": protocolv2.RoundPrice(ema)},
			})
			long = true
		} else if !above && long {
			exits = append(exits, execution.CloseConfirmedExitSignal{
				SignalID: fmt.Sprintf("ema200-exit-%06d", i), Strategy: ref(EMA200Code), Symbol: input.Symbol,
				SourceCandleTime: candle.Time, Diagnostics: map[string]float64{"ema200": protocolv2.RoundPrice(ema)},
			})
			long = false
		}
	}
	return run(config, ref(EMA200Code), input, entries, exits)
}

// Activity defines random-control entry activity. EntryCount and HorizonBars
// are derived from a reference strategy's close-confirmed entry signals, so a
// random control has the same number of attempts per observed interval.
type Activity struct {
	EntryCount  int
	HorizonBars int
}

// MatchActivity derives an activity rate from reference entry signals within
// the aligned candle range. Duplicate timestamps count once.
func MatchActivity(candles []execution.Candle, entries []execution.CloseConfirmedSignal) (Activity, error) {
	if len(candles) < 2 {
		return Activity{}, fmt.Errorf("controls: at least two candles are required")
	}
	index := make(map[int64]struct{}, len(candles))
	for _, candle := range candles[:len(candles)-1] {
		index[candle.Time.UnixNano()] = struct{}{}
	}
	seen := make(map[int64]struct{}, len(entries))
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return Activity{}, err
		}
		t := entry.SourceCandleTime.UnixNano()
		if _, eligible := index[t]; eligible {
			seen[t] = struct{}{}
		}
	}
	return Activity{EntryCount: len(seen), HorizonBars: len(candles) - 1}, nil
}

// RandomEntries chooses exactly Activity.EntryCount distinct source candles
// from the aligned range using the supplied seed. It matches the reference
// attempt frequency (entries per eligible bar), while randomizing timing.
func RandomEntries(input Input, activity Activity, seed int64) ([]execution.CloseConfirmedSignal, error) {
	if err := input.validate(); err != nil {
		return nil, err
	}
	scored := input.scoredCandles()
	if activity.HorizonBars != len(scored)-1 || activity.EntryCount < 0 || activity.EntryCount > activity.HorizonBars {
		return nil, fmt.Errorf("controls: activity must match the aligned eligible candle range")
	}
	indexes := rand.New(rand.NewSource(seed)).Perm(activity.HorizonBars)[:activity.EntryCount]
	sort.Ints(indexes)
	entries := make([]execution.CloseConfirmedSignal, 0, len(indexes))
	for _, index := range indexes {
		candle := scored[index]
		stop := causalProtectiveStop(candle.Close)
		entries = append(entries, execution.CloseConfirmedSignal{
			SignalID: fmt.Sprintf("random-entry-%06d", index), Strategy: ref(RandomCode), Symbol: input.Symbol,
			Timeframe: input.Timeframe, SourceCandleTime: candle.Time, Side: execution.SideLong, Stop: stop,
			Diagnostics: map[string]float64{"seed": float64(seed)},
		})
	}
	return entries, nil
}

// Random executes the seeded, frequency-matched signals through the same
// engine configuration as candidates and other controls.
func Random(config execution.Config, input Input, activity Activity, seed int64) (Result, error) {
	entries, err := RandomEntries(input, activity, seed)
	if err != nil {
		return Result{}, err
	}
	return run(config, ref(RandomCode), input, entries, nil)
}

func run(config execution.Config, control protocolv2.StrategyRef, input Input, entries []execution.CloseConfirmedSignal, exits []execution.CloseConfirmedExitSignal) (Result, error) {
	if err := input.validate(); err != nil {
		return Result{}, err
	}
	engine, err := execution.NewEngine(config)
	if err != nil {
		return Result{}, err
	}
	scored := input.scoredCandles()
	entries = filterEntries(entries, input.scoreStart())
	exits = filterExits(exits, input.scoreStart())
	result, err := engine.RunWithExits(scored, entries, exits)
	if err != nil {
		return Result{}, err
	}
	return Result{Control: control, Input: input, Entries: entries, Exits: exits, Engine: result}, nil
}

func holdEntry(control protocolv2.StrategyRef, input Input, id string) (execution.CloseConfirmedSignal, error) {
	if err := input.validate(); err != nil {
		return execution.CloseConfirmedSignal{}, err
	}
	scored := input.scoredCandles()
	stop := causalProtectiveStop(scored[0].Close)
	return execution.CloseConfirmedSignal{
		SignalID: id, Strategy: control, Symbol: input.Symbol, Timeframe: input.Timeframe,
		SourceCandleTime: scored[0].Time, Side: execution.SideLong, Stop: stop,
	}, nil
}

// causalProtectiveStop satisfies the common risk contract without inspecting
// any candle after the signal. The notional cap, rather than this distant stop,
// controls passive benchmark allocation.
func causalProtectiveStop(reference float64) float64 {
	// The common fill contract requires a positive price-rounded stop. For
	// low-priced assets such as SHIB, the distant proportional stop would round
	// to zero at protocol precision.
	return math.Max(0.00000001, protocolv2.RoundPrice(reference*0.000001))
}

func (in Input) scoreStart() time.Time {
	if in.ScoreStart.IsZero() {
		return in.Candles[0].Time
	}
	return in.ScoreStart
}

func (in Input) scoredCandles() []execution.Candle {
	start := in.scoreStart()
	index := sort.Search(len(in.Candles), func(i int) bool { return !in.Candles[i].Time.Before(start) })
	return in.Candles[index:]
}

func filterEntries(entries []execution.CloseConfirmedSignal, start time.Time) []execution.CloseConfirmedSignal {
	out := entries[:0]
	for _, entry := range entries {
		if !entry.SourceCandleTime.Before(start) {
			out = append(out, entry)
		}
	}
	return out
}

func filterExits(exits []execution.CloseConfirmedExitSignal, start time.Time) []execution.CloseConfirmedExitSignal {
	out := exits[:0]
	for _, exit := range exits {
		if !exit.SourceCandleTime.Before(start) {
			out = append(out, exit)
		}
	}
	return out
}
