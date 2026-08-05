package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	NR7TB1EMA200Period   = 200
	NR7TB1EMA50Period    = 50
	NR7TB1EMA200RiseBars = 20
	NR7TB1ATRPeriod      = 14
	NR7TB1TP1Fraction    = 0.5
	NR7TB1Minutes1H      = 60
)

// NR7TrendFilter selects the bull-trend gate (sweep).
type NR7TrendFilter int

const (
	NR7TrendCloseEMA200 NR7TrendFilter = iota
	NR7TrendEMA50Above200
	NR7TrendBoth
)

func (f NR7TrendFilter) Label() string {
	switch f {
	case NR7TrendCloseEMA200:
		return "close>EMA200"
	case NR7TrendEMA50Above200:
		return "EMA50>EMA200"
	case NR7TrendBoth:
		return "close>EMA200+EMA50>EMA200"
	default:
		return "unknown"
	}
}

// NR7TrendBreakoutV1Params tunes NR breakout (sweep grid).
type NR7TrendBreakoutV1Params struct {
	NRLength       int
	ATRCompression float64
	SetupLifetime  int
	TrendFilter    NR7TrendFilter
}

func DefaultNR7TrendBreakoutV1Params() NR7TrendBreakoutV1Params {
	return NR7TrendBreakoutV1Params{
		NRLength:       7,
		ATRCompression: 0.8,
		SetupLifetime:  12,
		TrendFilter:    NR7TrendCloseEMA200,
	}
}

type nr7tbState int

const (
	nr7tbIdle nr7tbState = iota
	nr7tbWaitBreakout
	nr7tbInPosition
)

type nr7tbSetup struct {
	nrHigh       float64
	nrLow        float64
	nrRange      float64
	setupIdx     int
	setupTime    time.Time
	expireIdx    int
	entryIdx     int
	entryPrice   float64
	entryTime    time.Time
	stopLevel    float64
	tp1Level     float64
	tp2Level     float64
	entryATR     float64
	tp1Done      bool
	tp1Hit       bool
	tp2Hit       bool
	coins        float64
	entryContext map[string]float64
	tradeEvents  []TradeEvent
}

func isNRCandle(candles []model.Candle, j, nrLen int) bool {
	if nrLen < 2 || j < nrLen-1 {
		return false
	}
	r0 := candles[j].High - candles[j].Low
	for k := 1; k < nrLen; k++ {
		rk := candles[j-k].High - candles[j-k].Low
		if r0 >= rk {
			return false
		}
	}
	return true
}

// SimulateNR7TrendBreakoutV1 runs 1H NR compression + trend breakout long (spec defaults).
func SimulateNR7TrendBreakoutV1(candles []model.Candle) SimulationReport {
	return SimulateNR7TrendBreakoutV1WithParams(candles, DefaultNR7TrendBreakoutV1Params())
}

// SimulateNR7TrendBreakoutV1WithParams runs with configurable sweep parameters.
func SimulateNR7TrendBreakoutV1WithParams(candles []model.Candle, p NR7TrendBreakoutV1Params) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if p.NRLength < 2 {
		p.NRLength = 7
	}
	if p.ATRCompression <= 0 {
		p.ATRCompression = 0.8
	}
	if p.SetupLifetime < 1 {
		p.SetupLifetime = 12
	}

	min1h := NR7TB1EMA200Period + NR7TB1EMA200RiseBars + p.NRLength + p.SetupLifetime + 10
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, NR7TB1Minutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema200 := EMA(c1h, NR7TB1EMA200Period)
	ema50 := EMA(c1h, NR7TB1EMA50Period)
	atr := ATRWilder(c1h, NR7TB1ATRPeriod)

	state := nr7tbIdle
	var setup *nr7tbSetup
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
		state = nr7tbIdle
		setup = nil
	}

	cancelSetup := func() {
		state = nr7tbIdle
		setup = nil
	}

	trendOK := func(j int) bool {
		if j < NR7TB1EMA200Period-1+NR7TB1EMA200RiseBars || ema200[j] <= 0 {
			return false
		}
		if ema200[j] <= ema200[j-NR7TB1EMA200RiseBars] {
			return false
		}
		switch p.TrendFilter {
		case NR7TrendEMA50Above200:
			if j < NR7TB1EMA50Period-1 || ema50[j] <= 0 {
				return false
			}
			return ema50[j] > ema200[j]
		case NR7TrendBoth:
			if j < NR7TB1EMA50Period-1 || ema50[j] <= 0 {
				return false
			}
			return c1h[j].Close > ema200[j] && ema50[j] > ema200[j]
		default:
			return c1h[j].Close > ema200[j]
		}
	}

	tryEntry := func(j int, c model.Candle) bool {
		if setup == nil || j < NR7TB1ATRPeriod-1 || atr[j] <= 0 {
			return false
		}
		barRange := c.High - c.Low
		if c.Close <= setup.nrHigh || barRange <= atr[j] {
			return false
		}
		entry := c.Close
		stop := setup.nrLow
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
		setup.entryATR = atr[j]
		setup.entryContext = map[string]float64{
			"nr_high":         setup.nrHigh,
			"nr_low":          setup.nrLow,
			"nr_range":        setup.nrRange,
			"atr":             atr[j],
			"sl":              stop,
			"tp1":             setup.tp1Level,
			"tp2":             setup.tp2Level,
			"risk_r":          risk,
			"risk_pct":        risk / entry * 100,
			"nr_length":       float64(p.NRLength),
			"atr_compression": p.ATRCompression,
			"trend_filter":    float64(p.TrendFilter),
			"breakout_range":  barRange,
		}
		rep.Signals = append(rep.Signals, EntrySignal{
			Time: c.OpenTime, EntryPrice: entry, Stop: stop,
			TP1: setup.tp1Level, TP2: setup.tp2Level, Diagnostics: CloneContext(setup.entryContext),
		})
		setup.coins = cash / entry
		cash = 0
		state = nr7tbInPosition
		return true
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		switch state {
		case nr7tbWaitBreakout:
			if setup == nil {
				state = nr7tbIdle
				continue
			}
			if j > setup.expireIdx {
				cancelSetup()
				continue
			}
			if tryEntry(j, c) {
				continue
			}

		case nr7tbInPosition:
			if setup == nil {
				state = nr7tbIdle
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
					sellCoins := setup.coins * NR7TB1TP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.tp1Hit = true
					setup.stopLevel = setup.entryPrice
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: NR7TB1TP1Fraction,
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

		case nr7tbIdle:
			if j < NR7TB1ATRPeriod-1 || atr[j] <= 0 {
				continue
			}
			if !trendOK(j) {
				continue
			}
			if !isNRCandle(c1h, j, p.NRLength) {
				continue
			}
			nrRange := c.High - c.Low
			if nrRange >= atr[j]*p.ATRCompression {
				continue
			}
			setup = &nr7tbSetup{
				nrHigh:    c.High,
				nrLow:     c.Low,
				nrRange:   nrRange,
				setupIdx:  j,
				setupTime: c.OpenTime,
				expireIdx: j + p.SetupLifetime,
			}
			state = nr7tbWaitBreakout
		}
	}

	if setup != nil && setup.coins > 0 && state == nr7tbInPosition {
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

// NR7TrendBreakoutV1SweepTrendFilters returns trend filter variants from the spec.
func NR7TrendBreakoutV1SweepTrendFilters() []NR7TrendFilter {
	return []NR7TrendFilter{NR7TrendCloseEMA200, NR7TrendEMA50Above200, NR7TrendBoth}
}
