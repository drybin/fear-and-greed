package protocolv2

import (
	"path/filepath"
	"strings"
)

// Default root directory names under an experiment output root.
const (
	DirManifests   = "manifests"
	DirCheckpoints = "checkpoints"
	DirReports     = "reports"
	DirFills       = "fills"
	DirTrades      = "trades"
	DirEquity      = "equity"
	DirEligibility = "eligibility"
	DirLogs        = "logs"
	DirFreeze      = "freeze"
	DirHoldout     = "holdout"
)

// ExperimentRoot returns <base>/protocol-v2/<experimentID>.
// Legacy scan reports must not be written under this tree.
func ExperimentRoot(baseDir string, id ExperimentID) string {
	return filepath.Join(filepath.Clean(baseDir), "protocol-v2", string(id))
}

// ManifestPath is the frozen manifest JSON for an experiment.
func ManifestPath(root string) string {
	return filepath.Join(root, DirManifests, "manifest.json")
}

// CheckpointDir stores atomic per-unit checkpoints.
func CheckpointDir(root string) string {
	return filepath.Join(root, DirCheckpoints)
}

// ReportDir stores versioned research reports.
func ReportDir(root string) string {
	return filepath.Join(root, DirReports)
}

// FreezeDir stores the immutable candidate bundle after development.
func FreezeDir(root string) string {
	return filepath.Join(root, DirFreeze)
}

// HoldoutDir stores final-phase holdout artifacts only.
func HoldoutDir(root string) string {
	return filepath.Join(root, DirHoldout)
}

// IsProtocolV2Path reports whether p is under a protocol-v2 experiment tree.
func IsProtocolV2Path(p string) bool {
	clean := filepath.ToSlash(filepath.Clean(p))
	return strings.Contains(clean, "/protocol-v2/") || strings.HasSuffix(clean, "/protocol-v2")
}
