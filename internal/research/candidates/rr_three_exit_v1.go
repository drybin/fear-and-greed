package candidates

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const rrThreeExitMultiple = 3.0

type rrThreeAdapter struct {
	metadata  execution.StrategyMetadata
	source    Adapter
	candidate protocolv2.ParameterCandidateID
}

func rrThreeExitAdapter(source Adapter, candidate protocolv2.ParameterCandidateID, code protocolv2.StrategyCode, name string) Adapter {
	metadata := source.Metadata()
	metadata.Ref = ref(code, "v1.0.0")
	metadata.Name = name
	metadata.Description = "Frozen source entry, stop, and time exit with the full position closed at an exact 3R target calculated from the actual entry fill."
	return rrThreeAdapter{metadata: metadata, source: source, candidate: candidate}
}

func (a rrThreeAdapter) Metadata() execution.StrategyMetadata { return a.metadata }

func (a rrThreeAdapter) Grid() []ParameterCandidate {
	return []ParameterCandidate{{ID: a.candidate, Values: map[string]any{"target_risk_multiple": rrThreeExitMultiple}}}
}

func (a rrThreeAdapter) Signals(symbol protocolv2.Symbol, candles []model.Candle, candidate protocolv2.ParameterCandidateID) ([]execution.CloseConfirmedSignal, error) {
	if candidate != a.candidate {
		return nil, fmt.Errorf("candidates: %s has no candidate %q", a.metadata.Ref, candidate)
	}
	signals, err := a.source.Signals(symbol, candles, a.candidate)
	if err != nil {
		return nil, err
	}
	for i := range signals {
		signals[i].SignalID = fmt.Sprintf("%s-%s-%d", a.metadata.Ref.Code, candidate, i)
		signals[i].Strategy = a.metadata.Ref
		signals[i].Targets = nil
		signals[i].TargetPercent = 0
		signals[i].TargetRiskMultiple = rrThreeExitMultiple
		signals[i].ExitAllAtTP1 = true
		if signals[i].Diagnostics == nil {
			signals[i].Diagnostics = map[string]float64{}
		}
		signals[i].Diagnostics["target_risk_multiple"] = rrThreeExitMultiple
		if err := signals[i].Validate(); err != nil {
			return nil, err
		}
	}
	return signals, nil
}
