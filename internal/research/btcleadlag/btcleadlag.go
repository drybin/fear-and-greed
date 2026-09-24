// Package btcleadlag measures whether completed BTC impulses precede aligned
// moves in a frozen universe. It is descriptive research, not a trading model.
package btcleadlag

import (
	"fmt"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

var defaultHorizons = []int{1, 2, 4, 8}

type Config struct {
	ImpulseThreshold float64
	Horizons         []int
}

type HorizonSummary struct {
	Hours            int     `json:"hours"`
	Observations     int     `json:"observations"`
	MeanReturn       float64 `json:"mean_return"`
	MedianReturn     float64 `json:"median_return"`
	PositiveFraction float64 `json:"positive_fraction"`
}

type DirectionSummary struct {
	Events   int              `json:"events"`
	Horizons []HorizonSummary `json:"horizons"`
}

type SymbolReport struct {
	Symbol string           `json:"symbol"`
	Up     DirectionSummary `json:"btc_up"`
	Down   DirectionSummary `json:"btc_down"`
}

type Report struct {
	SchemaVersion       string         `json:"schema_version"`
	GeneratedAt         time.Time      `json:"generated_at"`
	ImpulseThreshold    float64        `json:"btc_impulse_threshold"`
	EntryRule           string         `json:"entry_rule"`
	SurvivorshipWarning string         `json:"survivorship_warning"`
	Symbols             []SymbolReport `json:"symbols"`
	Aggregate           SymbolReport   `json:"aggregate"`
}

type btcEvent struct {
	time      time.Time
	direction float64
}

func Analyze(btc []model.Candle, alts map[string][]model.Candle, config Config) (Report, error) {
	if config.ImpulseThreshold <= 0 || config.ImpulseThreshold >= 1 {
		return Report{}, fmt.Errorf("btc lead-lag: impulse threshold must be in (0, 1)")
	}
	horizons := append([]int(nil), config.Horizons...)
	if len(horizons) == 0 {
		horizons = append([]int(nil), defaultHorizons...)
	}
	for _, horizon := range horizons {
		if horizon <= 0 {
			return Report{}, fmt.Errorf("btc lead-lag: horizons must be positive")
		}
	}
	sort.Ints(horizons)
	events := btcImpulses(btc, config.ImpulseThreshold)
	report := Report{
		SchemaVersion:       "btc-lead-lag.v1",
		GeneratedAt:         time.Now().UTC(),
		ImpulseThreshold:    config.ImpulseThreshold,
		EntryRule:           "BTC impulse is known only after its 1h candle closes; each alt return starts at the next synchronized 1h open and ends at the close after the stated horizon.",
		SurvivorshipWarning: "Frozen-current-cohort results describe this frozen watchlist tested backward; they do not establish historical top-N performance.",
		Symbols:             make([]SymbolReport, 0, len(alts)),
	}
	keys := make([]string, 0, len(alts))
	for symbol := range alts {
		keys = append(keys, symbol)
	}
	sort.Strings(keys)
	allUp, allDown := make([]observation, 0), make([]observation, 0)
	for _, symbol := range keys {
		if symbol == "BTCUSDT" {
			continue
		}
		up, down := observationsForAlt(alts[symbol], events, horizons)
		allUp = append(allUp, up...)
		allDown = append(allDown, down...)
		report.Symbols = append(report.Symbols, SymbolReport{Symbol: symbol, Up: summarize(up, horizons), Down: summarize(down, horizons)})
	}
	report.Aggregate = SymbolReport{Symbol: "ALL_ALTS", Up: summarize(allUp, horizons), Down: summarize(allDown, horizons)}
	return report, nil
}

func btcImpulses(candles []model.Candle, threshold float64) []btcEvent {
	out := make([]btcEvent, 0)
	for _, candle := range candles {
		if candle.Open <= 0 {
			continue
		}
		change := candle.Close/candle.Open - 1
		if change >= threshold {
			out = append(out, btcEvent{time: candle.OpenTime.UTC().Add(time.Hour), direction: 1})
		} else if change <= -threshold {
			out = append(out, btcEvent{time: candle.OpenTime.UTC().Add(time.Hour), direction: -1})
		}
	}
	return out
}

type observation struct {
	hours    int
	directed float64
}

func observationsForAlt(candles []model.Candle, events []btcEvent, horizons []int) (up, down []observation) {
	byTime := make(map[time.Time]model.Candle, len(candles))
	for _, candle := range candles {
		byTime[candle.OpenTime.UTC()] = candle
	}
	for _, event := range events {
		entry, ok := byTime[event.time]
		if !ok || entry.Open <= 0 {
			continue
		}
		for _, hours := range horizons {
			exit, ok := byTime[event.time.Add(time.Duration(hours-1)*time.Hour)]
			if !ok || exit.Close <= 0 {
				continue
			}
			value := observation{hours: hours, directed: event.direction * (exit.Close/entry.Open - 1)}
			if event.direction > 0 {
				up = append(up, value)
			} else {
				down = append(down, value)
			}
		}
	}
	return up, down
}

func summarize(observations []observation, horizons []int) DirectionSummary {
	result := DirectionSummary{Horizons: make([]HorizonSummary, 0, len(horizons))}
	for _, hours := range horizons {
		values := make([]float64, 0)
		for _, observation := range observations {
			if observation.hours == hours {
				values = append(values, observation.directed)
			}
		}
		if hours == horizons[0] {
			result.Events = len(values)
		}
		result.Horizons = append(result.Horizons, summarizeValues(hours, values))
	}
	return result
}

func summarizeValues(hours int, values []float64) HorizonSummary {
	result := HorizonSummary{Hours: hours, Observations: len(values)}
	if len(values) == 0 {
		return result
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	for _, value := range values {
		result.MeanReturn += value
		if value > 0 {
			result.PositiveFraction++
		}
	}
	result.MeanReturn /= float64(len(values))
	result.PositiveFraction /= float64(len(values))
	middle := len(ordered) / 2
	result.MedianReturn = ordered[middle]
	if len(ordered)%2 == 0 {
		result.MedianReturn = (ordered[middle-1] + ordered[middle]) / 2
	}
	return result
}
