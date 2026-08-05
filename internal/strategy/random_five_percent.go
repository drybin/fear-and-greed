package strategy

import (
	"math/rand"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	StartCash   = 100.0
	DefaultSeed = 42
)

// Trade is one completed round trip (long: buy→sell; short: open→cover).
type Trade struct {
	BuyTime   time.Time // entry time (long buy or short open)
	SellTime  time.Time // exit time (long sell or short cover)
	WaitHours float64
	BuyPrice  float64 // entry price
	SellPrice float64 // exit price
	VolumeRatio float64 // entry volume / SMA(volume); 0 if not tracked
	ExitReason   string             // stop | breakeven | tp2 | target | ...
	EntryContext map[string]float64 // indicator levels at entry (algo-specific)
	ExitContext  map[string]float64 // indicator levels at final exit
	Events       []TradeEvent       // partial exits (e.g. TP1 50%)
}

// SimulationReport holds strategy output.
type SimulationReport struct {
	TargetPct      float64
	Trades         []Trade
	Signals        []EntrySignal // close-confirmed entries, including still-open positions
	CompletedCount int
	OpenPosition   bool // true if a position was still open at end of data (may coexist with completed trades)
	FinalCash      float64 // mark-to-market at end (includes open leg)
	RealizedCash   float64 // cash after last completed sell; strategy P/L is based on this
	StartCash      float64
	ProfitUSD      float64 // RealizedCash - StartCash
	ProfitPct      float64 // from RealizedCash only (not open-leg MTM)
	OpenLegUSD         float64 // FinalCash - RealizedCash when OpenPosition
	LiquidationCount   int
	Bankrupt           bool
	Leverage           int
	MarginUSD          float64
	WaitHoursMin       float64
	WaitHoursMax   float64
	WaitHoursAvg   float64
}

// SimulateRandomTarget runs random daily entry; sell when close >= buy * (1 + targetPct/100).
func SimulateRandomTarget(candles []model.Candle, seed int64, targetPct float64) SimulationReport {
	return simulateRandomLongExit(candles, seed, targetPct)
}

// SimulateRandomTargetDrop runs random daily short (1x); cover when close <= entry * (1 - targetPct/100).
func SimulateRandomTargetDrop(candles []model.Candle, seed int64, targetPct float64) SimulationReport {
	return simulateRandomShortExit(candles, seed, targetPct)
}

func simulateRandomLongExit(candles []model.Candle, seed int64, targetPct float64) SimulationReport {
	rep := SimulationReport{
		TargetPct: targetPct,
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) == 0 || targetPct <= 0 {
		return rep
	}

	rng := rand.New(rand.NewSource(seed))
	cash := StartCash
	realizedCash := StartCash
	i := 0
	inPosition := false
	var buyIdx int
	var buyPrice, coins float64

	for i < len(candles) {
		if !inPosition {
			day := truncateDay(candles[i].OpenTime)
			eligible := indicesOnDayFrom(candles, day, i)
			if len(eligible) == 0 {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
			buyIdx = eligible[rng.Intn(len(eligible))]
			buyPrice = candles[buyIdx].Close
			if buyPrice <= 0 {
				i = buyIdx + 1
				continue
			}
			coins = cash / buyPrice
			inPosition = true
			i = buyIdx + 1
			continue
		}

		target := buyPrice * (1 + targetPct/100)
		hit := candles[i].Close >= target
		if hit {
			sellPrice := candles[i].Close
			cash = coins * sellPrice
			realizedCash = cash
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:    candles[buyIdx].OpenTime,
				SellTime:   candles[i].OpenTime,
				WaitHours:  candles[i].OpenTime.Sub(candles[buyIdx].OpenTime).Hours(),
				BuyPrice:   buyPrice,
				SellPrice:  sellPrice,
				ExitReason: ExitReasonTarget,
			})
			inPosition = false

			nextDay, ok := nextDayWithData(candles, candles[i].OpenTime)
			if !ok {
				rep.RealizedCash = cash
				rep.FinalCash = cash
				rep.fillStats()
				return rep
			}
			i = firstIndexOnDay(candles, nextDay)
			continue
		}
		i++
	}

	if inPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		rep.FinalCash = coins * candles[len(candles)-1].Close
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

// shortCashMTM returns wallet cash for a 1x short with fixed USD notional at entry.
func shortCashMTM(notional, entryPrice, markPrice float64) float64 {
	if entryPrice <= 0 {
		return notional
	}
	return notional + notional*(entryPrice-markPrice)/entryPrice
}

func simulateRandomShortExit(candles []model.Candle, seed int64, targetPct float64) SimulationReport {
	rep := SimulationReport{
		TargetPct: targetPct,
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) == 0 || targetPct <= 0 {
		return rep
	}

	rng := rand.New(rand.NewSource(seed))
	cash := StartCash
	realizedCash := StartCash
	i := 0
	inPosition := false
	var entryIdx int
	var entryPrice, notional float64

	for i < len(candles) {
		if !inPosition {
			day := truncateDay(candles[i].OpenTime)
			eligible := indicesOnDayFrom(candles, day, i)
			if len(eligible) == 0 {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
			entryIdx = eligible[rng.Intn(len(eligible))]
			entryPrice = candles[entryIdx].Close
			if entryPrice <= 0 {
				i = entryIdx + 1
				continue
			}
			notional = cash
			inPosition = true
			i = entryIdx + 1
			continue
		}

		target := entryPrice * (1 - targetPct/100)
		if candles[i].Close <= target {
			coverPrice := candles[i].Close
			cash = shortCashMTM(notional, entryPrice, coverPrice)
			realizedCash = cash
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:    candles[entryIdx].OpenTime,
				SellTime:   candles[i].OpenTime,
				WaitHours:  candles[i].OpenTime.Sub(candles[entryIdx].OpenTime).Hours(),
				BuyPrice:   entryPrice,
				SellPrice:  coverPrice,
				ExitReason: ExitReasonCover,
			})
			inPosition = false

			nextDay, ok := nextDayWithData(candles, candles[i].OpenTime)
			if !ok {
				rep.RealizedCash = cash
				rep.FinalCash = cash
				rep.fillStats()
				return rep
			}
			i = firstIndexOnDay(candles, nextDay)
			continue
		}
		i++
	}

	if inPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := candles[len(candles)-1].Close
		rep.FinalCash = shortCashMTM(notional, entryPrice, last)
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

func indicesOnDayFrom(candles []model.Candle, day time.Time, from int) []int {
	var idx []int
	for j := from; j < len(candles); j++ {
		if !truncateDay(candles[j].OpenTime).Equal(day) {
			break
		}
		idx = append(idx, j)
	}
	return idx
}

func skipToNextDay(candles []model.Candle, i int) int {
	if i >= len(candles) {
		return len(candles)
	}
	day := truncateDay(candles[i].OpenTime)
	for j := i + 1; j < len(candles); j++ {
		if !truncateDay(candles[j].OpenTime).Equal(day) {
			return j
		}
	}
	return len(candles)
}

func firstIndexOnDay(candles []model.Candle, day time.Time) int {
	for i, c := range candles {
		if truncateDay(c.OpenTime).Equal(day) {
			return i
		}
	}
	return len(candles)
}

func (r *SimulationReport) fillStats() {
	r.CompletedCount = len(r.Trades)
	if r.RealizedCash == 0 {
		r.RealizedCash = r.StartCash
	}
	r.ProfitUSD = r.RealizedCash - r.StartCash
	if r.StartCash > 0 {
		r.ProfitPct = (r.RealizedCash/r.StartCash - 1) * 100
	}
	if r.OpenPosition {
		r.OpenLegUSD = r.FinalCash - r.RealizedCash
	}
	if len(r.Trades) == 0 {
		return
	}
	minH := r.Trades[0].WaitHours
	maxH := r.Trades[0].WaitHours
	var sum float64
	for _, t := range r.Trades {
		sum += t.WaitHours
		if t.WaitHours < minH {
			minH = t.WaitHours
		}
		if t.WaitHours > maxH {
			maxH = t.WaitHours
		}
	}
	r.WaitHoursMin = minH
	r.WaitHoursMax = maxH
	r.WaitHoursAvg = sum / float64(len(r.Trades))
}

func nextDayWithData(candles []model.Candle, sellTime time.Time) (time.Time, bool) {
	sellDay := truncateDay(sellTime)
	lastDay := truncateDay(candles[len(candles)-1].OpenTime)
	for d := sellDay.Add(24 * time.Hour); !d.After(lastDay); d = d.Add(24 * time.Hour) {
		if len(indicesOnDay(candles, d)) > 0 {
			return d, true
		}
	}
	return time.Time{}, false
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func indicesOnDay(candles []model.Candle, day time.Time) []int {
	var idx []int
	for i, c := range candles {
		if truncateDay(c.OpenTime).Equal(day) {
			idx = append(idx, i)
		}
	}
	return idx
}

// FilterLastYears keeps candles with time >= end - years.
func FilterLastYears(candles []model.Candle, years float64) []model.Candle {
	if len(candles) == 0 {
		return nil
	}
	end := candles[len(candles)-1].OpenTime
	cutoff := end.Add(-time.Duration(years * 365.25 * 24 * float64(time.Hour)))
	out := make([]model.Candle, 0)
	for _, c := range candles {
		if !c.OpenTime.Before(cutoff) {
			out = append(out, c)
		}
	}
	return out
}

// FilterCurrentYear keeps candles in the calendar year of the last candle in the series.
func FilterCurrentYear(candles []model.Candle) []model.Candle {
	if len(candles) == 0 {
		return nil
	}
	y := candles[len(candles)-1].OpenTime.Year()
	start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]model.Candle, 0)
	for _, c := range candles {
		if !c.OpenTime.Before(start) {
			out = append(out, c)
		}
	}
	return out
}
