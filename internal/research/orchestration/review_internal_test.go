package orchestration

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

func TestReadCheckpointEvidenceStreamsLargeUnusedArrays(t *testing.T) {
	unit := Unit{
		Strategy:  protocolv2.StrategyRef{Code: "test", Version: "v1"},
		Candidate: "default",
		Fold:      "fold-001-test",
		Cost:      "base",
	}
	points := make([]map[string]float64, 20_000)
	for i := range points {
		points[i] = map[string]float64{"equity": float64(i)}
	}
	raw, err := json.Marshal(map[string]any{
		"unit": unit,
		"symbols": []map[string]any{{
			"symbol":       "BTCUSDT",
			"metrics":      map[string]float64{"max_drawdown": 0.12},
			"final_equity": 125,
			"trades":       []any{},
			"equity":       points,
			"audit":        points,
		}},
		"aggregate_trade_count": 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	path := filepath.Join(protocolv2.CheckpointDir(root), unit.Key()+".json.artifact")
	digest, err := writeCompressedArtifact(path, raw)
	if err != nil {
		t.Fatal(err)
	}
	summary, err := readCheckpointEvidence(root, Checkpoint{Unit: unit, ArtifactSHA256: digest}, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Unit.Key() != unit.Key() || summary.NetPnL != 25 || summary.EligibleSymbols != 1 || summary.MedianDrawdownPercent != 12 {
		t.Fatalf("unexpected streamed summary: %+v", summary)
	}
}
