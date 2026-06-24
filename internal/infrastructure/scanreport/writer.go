package scanreport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	DataSubdir     = "data"
	ManifestFile   = "manifest.json"
	CatalogFile    = "algorithms.json"
	DefaultHTML    = "report.html"
	DefaultChartHTML = "chart.html"
)

// Writer persists scan results under root/data/<algo>/.
type Writer struct {
	root      string // report root (e.g. reports/)
	manifest  Manifest
	algoRuns  map[string]*AlgoRunSummary
	now       time.Time
}

// NewWriter creates a writer; ensures root and copies embedded catalog if missing.
func NewWriter(root string) (*Writer, error) {
	if root == "" {
		return nil, fmt.Errorf("report root is empty")
	}
	if err := os.MkdirAll(filepath.Join(root, DataSubdir), 0o755); err != nil {
		return nil, err
	}
	if err := EnsureCatalog(root); err != nil {
		return nil, err
	}
	return &Writer{
		root: root,
		manifest: Manifest{
			DataDir:    DataSubdir + "/",
			Algorithms: make(map[string]AlgoRunSummary),
		},
		algoRuns: make(map[string]*AlgoRunSummary),
		now:      time.Now().UTC(),
	}, nil
}

// Root returns the report directory passed to NewWriter.
func (w *Writer) Root() string {
	return w.root
}

// ClearAlgo removes all JSON files for an algorithm before a new partial run.
func (w *Writer) ClearAlgo(algo string) error {
	dir := filepath.Join(w.root, DataSubdir, algo)
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// Save writes one result JSON and updates in-memory manifest.
func (w *Writer) Save(r Result) error {
	if r.UpdatedAt == "" {
		r.UpdatedAt = w.now.Format(time.RFC3339)
	}
	dir := filepath.Join(w.root, DataSubdir, r.Algo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	name := fmt.Sprintf("%s__%s.json", sanitizeFilePart(r.Symbol), sanitizeFilePart(r.Period))
	path := filepath.Join(dir, name)
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return err
	}

	sum := w.algoRuns[r.Algo]
	if sum == nil {
		sum = &AlgoRunSummary{UpdatedAt: r.UpdatedAt}
		w.algoRuns[r.Algo] = sum
	}
	sum.UpdatedAt = r.UpdatedAt
	sum.Symbols = appendUnique(sum.Symbols, r.Symbol)
	sum.Periods = appendUnique(sum.Periods, r.Period)
	return nil
}

// FinishManifest writes manifest.json with run options.
func (w *Writer) FinishManifest(options map[string]interface{}) error {
	w.manifest.LastRunAt = w.now.Format(time.RFC3339)
	w.manifest.Options = options
	w.manifest.Algorithms = make(map[string]AlgoRunSummary, len(w.algoRuns))
	for algo, s := range w.algoRuns {
		sort.Strings(s.Symbols)
		sort.Strings(s.Periods)
		w.manifest.Algorithms[algo] = *s
	}
	path := filepath.Join(w.root, ManifestFile)
	body, err := json.MarshalIndent(w.manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o644)
}

// LoadAllResults reads every result JSON under data/.
func LoadAllResults(root string) ([]Result, error) {
	dataRoot := filepath.Join(root, DataSubdir)
	var out []Result
	err := filepath.Walk(dataRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".json") || IsOHLCFile(info.Name()) {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var r Result
		if err := json.Unmarshal(body, &r); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

func sanitizeFilePart(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return s
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}
