package strategy

import (
	"math/rand"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	// TrendSMAPeriod is daily SMA lookback for bull/bear filter.
	TrendSMAPeriod = 50
	// TrendMarginUSD is isolated margin per short (leverage 1).
	TrendMarginUSD = 30
	// TrendShortLeverage is fixed short leverage for bear regime.
	TrendShortLeverage = 1
	// TrendRetestEpsilonPct is default tolerance around SMA touch.
	TrendRetestEpsilonPct = 0.1
	// TrendRetestLookaheadCandles is default max candles after breakout to find retest.
	TrendRetestLookaheadCandles = 60
)

// TrendParams holds independent long/short take-profit percents.
type TrendParams struct {
	LongTargetPct  float64
	ShortTargetPct float64
}

// TrendDailyCache stores precomputed daily close and SMA maps for one candle slice.
type TrendDailyCache struct {
	DailyClose map[time.Time]float64
	DailySMA   map[int]map[time.Time]float64
}

type dayRange struct {
	day   time.Time
	start int
	end   int
}

// NewTrendDailyCache builds daily close + requested SMA(period) maps once.
func NewTrendDailyCache(candles []model.Candle, smaPeriods []int) *TrendDailyCache {
	dailyClose := buildDailyCloses(candles)
	cache := &TrendDailyCache{
		DailyClose: dailyClose,
		DailySMA:   make(map[int]map[time.Time]float64),
	}
	seen := make(map[int]bool)
	for _, period := range smaPeriods {
		if period < 1 || seen[period] {
			continue
		}
		seen[period] = true
		cache.DailySMA[period] = buildDailySMA(dailyClose, period)
	}
	return cache
}

// SimulateTrendAdaptive: SMA(50) on daily close vs previous day — long above, short below.
// Short uses margin $30, leverage 1x, with liquidation.
func SimulateTrendAdaptive(candles []model.Candle, seed int64, p TrendParams) SimulationReport {
	return SimulateTrendAdaptiveWithCache(candles, seed, p, nil)
}

// SimulateTrendAdaptiveWithCache is SimulateTrendAdaptive with optional daily cache.
func SimulateTrendAdaptiveWithCache(candles []model.Candle, seed int64, p TrendParams, cache *TrendDailyCache) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
		MarginUSD: TrendMarginUSD,
		Leverage:  TrendShortLeverage,
	}
	if len(candles) == 0 || p.LongTargetPct <= 0 || p.ShortTargetPct <= 0 {
		return rep
	}

	dailyClose := buildDailyCloses(candles)
	dailySMA := buildDailySMA(dailyClose, TrendSMAPeriod)
	if cache != nil {
		if cache.DailyClose != nil {
			dailyClose = cache.DailyClose
		}
		if m, ok := cache.DailySMA[TrendSMAPeriod]; ok {
			dailySMA = m
		}
	}

	rng := rand.New(rand.NewSource(seed))
	cash := StartCash
	realizedCash := StartCash
	i := 0
	inPosition := false
	isLong := false
	var entryIdx int
	var entryPrice float64
	var coins, margin float64
	liqPrice := 0.0

	for i < len(candles) {
		if rep.Bankrupt {
			break
		}

		if !inPosition {
			day := truncateDay(candles[i].OpenTime)
			bull, ok := trendRegime(day, dailyClose, dailySMA)
			if !ok {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}

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

			if bull {
				coins = cash / entryPrice
				isLong = true
				inPosition = true
				i = entryIdx + 1
				continue
			}

			margin = TrendMarginUSD
			if margin > cash {
				margin = cash
			}
			if margin < 1 {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
			liqPrice = shortLiquidationPrice(entryPrice, TrendShortLeverage)
			isLong = false
			inPosition = true
			i = entryIdx + 1
			continue
		}

		close := candles[i].Close
		if isLong {
			target := entryPrice * (1 + p.LongTargetPct/100)
			if close >= target {
				cash = coins * close
				realizedCash = cash
				rep.Trades = append(rep.Trades, Trade{
					BuyTime:   candles[entryIdx].OpenTime,
					SellTime:  candles[i].OpenTime,
					WaitHours: candles[i].OpenTime.Sub(candles[entryIdx].OpenTime).Hours(),
					BuyPrice:  entryPrice,
					SellPrice: close,
				})
				inPosition = false
				if cash <= 0 {
					rep.Bankrupt = true
					cash = 0
					realizedCash = 0
					break
				}
				i = advanceAfterExit(candles, i)
				continue
			}
		} else {
			if close >= liqPrice {
				cash -= margin
				rep.LiquidationCount++
				inPosition = false
				realizedCash = cash
				if cash <= 0 {
					cash = 0
					realizedCash = 0
					rep.Bankrupt = true
					break
				}
				i = advanceAfterExit(candles, i)
				continue
			}
			tp := entryPrice * (1 - p.ShortTargetPct/100)
			if close <= tp {
				cash += shortPnLUSD(margin, float64(TrendShortLeverage), entryPrice, close)
				realizedCash = cash
				rep.Trades = append(rep.Trades, Trade{
					BuyTime:   candles[entryIdx].OpenTime,
					SellTime:  candles[i].OpenTime,
					WaitHours: candles[i].OpenTime.Sub(candles[entryIdx].OpenTime).Hours(),
					BuyPrice:  entryPrice,
					SellPrice: close,
				})
				inPosition = false
				if cash <= 0 {
					rep.Bankrupt = true
					cash = 0
					realizedCash = 0
					break
				}
				i = advanceAfterExit(candles, i)
				continue
			}
		}
		i++
	}

	if inPosition && !rep.Bankrupt {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := candles[len(candles)-1].Close
		if isLong {
			rep.FinalCash = coins * last
		} else if last >= liqPrice {
			cash -= margin
			rep.LiquidationCount++
			rep.OpenPosition = false
			rep.RealizedCash = cash
			rep.FinalCash = cash
			if cash <= 0 {
				rep.Bankrupt = true
			}
		} else {
			rep.FinalCash = cash + shortPnLUSD(margin, float64(TrendShortLeverage), entryPrice, last)
		}
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

func advanceAfterExit(candles []model.Candle, exitIdx int) int {
	nextDay, ok := nextDayWithData(candles, candles[exitIdx].OpenTime)
	if !ok {
		return len(candles)
	}
	return firstIndexOnDay(candles, nextDay)
}

// trendRegime uses previous calendar day close vs its SMA(50). ok=false when equal or no data.
func trendRegime(entryDay time.Time, dailyClose, dailySMA map[time.Time]float64) (bull bool, ok bool) {
	prev := entryDay.AddDate(0, 0, -1)
	close, okC := dailyClose[prev]
	sma, okS := dailySMA[prev]
	if !okC || !okS {
		return false, false
	}
	if close > sma {
		return true, true
	}
	if close < sma {
		return false, true
	}
	return false, false
}

func buildDailyCloses(candles []model.Candle) map[time.Time]float64 {
	out := make(map[time.Time]float64)
	for _, c := range candles {
		d := truncateDay(c.OpenTime)
		out[d] = c.Close
	}
	return out
}

func buildDailySMA(closes map[time.Time]float64, period int) map[time.Time]float64 {
	if period < 1 {
		return nil
	}
	days := make([]time.Time, 0, len(closes))
	for d := range closes {
		days = append(days, d)
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })

	out := make(map[time.Time]float64)
	for i := period - 1; i < len(days); i++ {
		var sum float64
		for j := i - period + 1; j <= i; j++ {
			sum += closes[days[j]]
		}
		out[days[i]] = sum / float64(period)
	}
	return out
}

// SimulateTrendLongOnly enters only long positions on bullish trend days (SMA period = TrendSMAPeriod).
func SimulateTrendLongOnly(candles []model.Candle, seed int64, longTargetPct float64) SimulationReport {
	return SimulateTrendLongOnlySMAWithCache(candles, seed, longTargetPct, TrendSMAPeriod, nil)
}

// SimulateTrendLongOnlySMA enters long only when previous day close > SMA(period) on daily bars.
func SimulateTrendLongOnlySMA(candles []model.Candle, seed int64, longTargetPct float64, smaPeriod int) SimulationReport {
	return SimulateTrendLongOnlySMAWithCache(candles, seed, longTargetPct, smaPeriod, nil)
}

// SimulateTrendLongOnlySMAWithCache is SimulateTrendLongOnlySMA with optional daily cache.
func SimulateTrendLongOnlySMAWithCache(
	candles []model.Candle,
	seed int64,
	longTargetPct float64,
	smaPeriod int,
	cache *TrendDailyCache,
) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) == 0 || longTargetPct <= 0 || smaPeriod < 1 {
		return rep
	}

	dailyClose := buildDailyCloses(candles)
	dailySMA := buildDailySMA(dailyClose, smaPeriod)
	if cache != nil {
		if cache.DailyClose != nil {
			dailyClose = cache.DailyClose
		}
		if m, ok := cache.DailySMA[smaPeriod]; ok {
			dailySMA = m
		}
	}
	rng := rand.New(rand.NewSource(seed))

	cash := StartCash
	realizedCash := StartCash
	i := 0
	inPosition := false
	var entryIdx int
	var entryPrice, coins float64

	for i < len(candles) {
		if !inPosition {
			day := truncateDay(candles[i].OpenTime)
			bull, ok := trendRegime(day, dailyClose, dailySMA)
			if !ok || !bull {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
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
			coins = cash / entryPrice
			inPosition = true
			i = entryIdx + 1
			continue
		}

		target := entryPrice * (1 + longTargetPct/100)
		if candles[i].Close >= target {
			exitPrice := candles[i].Close
			cash = coins * exitPrice
			realizedCash = cash
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:   candles[entryIdx].OpenTime,
				SellTime:  candles[i].OpenTime,
				WaitHours: candles[i].OpenTime.Sub(candles[entryIdx].OpenTime).Hours(),
				BuyPrice:  entryPrice,
				SellPrice: exitPrice,
			})
			inPosition = false
			i = advanceAfterExit(candles, i)
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

// SimulateTrendLongRetestSMAWithCache enters long after close breaks above SMA(period),
// then price retests SMA within lookahead candles (with epsilon tolerance).
func SimulateTrendLongRetestSMAWithCache(
	candles []model.Candle,
	seed int64,
	longTargetPct float64,
	smaPeriod int,
	epsilonPct float64,
	lookahead int,
	cache *TrendDailyCache,
) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) == 0 || longTargetPct <= 0 || smaPeriod < 1 || lookahead < 1 || epsilonPct < 0 {
		return rep
	}

	dailyClose := buildDailyCloses(candles)
	dailySMA := buildDailySMA(dailyClose, smaPeriod)
	if cache != nil {
		if cache.DailyClose != nil {
			dailyClose = cache.DailyClose
		}
		if m, ok := cache.DailySMA[smaPeriod]; ok {
			dailySMA = m
		}
	}

	_ = rand.New(rand.NewSource(seed)) // keep deterministic signature parity with other strategies
	cash := StartCash
	realizedCash := StartCash
	i := 1
	inPosition := false
	var entryIdx int
	var entryPrice, coins float64
	waitRetest := false
	retestUntil := -1

	for i < len(candles) {
		if inPosition {
			target := entryPrice * (1 + longTargetPct/100)
			if candles[i].Close >= target {
				exitPrice := candles[i].Close
				cash = coins * exitPrice
				realizedCash = cash
				rep.Trades = append(rep.Trades, Trade{
					BuyTime:   candles[entryIdx].OpenTime,
					SellTime:  candles[i].OpenTime,
					WaitHours: candles[i].OpenTime.Sub(candles[entryIdx].OpenTime).Hours(),
					BuyPrice:  entryPrice,
					SellPrice: exitPrice,
				})
				inPosition = false
				waitRetest = false
				retestUntil = -1
				i = advanceAfterExit(candles, i)
				if i < 1 {
					i = 1
				}
				continue
			}
			i++
			continue
		}

		day := truncateDay(candles[i].OpenTime)
		bull, ok := trendRegime(day, dailyClose, dailySMA)
		if !ok || !bull {
			waitRetest = false
			retestUntil = -1
			i++
			continue
		}

		sma, hasSMA := dailySMA[day.AddDate(0, 0, -1)]
		if !hasSMA {
			waitRetest = false
			retestUntil = -1
			i++
			continue
		}

		prevClose := candles[i-1].Close
		close := candles[i].Close
		if !waitRetest {
			if prevClose <= sma && close > sma {
				waitRetest = true
				retestUntil = i + lookahead
			}
			i++
			continue
		}

		if i > retestUntil {
			waitRetest = false
			retestUntil = -1
			continue
		}

		touchMin := sma * (1 - epsilonPct/100)
		touchMax := sma * (1 + epsilonPct/100)
		low := candles[i].Low
		high := candles[i].High
		touched := low <= touchMax && high >= touchMin
		if touched {
			entryIdx = i
			entryPrice = candles[i].Close
			if entryPrice > 0 {
				coins = cash / entryPrice
				inPosition = true
				waitRetest = false
				retestUntil = -1
				i++
				continue
			}
			waitRetest = false
			retestUntil = -1
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

type trendLongSweepState struct {
	targetPct float64
	rep       SimulationReport
	rng       *rand.Rand

	cash         float64
	realizedCash float64
	inPosition   bool
	entryIdx     int
	entryPrice   float64
	coins        float64
	nextDay      time.Time
}

// SimulateTrendLongOnlySMASweepWithCache runs long-only trend simulation for multiple targets
// in one candle pass for a fixed SMA period.
func SimulateTrendLongOnlySMASweepWithCache(
	candles []model.Candle,
	smaPeriod int,
	targets []float64,
	seeds []int64,
	cache *TrendDailyCache,
) []SimulationReport {
	out := make([]SimulationReport, len(targets))
	if len(candles) == 0 || smaPeriod < 1 || len(targets) == 0 || len(targets) != len(seeds) {
		return out
	}

	dailyClose := buildDailyCloses(candles)
	dailySMA := buildDailySMA(dailyClose, smaPeriod)
	if cache != nil {
		if cache.DailyClose != nil {
			dailyClose = cache.DailyClose
		}
		if m, ok := cache.DailySMA[smaPeriod]; ok {
			dailySMA = m
		}
	}

	dayRanges := buildDayRanges(candles)
	states := make([]trendLongSweepState, len(targets))
	for i := range targets {
		rep := SimulationReport{
			StartCash: StartCash,
			FinalCash: StartCash,
		}
		out[i] = rep
		if targets[i] <= 0 {
			continue
		}
		states[i] = trendLongSweepState{
			targetPct:    targets[i],
			rep:          rep,
			rng:          rand.New(rand.NewSource(seeds[i])),
			cash:         StartCash,
			realizedCash: StartCash,
			entryIdx:     -1,
		}
	}

	for _, dr := range dayRanges {
		bull, ok := trendRegime(dr.day, dailyClose, dailySMA)
		if ok && bull {
			for i := range states {
				s := &states[i]
				if s.targetPct <= 0 || s.inPosition {
					continue
				}
				if !s.nextDay.IsZero() && dr.day.Before(s.nextDay) {
					continue
				}
				start := dr.start
				for start <= dr.end {
					entryIdx := start + s.rng.Intn(dr.end-start+1)
					entryPrice := candles[entryIdx].Close
					if entryPrice > 0 {
						s.entryIdx = entryIdx
						s.entryPrice = entryPrice
						s.coins = s.cash / entryPrice
						s.inPosition = true
						break
					}
					start = entryIdx + 1
				}
			}
		}

		for ci := dr.start; ci <= dr.end; ci++ {
			close := candles[ci].Close
			for i := range states {
				s := &states[i]
				if !s.inPosition || ci <= s.entryIdx {
					continue
				}
				target := s.entryPrice * (1 + s.targetPct/100)
				if close < target {
					continue
				}
				exitPrice := close
				s.cash = s.coins * exitPrice
				s.realizedCash = s.cash
				s.rep.Trades = append(s.rep.Trades, Trade{
					BuyTime:   candles[s.entryIdx].OpenTime,
					SellTime:  candles[ci].OpenTime,
					WaitHours: candles[ci].OpenTime.Sub(candles[s.entryIdx].OpenTime).Hours(),
					BuyPrice:  s.entryPrice,
					SellPrice: exitPrice,
				})
				s.inPosition = false
				s.entryIdx = -1
				s.nextDay = dr.day.AddDate(0, 0, 1)
			}
		}
	}

	lastClose := candles[len(candles)-1].Close
	for i := range states {
		s := &states[i]
		if s.targetPct <= 0 {
			continue
		}
		if s.inPosition {
			s.rep.OpenPosition = true
			s.rep.RealizedCash = s.realizedCash
			s.rep.FinalCash = s.coins * lastClose
		} else {
			s.rep.RealizedCash = s.cash
			s.rep.FinalCash = s.cash
		}
		s.rep.fillStats()
		out[i] = s.rep
	}
	return out
}

func buildDayRanges(candles []model.Candle) []dayRange {
	if len(candles) == 0 {
		return nil
	}
	var out []dayRange
	start := 0
	day := truncateDay(candles[0].OpenTime)
	for i := 1; i < len(candles); i++ {
		d := truncateDay(candles[i].OpenTime)
		if d.Equal(day) {
			continue
		}
		out = append(out, dayRange{day: day, start: start, end: i - 1})
		start = i
		day = d
	}
	out = append(out, dayRange{day: day, start: start, end: len(candles) - 1})
	return out
}
