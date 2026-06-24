package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	FPT1EMA200Period    = 200
	FPT1EMA20Period     = 20
	FPT1EMA200RiseBars  = 20
	FPT1Fib786Ratio     = 0.786
	FPT1TP1Fraction     = 0.5
	FPT1Minutes15M      = 15
	FPT1Minutes1H       = 60
)

// FibPullbackTrendZone defines the pullback touch band (retracement from legHigh).
type FibPullbackTrendZone struct {
	TopRatio    float64 // e.g. 0.5 → fib50
	BottomRatio float64 // e.g. 0.618 → fib618
}

func (z FibPullbackTrendZone) Label() string {
	switch {
	case z.TopRatio == 0.382 && z.BottomRatio == 0.618:
		return "0.382-0.618"
	case z.TopRatio == 0.5 && z.BottomRatio == 0.618:
		return "0.500-0.618"
	case z.TopRatio == 0.5 && z.BottomRatio == 0.786:
		return "0.500-0.786"
	default:
		return "custom"
	}
}

// FibPullbackTrendV1Params tunes the spec algorithm (sweep grid).
type FibPullbackTrendV1Params struct {
	PivotLength    int
	MinImpulsePct  float64
	Zone           FibPullbackTrendZone
	MaxWaitBars15M int
}

func DefaultFibPullbackTrendV1Params() FibPullbackTrendV1Params {
	return FibPullbackTrendV1Params{
		PivotLength:    5,
		MinImpulsePct:  8,
		Zone:           FibPullbackTrendZone{TopRatio: 0.5, BottomRatio: 0.618},
		MaxWaitBars15M: 48,
	}
}

type fptState int

const (
	fptIdle fptState = iota
	fptWaitPullback
	fptWaitConfirm
	fptInPosition
)

type fptSwingPoint struct {
	idx   int
	price float64
}

type fptSetup struct {
	legLow        float64
	legHigh       float64
	fib50         float64
	fib618        float64
	fib786        float64
	zoneTop       float64
	zoneBottom    float64
	bosIdx15m     int
	expireIdx     int
	entryIdx15m   int
	entryPrice    float64
	entryTime     time.Time
	stopLevel     float64
	tp1Level      float64
	tp2Level      float64
	legImpulsePct float64
	tp1Done       bool
	tp1Hit        bool
	tp2Hit        bool
	coins         float64
	entryContext  map[string]float64
	tradeEvents   []TradeEvent
}

// SimulateFibPullbackTrendV1 runs spec Fib Pullback Trend v1 (1H BOS + 15M fib retest).
func SimulateFibPullbackTrendV1(candles []model.Candle) SimulationReport {
	return SimulateFibPullbackTrendV1WithParams(candles, DefaultFibPullbackTrendV1Params())
}

// SimulateFibPullbackTrendV1WithParams runs with configurable sweep parameters.
func SimulateFibPullbackTrendV1WithParams(candles []model.Candle, p FibPullbackTrendV1Params) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if p.PivotLength < 1 {
		p.PivotLength = 5
	}
	if p.MinImpulsePct <= 0 {
		p.MinImpulsePct = 8
	}
	if p.Zone.TopRatio <= 0 || p.Zone.BottomRatio <= 0 {
		p.Zone = DefaultFibPullbackTrendV1Params().Zone
	}
	if p.MaxWaitBars15M < 1 {
		p.MaxWaitBars15M = 48
	}

	min15 := FPT1EMA200Period*4 + p.PivotLength*8 + p.MaxWaitBars15M + 30
	if len(candles) < min15*15 {
		return rep
	}

	c15 := AggregateMinutes(candles, FPT1Minutes15M)
	c1h := AggregateMinutes(candles, FPT1Minutes1H)
	if len(c15) < min15 || len(c1h) < FPT1EMA200Period+FPT1EMA200RiseBars+5 {
		return rep
	}

	ema20 := EMA(c15, FPT1EMA20Period)
	ema1h := EMA(c1h, FPT1EMA200Period)

	h1Index := make(map[time.Time]int, len(c1h))
	for i, c := range c1h {
		h1Index[c.OpenTime] = i
	}

	state := fptIdle
	var setup *fptSetup
	var swingHighs, swingLows []fptSwingPoint
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
			if setup.entryIdx15m >= 0 && exitIdx >= setup.entryIdx15m {
				barsInTrade = exitIdx - setup.entryIdx15m
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
		state = fptIdle
		setup = nil
	}

	trendOK1H := func(hIdx int) bool {
		if hIdx < FPT1EMA200Period-1+FPT1EMA200RiseBars || ema1h[hIdx] <= 0 {
			return false
		}
		if c1h[hIdx].Close <= ema1h[hIdx] {
			return false
		}
		return ema1h[hIdx] > ema1h[hIdx-FPT1EMA200RiseBars]
	}

	cancelSetup := func() {
		state = fptIdle
		setup = nil
	}

	tryBOS := func(hIdx, j int) {
		if state != fptIdle || hIdx < p.PivotLength*2 {
			return
		}
		if len(swingHighs) == 0 || len(swingLows) == 0 {
			return
		}
		if !trendOK1H(hIdx) {
			return
		}

		lastSH := swingHighs[len(swingHighs)-1]
		closeH := c1h[hIdx].Close
		if closeH <= lastSH.price {
			return
		}

		legLow := swingLows[len(swingLows)-1]
		legHigh := c1h[hIdx].High
		if legLow.price <= 0 || legHigh <= legLow.price {
			return
		}

		impulsePct := (legHigh - legLow.price) / legLow.price * 100
		if impulsePct < p.MinImpulsePct {
			return
		}

		rng := legHigh - legLow.price
		zoneTop := legHigh - p.Zone.TopRatio*rng
		zoneBottom := legHigh - p.Zone.BottomRatio*rng
		if zoneTop <= zoneBottom {
			return
		}

		setup = &fptSetup{
			legLow:        legLow.price,
			legHigh:       legHigh,
			fib50:         legHigh - 0.50*rng,
			fib618:        legHigh - 0.618*rng,
			fib786:        legHigh - FPT1Fib786Ratio*rng,
			zoneTop:       zoneTop,
			zoneBottom:    zoneBottom,
			bosIdx15m:     j,
			expireIdx:     j + p.MaxWaitBars15M,
			legImpulsePct: impulsePct,
		}
		state = fptWaitPullback
	}

	on1HClose := func(hIdx int) {
		pi := hIdx - p.PivotLength
		if pi >= p.PivotLength && isPivotHigh(c1h, pi, p.PivotLength) {
			swingHighs = append(swingHighs, fptSwingPoint{pi, c1h[pi].High})
		}
		if pi >= p.PivotLength && isPivotLow(c1h, pi, p.PivotLength) {
			swingLows = append(swingLows, fptSwingPoint{pi, c1h[pi].Low})
		}
	}

	touchZone := func(c model.Candle) bool {
		if setup == nil {
			return false
		}
		return c.Low <= setup.zoneTop && c.High >= setup.zoneBottom
	}

	for j := 0; j < len(c15); j++ {
		c := c15[j]
		bucket1h := bucketOpenUTC(c.OpenTime, FPT1Minutes1H)
		if hIdx, ok := h1Index[bucket1h]; ok {
			is1hClose := j == len(c15)-1
			if !is1hClose {
				nextBucket := bucketOpenUTC(c15[j+1].OpenTime, FPT1Minutes1H)
				is1hClose = !nextBucket.Equal(bucket1h)
			}
			if is1hClose {
				on1HClose(hIdx)
				tryBOS(hIdx, j)
			}
		}

		switch state {
		case fptWaitPullback, fptWaitConfirm:
			if setup == nil {
				state = fptIdle
				continue
			}
			if j > setup.expireIdx {
				cancelSetup()
				continue
			}
			if c.Close < setup.fib786 {
				cancelSetup()
				continue
			}

			if state == fptWaitPullback {
				if touchZone(c) {
					state = fptWaitConfirm
				}
				continue
			}

			if j <= setup.bosIdx15m {
				continue
			}
			if j < FPT1EMA20Period-1 || ema20[j] <= 0 {
				continue
			}
			prevHigh := c15[j-1].High
			if c.Close <= ema20[j] || c.Close <= prevHigh {
				continue
			}

			entry := c.Close
			stop := setup.fib786
			risk := entry - stop
			if risk <= 0 {
				cancelSetup()
				continue
			}

			setup.entryPrice = entry
			setup.entryTime = c.OpenTime
			setup.entryIdx15m = j
			setup.stopLevel = stop
			setup.tp1Level = entry + risk
			setup.tp2Level = entry + 2*risk
			setup.entryContext = map[string]float64{
				"leg_low":      setup.legLow,
				"leg_high":     setup.legHigh,
				"fib50":        setup.fib50,
				"fib618":       setup.fib618,
				"fib786":       setup.fib786,
				"zone_top":     setup.zoneTop,
				"zone_bottom":  setup.zoneBottom,
				"ema20_15m":    ema20[j],
				"sl":           stop,
				"tp1":          setup.tp1Level,
				"tp2":          setup.tp2Level,
				"risk_r":       risk,
				"risk_pct":     risk / entry * 100,
				"impulse_pct":  setup.legImpulsePct,
				"pivot_length": float64(p.PivotLength),
				"zone_label":   zoneLabelCode(p.Zone),
			}
			setup.coins = cash / entry
			cash = 0
			state = fptInPosition

		case fptInPosition:
			if setup == nil {
				state = fptIdle
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
					sellCoins := setup.coins * FPT1TP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.tp1Hit = true
					setup.stopLevel = setup.entryPrice
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: FPT1TP1Fraction,
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
		}
	}

	if setup != nil && setup.coins > 0 && state == fptInPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := c15[len(c15)-1].Close
		rep.FinalCash = cash + setup.coins*last
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

func zoneLabelCode(z FibPullbackTrendZone) float64 {
	switch z.Label() {
	case "0.382-0.618":
		return 382
	case "0.500-0.618":
		return 500
	case "0.500-0.786":
		return 786
	default:
		return 0
	}
}

// FibPullbackTrendV1SweepZones returns pullback zones from the spec.
func FibPullbackTrendV1SweepZones() []FibPullbackTrendZone {
	return []FibPullbackTrendZone{
		{TopRatio: 0.382, BottomRatio: 0.618},
		{TopRatio: 0.5, BottomRatio: 0.618},
		{TopRatio: 0.5, BottomRatio: 0.786},
	}
}
