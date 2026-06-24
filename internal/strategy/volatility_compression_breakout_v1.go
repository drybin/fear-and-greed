package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	VCB1EMA200Period    = 200
	VCB1EMA50Period     = 50
	VCB1EMA200RiseBars  = 20
	VCB1ATRPeriod       = 14
	VCB1SetupLifetime   = 24
	VCB1TP1Fraction     = 0.5
	VCB1Minutes1H       = 60
)

// VCBStopMode selects stop placement (optional test mode).
type VCBStopMode int

const (
	VCBStopCompressionLow VCBStopMode = iota
	VCBStopATR2BelowEntry
)

// VolatilityCompressionBreakoutV1Params tunes VCB (sweep grid).
type VolatilityCompressionBreakoutV1Params struct {
	CompressionWindow int
	RangeWindow       int
	ATRCompression    float64
	BreakoutExpansion float64
	TrendFilter       NR7TrendFilter
	StopMode          VCBStopMode
}

func DefaultVolatilityCompressionBreakoutV1Params() VolatilityCompressionBreakoutV1Params {
	return VolatilityCompressionBreakoutV1Params{
		CompressionWindow: 100,
		RangeWindow:       10,
		ATRCompression:    0.6,
		BreakoutExpansion: 1.5,
		TrendFilter:       NR7TrendCloseEMA200,
		StopMode:          VCBStopCompressionLow,
	}
}

type vcbState int

const (
	vcbIdle vcbState = iota
	vcbWaitBreakout
	vcbInPosition
)

type vcbSetup struct {
	compHigh      float64
	compLow       float64
	compATR       float64
	setupIdx      int
	setupTime     time.Time
	expireIdx     int
	entryIdx      int
	entryPrice    float64
	entryTime     time.Time
	stopLevel     float64
	tp1Level      float64
	tp2Level      float64
	tp1Done       bool
	tp1Hit        bool
	tp2Hit        bool
	coins         float64
	entryContext  map[string]float64
	tradeEvents   []TradeEvent
}

func isATRCompressionBar(atr []float64, j, window int, factor float64) bool {
	if window < 2 || j < window-1 {
		return false
	}
	atrNow := atr[j]
	if atrNow <= 0 {
		return false
	}
	start := j - window + 1
	minATR := atr[start]
	sum := 0.0
	for k := start; k <= j; k++ {
		if atr[k] <= 0 {
			return false
		}
		if atr[k] < minATR {
			minATR = atr[k]
		}
		sum += atr[k]
	}
	if atrNow > minATR+1e-9 {
		return false
	}
	avg := sum / float64(window)
	return atrNow < avg*factor
}

func compressionRangeHL(candles []model.Candle, end, window int) (high, low float64, ok bool) {
	if window < 1 || end < window-1 {
		return 0, 0, false
	}
	start := end - window + 1
	high = candles[start].High
	low = candles[start].Low
	for k := start + 1; k <= end; k++ {
		if candles[k].High > high {
			high = candles[k].High
		}
		if candles[k].Low < low {
			low = candles[k].Low
		}
	}
	if high <= low {
		return 0, 0, false
	}
	return high, low, true
}

// SimulateVolatilityCompressionBreakoutV1 runs 1H ATR compression range breakout long.
func SimulateVolatilityCompressionBreakoutV1(candles []model.Candle) SimulationReport {
	return SimulateVolatilityCompressionBreakoutV1WithParams(candles, DefaultVolatilityCompressionBreakoutV1Params())
}

// SimulateVolatilityCompressionBreakoutV1WithParams runs with configurable sweep parameters.
func SimulateVolatilityCompressionBreakoutV1WithParams(candles []model.Candle, p VolatilityCompressionBreakoutV1Params) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if p.CompressionWindow < 2 {
		p.CompressionWindow = 100
	}
	if p.RangeWindow < 1 {
		p.RangeWindow = 10
	}
	if p.ATRCompression <= 0 {
		p.ATRCompression = 0.6
	}
	if p.BreakoutExpansion <= 0 {
		p.BreakoutExpansion = 1.5
	}

	min1h := VCB1EMA200Period + VCB1EMA200RiseBars + p.CompressionWindow + VCB1SetupLifetime + 10
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, VCB1Minutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema200 := EMA(c1h, VCB1EMA200Period)
	ema50 := EMA(c1h, VCB1EMA50Period)
	atr := ATRWilder(c1h, VCB1ATRPeriod)

	state := vcbIdle
	var setup *vcbSetup
	cash := StartCash
	realizedCash := StartCash

	closeSetup := func(exitPrice float64, exitTime time.Time, exitIdx int, reason string, exitCtx map[string]float64) {
		if setup == nil || setup.coins <= 0 {
			return
		}
		cash += setup.coins * exitPrice
		setup.coins = 0
		realizedCash = cash
		if setup.entryPrice > 0 {
			barsInTrade := 0
			if setup.entryIdx >= 0 && exitIdx >= setup.entryIdx {
				barsInTrade = exitIdx - setup.entryIdx
			}
			exitCtx["tp1_hit"] = CtxFlag(setup.tp1Hit)
			exitCtx["tp2_hit"] = CtxFlag(setup.tp2Hit)
			exitCtx["bars_in_trade"] = float64(barsInTrade)
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:      setup.entryTime,
				SellTime:     exitTime,
				WaitHours:    exitTime.Sub(setup.entryTime).Hours(),
				BuyPrice:     setup.entryPrice,
				SellPrice:    exitPrice,
				ExitReason:   reason,
				EntryContext: CloneContext(setup.entryContext),
				ExitContext:  CloneContext(exitCtx),
				Events:       CloneEvents(setup.tradeEvents),
			})
		}
		state = vcbIdle
		setup = nil
	}

	cancelSetup := func() {
		state = vcbIdle
		setup = nil
	}

	trendOK := func(j int) bool {
		if j < VCB1EMA200Period-1+VCB1EMA200RiseBars || ema200[j] <= 0 {
			return false
		}
		if ema200[j] <= ema200[j-VCB1EMA200RiseBars] {
			return false
		}
		switch p.TrendFilter {
		case NR7TrendEMA50Above200:
			if j < VCB1EMA50Period-1 || ema50[j] <= 0 {
				return false
			}
			return ema50[j] > ema200[j]
		case NR7TrendBoth:
			if j < VCB1EMA50Period-1 || ema50[j] <= 0 {
				return false
			}
			return c1h[j].Close > ema200[j] && ema50[j] > ema200[j]
		default:
			return c1h[j].Close > ema200[j]
		}
	}

	computeStop := func(j int, entry float64) float64 {
		if p.StopMode == VCBStopATR2BelowEntry && j >= 0 && atr[j] > 0 {
			return entry - atr[j]*2
		}
		if setup != nil {
			return setup.compLow
		}
		return 0
	}

	tryEntry := func(j int, c model.Candle) bool {
		if setup == nil || j < VCB1ATRPeriod-1 || atr[j] <= 0 {
			return false
		}
		barRange := c.High - c.Low
		minRange := atr[j] * p.BreakoutExpansion
		if c.Close <= setup.compHigh || barRange <= minRange {
			return false
		}
		entry := c.Close
		stop := computeStop(j, entry)
		risk := entry - stop
		if risk <= 0 {
			return false
		}
		setup.entryPrice = entry
		setup.entryTime = c.OpenTime
		setup.entryIdx = j
		setup.stopLevel = stop
		setup.tp1Level = entry + risk
		setup.tp2Level = entry + 2*risk
		setup.entryContext = map[string]float64{
			"compression_high": setup.compHigh,
			"compression_low":  setup.compLow,
			"compression_atr":  setup.compATR,
			"atr":              atr[j],
			"sl":               stop,
			"tp1":              setup.tp1Level,
			"tp2":              setup.tp2Level,
			"risk_r":           risk,
			"risk_pct":         risk / entry * 100,
			"comp_window":      float64(p.CompressionWindow),
			"range_window":     float64(p.RangeWindow),
			"atr_compression":  p.ATRCompression,
			"breakout_expansion": p.BreakoutExpansion,
			"trend_filter":     float64(p.TrendFilter),
			"breakout_range":   barRange,
			"stop_mode":        float64(p.StopMode),
		}
		setup.coins = cash / entry
		cash = 0
		state = vcbInPosition
		return true
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		switch state {
		case vcbWaitBreakout:
			if setup == nil {
				state = vcbIdle
				continue
			}
			if j > setup.expireIdx {
				cancelSetup()
				continue
			}
			if tryEntry(j, c) {
				continue
			}

		case vcbInPosition:
			if setup == nil {
				state = vcbIdle
				continue
			}
			if !setup.tp1Done {
				if c.Low <= setup.stopLevel {
					closeSetup(setup.stopLevel, c.OpenTime, j, ExitReasonStop, ExitCtx(setup.stopLevel, map[string]float64{
						"stop": setup.stopLevel, "tp1_done": 0,
					}))
					continue
				}
				if c.High >= setup.tp1Level {
					exitPx := setup.tp1Level
					sellCoins := setup.coins * VCB1TP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.tp1Hit = true
					setup.stopLevel = setup.entryPrice
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: VCB1TP1Fraction,
					})
				}
				continue
			}
			if c.Low <= setup.entryPrice {
				closeSetup(setup.entryPrice, c.OpenTime, j, ExitReasonBreakeven, ExitCtx(setup.entryPrice, map[string]float64{
					"entry": setup.entryPrice, "tp1_done": 1,
				}))
				continue
			}
			if c.High >= setup.tp2Level {
				exitPx := setup.tp2Level
				if c.Close > exitPx {
					exitPx = c.Close
				}
				setup.tp2Hit = true
				closeSetup(exitPx, c.OpenTime, j, ExitReasonTP2, ExitCtx(exitPx, map[string]float64{
					"tp2": setup.tp2Level, "tp1_done": 1,
				}))
			}

		case vcbIdle:
			if j < VCB1ATRPeriod-1 || atr[j] <= 0 {
				continue
			}
			if !trendOK(j) {
				continue
			}
			if !isATRCompressionBar(atr, j, p.CompressionWindow, p.ATRCompression) {
				continue
			}
			compHigh, compLow, ok := compressionRangeHL(c1h, j, p.RangeWindow)
			if !ok {
				continue
			}
			setup = &vcbSetup{
				compHigh:  compHigh,
				compLow:   compLow,
				compATR:   atr[j],
				setupIdx:  j,
				setupTime: c.OpenTime,
				expireIdx: j + VCB1SetupLifetime,
			}
			state = vcbWaitBreakout
		}
	}

	if setup != nil && setup.coins > 0 && state == vcbInPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := c1h[len(c1h)-1].Close
		rep.FinalCash = cash + setup.coins*last
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

// VolatilityCompressionBreakoutV1SweepCompressionWindows returns compression window sizes.
func VolatilityCompressionBreakoutV1SweepCompressionWindows() []int {
	return []int{50, 100, 200}
}

// VolatilityCompressionBreakoutV1SweepRangeWindows returns range window sizes.
func VolatilityCompressionBreakoutV1SweepRangeWindows() []int {
	return []int{5, 10, 20}
}

// VolatilityCompressionBreakoutV1SweepATRCompression returns ATR compression factors.
func VolatilityCompressionBreakoutV1SweepATRCompression() []float64 {
	return []float64{0.5, 0.6, 0.7, 0.8}
}

// VolatilityCompressionBreakoutV1SweepBreakoutExpansion returns breakout expansion multipliers.
func VolatilityCompressionBreakoutV1SweepBreakoutExpansion() []float64 {
	return []float64{1.2, 1.5, 2.0}
}