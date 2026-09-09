package portfolio

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

type DailyBar struct {
	Time, End       time.Time
	Open, High, Low float64
	Close, Volume   float64
}

type Rank struct {
	Symbol              protocolv2.Symbol `json:"symbol"`
	Rank                int               `json:"rank"`
	Score               float64           `json:"score"`
	Return              float64           `json:"return"`
	Volatility          float64           `json:"volatility"`
	StopDistance        float64           `json:"stop_distance"`
	EntryEligible       bool              `json:"entry_eligible"`
	MaxEntryPrice       float64           `json:"max_entry_price,omitempty"`
	TargetWeightPercent float64           `json:"target_weight_percent,omitempty"`
}

type Rebalance struct {
	FillTime        time.Time                  `json:"fill_time"`
	RegimeOn        bool                       `json:"regime_on"`
	ReplaceAll      bool                       `json:"replace_all,omitempty"`
	BTCAboveEMA     bool                       `json:"btc_above_ema"`
	PositiveBreadth float64                    `json:"positive_breadth"`
	Targets         []Rank                     `json:"targets"`
	Retain          map[protocolv2.Symbol]bool `json:"-"`
	Ranking         []Rank                     `json:"ranking"`
}

func AggregateDaily(minutes []model.Candle) ([]DailyBar, error) {
	if len(minutes) == 0 {
		return nil, nil
	}
	var out []DailyBar
	for _, c := range minutes {
		if c.OpenTime.Location() != time.UTC {
			c.OpenTime = c.OpenTime.UTC()
		}
		day := c.OpenTime.Truncate(24 * time.Hour)
		if len(out) == 0 || !out[len(out)-1].Time.Equal(day) {
			out = append(out, DailyBar{Time: day, End: day.Add(24 * time.Hour), Open: c.Open, High: c.High, Low: c.Low, Close: c.Close, Volume: c.Volume})
			continue
		}
		bar := &out[len(out)-1]
		if c.High > bar.High {
			bar.High = c.High
		}
		if c.Low < bar.Low {
			bar.Low = c.Low
		}
		bar.Close, bar.Volume = c.Close, bar.Volume+c.Volume
	}
	for i, bar := range out {
		if !positive(bar.Open) || !positive(bar.High) || !positive(bar.Low) || !positive(bar.Close) || bar.Low > bar.High || (i > 0 && !bar.Time.After(out[i-1].Time)) {
			return nil, fmt.Errorf("portfolio: invalid daily bar")
		}
	}
	return out, nil
}

func RelativeStrengthRebalances(bars map[protocolv2.Symbol][]DailyBar, cfg RelativeStrengthConfig, evaluationStart, evaluationEnd time.Time) ([]Rebalance, error) {
	if err := cfg.RegimeMode.Validate(); err != nil {
		return nil, err
	}
	if err := cfg.EntryMode.Validate(); err != nil {
		return nil, err
	}
	btc := bars["BTCUSDT"]
	if len(btc) == 0 {
		return nil, fmt.Errorf("portfolio: BTCUSDT is required for regime detection")
	}
	var events []Rebalance
	for _, fill := range btc {
		if fill.Time.Before(evaluationStart) || !fill.Time.Before(evaluationEnd) || fill.Time.Weekday() != cfg.RebalanceWeekday {
			continue
		}
		btcHistory := completedBefore(btc, fill.Time)
		if len(btcHistory) < cfg.BTCEMADays {
			continue
		}
		btcEMA := emaClose(btcHistory[len(btcHistory)-cfg.BTCEMADays:], cfg.BTCEMADays)
		btcAbove := btcHistory[len(btcHistory)-1].Close > btcEMA
		ranking := make([]Rank, 0, len(bars))
		positiveReturns := 0
		for symbol, series := range bars {
			history := completedBefore(series, fill.Time)
			rank, ok := score(symbol, history, cfg)
			if !ok {
				continue
			}
			if rank.Return > 0 {
				positiveReturns++
			}
			ranking = append(ranking, rank)
		}
		if len(ranking) == 0 {
			continue
		}
		sort.Slice(ranking, func(i, j int) bool {
			if ranking[i].Score != ranking[j].Score {
				return ranking[i].Score > ranking[j].Score
			}
			if ranking[i].Return != ranking[j].Return {
				return ranking[i].Return > ranking[j].Return
			}
			return ranking[i].Symbol < ranking[j].Symbol
		})
		for i := range ranking {
			ranking[i].Rank = i + 1
		}
		breadth := float64(positiveReturns) / float64(len(ranking))
		regime := regimeEnabled(cfg.RegimeMode, btcAbove, breadth >= cfg.MinPositiveBreadth)
		event := Rebalance{FillTime: fill.Time, RegimeOn: regime, BTCAboveEMA: btcAbove, PositiveBreadth: breadth, Ranking: ranking, Retain: map[protocolv2.Symbol]bool{}}
		if regime {
			for _, ranked := range ranking {
				if ranked.Rank <= cfg.TopK && ranked.EntryEligible {
					event.Targets = append(event.Targets, ranked)
				}
				if ranked.Rank <= cfg.ExitRank {
					event.Retain[ranked.Symbol] = true
				}
			}
		}
		events = append(events, event)
	}
	return events, nil
}

// SlowMomentumRebalances ranks symbols solely by their completed 60d or 90d
// return. Each weekly event replaces the whole target set; when no return is
// positive it emits an empty target set and the engine remains in cash.
func SlowMomentumRebalances(bars map[protocolv2.Symbol][]DailyBar, cfg SlowMomentumConfig, evaluationStart, evaluationEnd time.Time) ([]Rebalance, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	calendar := bars["BTCUSDT"]
	if len(calendar) == 0 {
		return nil, fmt.Errorf("portfolio: BTCUSDT is required for rebalance calendar")
	}
	events := make([]Rebalance, 0)
	for _, fill := range calendar {
		if fill.Time.Before(evaluationStart) || !fill.Time.Before(evaluationEnd) || fill.Time.Weekday() != cfg.RebalanceWeekday {
			continue
		}
		ranking := make([]Rank, 0, len(bars))
		for symbol, series := range bars {
			rank, ok := slowMomentumReturn(symbol, completedBefore(series, fill.Time), cfg.LookbackDays)
			if ok && rank.EntryEligible {
				ranking = append(ranking, rank)
			}
		}
		sortSlowMomentumRanks(ranking)
		event := Rebalance{FillTime: fill.Time, RegimeOn: true, ReplaceAll: true, Retain: map[protocolv2.Symbol]bool{}, Ranking: ranking}
		for _, rank := range ranking {
			if rank.Rank > cfg.TopK {
				break
			}
			rank.TargetWeightPercent = 100 / float64(cfg.TopK)
			event.Targets = append(event.Targets, rank)
			event.Retain[rank.Symbol] = true
		}
		events = append(events, event)
	}
	return events, nil
}

// DefensiveSlowMomentumRebalances ranks completed raw returns each Monday as
// slow momentum does, but evaluates a completed-day BTC and breadth guard at
// every daily open. A failed guard emits a cash event immediately; a healthy
// non-Monday emits nothing, so existing holdings are left untouched.
func DefensiveSlowMomentumRebalances(bars map[protocolv2.Symbol][]DailyBar, cfg DefensiveSlowMomentumConfig, evaluationStart, evaluationEnd time.Time) ([]Rebalance, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	calendar := bars["BTCUSDT"]
	if len(calendar) == 0 {
		return nil, fmt.Errorf("portfolio: BTCUSDT is required for defensive slow momentum")
	}
	events := make([]Rebalance, 0)
	for _, fill := range calendar {
		if fill.Time.Before(evaluationStart) || !fill.Time.Before(evaluationEnd) {
			continue
		}
		btcHistory := completedBefore(calendar, fill.Time)
		event := Rebalance{FillTime: fill.Time, ReplaceAll: true, Retain: map[protocolv2.Symbol]bool{}}
		if len(btcHistory) < cfg.BTCEMADays {
			events = append(events, event)
			continue
		}
		btcEMA := emaClose(btcHistory[len(btcHistory)-cfg.BTCEMADays:], cfg.BTCEMADays)
		event.BTCAboveEMA = btcHistory[len(btcHistory)-1].Close > btcEMA
		ranking := make([]Rank, 0, len(bars))
		positiveReturns := 0
		for symbol, series := range bars {
			rank, ok := slowMomentumReturn(symbol, completedBefore(series, fill.Time), cfg.SlowMomentum.LookbackDays)
			if !ok {
				continue
			}
			if rank.EntryEligible {
				positiveReturns++
			}
			ranking = append(ranking, rank)
		}
		if len(ranking) > 0 {
			event.PositiveBreadth = float64(positiveReturns) / float64(len(ranking))
			sortSlowMomentumRanks(ranking)
			event.Ranking = ranking
		}
		event.RegimeOn = event.BTCAboveEMA && event.PositiveBreadth >= cfg.MinPositiveBreadth
		if !event.RegimeOn {
			events = append(events, event)
			continue
		}
		if fill.Time.Weekday() != cfg.SlowMomentum.RebalanceWeekday {
			continue
		}
		for _, rank := range ranking {
			if !rank.EntryEligible || len(event.Targets) == cfg.SlowMomentum.TopK {
				break
			}
			rank.TargetWeightPercent = 100 / float64(cfg.SlowMomentum.TopK)
			event.Targets = append(event.Targets, rank)
			event.Retain[rank.Symbol] = true
		}
		events = append(events, event)
	}
	return events, nil
}

func slowMomentumReturn(symbol protocolv2.Symbol, history []DailyBar, lookback int) (Rank, bool) {
	if len(history) < lookback+1 {
		return Rank{}, false
	}
	last, base := history[len(history)-1].Close, history[len(history)-1-lookback].Close
	if !positive(last) || !positive(base) {
		return Rank{}, false
	}
	ret := last/base - 1
	return Rank{Symbol: symbol, Score: protocolv2.RoundMetric(ret), Return: protocolv2.RoundMetric(ret), EntryEligible: ret > 0}, true
}

func sortSlowMomentumRanks(ranking []Rank) {
	sort.Slice(ranking, func(i, j int) bool {
		if ranking[i].Return != ranking[j].Return {
			return ranking[i].Return > ranking[j].Return
		}
		return ranking[i].Symbol < ranking[j].Symbol
	})
	for i := range ranking {
		ranking[i].Rank = i + 1
	}
}

func regimeEnabled(mode RegimeMode, btcAboveEMA, breadthPositive bool) bool {
	switch mode.normalized() {
	case RegimeModeBoth:
		return btcAboveEMA && breadthPositive
	case RegimeModeBTCEMA:
		return btcAboveEMA
	case RegimeModeBreadth:
		return breadthPositive
	case RegimeModeNone:
		return true
	default:
		return false
	}
}

func completedBefore(in []DailyBar, t time.Time) []DailyBar {
	i := sort.Search(len(in), func(i int) bool { return !in[i].Time.Before(t) })
	return in[:i]
}

func score(symbol protocolv2.Symbol, h []DailyBar, cfg RelativeStrengthConfig) (Rank, bool) {
	need := maxInt(cfg.ReturnLookbackDays+1, cfg.VolatilityDays+1, cfg.ATRDays+1)
	if cfg.EntryMode.normalized() == EntryModeTrendPullback {
		need = maxInt(need, cfg.PullbackEMADays)
	}
	if len(h) < need {
		return Rank{}, false
	}
	last := h[len(h)-1].Close
	base := h[len(h)-1-cfg.ReturnLookbackDays].Close
	if !positive(last) || !positive(base) {
		return Rank{}, false
	}
	ret := last/base - 1
	returns := make([]float64, 0, cfg.VolatilityDays)
	for i := len(h) - cfg.VolatilityDays; i < len(h); i++ {
		if i == 0 || !positive(h[i-1].Close) {
			return Rank{}, false
		}
		returns = append(returns, math.Log(h[i].Close/h[i-1].Close))
	}
	vol := standardDeviation(returns)
	if vol <= 0 || !finite(vol) {
		return Rank{}, false
	}
	atr := averageTrueRange(h[len(h)-cfg.ATRDays-1:])
	if atr <= 0 || !finite(atr) {
		return Rank{}, false
	}
	rank := Rank{Symbol: symbol, Score: protocolv2.RoundMetric(ret / vol), Return: protocolv2.RoundMetric(ret), Volatility: protocolv2.RoundMetric(vol), StopDistance: protocolv2.RoundPrice(cfg.StopATR * atr), EntryEligible: true}
	if cfg.EntryMode.normalized() == EntryModeTrendPullback {
		entryEMA := emaClose(h[len(h)-cfg.PullbackEMADays:], cfg.PullbackEMADays)
		maxEntry := protocolv2.RoundPrice(entryEMA + cfg.MaxEntryDistanceATR*atr)
		rank.EntryEligible = last > entryEMA && last <= maxEntry
		rank.MaxEntryPrice = maxEntry
	}
	return rank, true
}

func emaClose(in []DailyBar, period int) float64 {
	value, alpha := in[0].Close, 2/float64(period+1)
	for i := 1; i < len(in); i++ {
		value += alpha * (in[i].Close - value)
	}
	return value
}

func averageTrueRange(in []DailyBar) float64 {
	if len(in) < 2 {
		return 0
	}
	total := 0.0
	for i := 1; i < len(in); i++ {
		tr := math.Max(in[i].High-in[i].Low, math.Max(math.Abs(in[i].High-in[i-1].Close), math.Abs(in[i].Low-in[i-1].Close)))
		total += tr
	}
	return total / float64(len(in)-1)
}

func standardDeviation(in []float64) float64 {
	if len(in) < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range in {
		mean += v
	}
	mean /= float64(len(in))
	variance := 0.0
	for _, v := range in {
		d := v - mean
		variance += d * d
	}
	return math.Sqrt(variance / float64(len(in)-1))
}

func maxInt(values ...int) int {
	out := values[0]
	for _, v := range values[1:] {
		if v > out {
			out = v
		}
	}
	return out
}
