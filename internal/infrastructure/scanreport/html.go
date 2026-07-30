package scanreport

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

func dataDirFromManifest(m Manifest) string {
	if m.Options != nil {
		if d, ok := m.Options["data_dir"].(string); ok && strings.TrimSpace(d) != "" {
			return strings.TrimSpace(d)
		}
	}
	return "data"
}

//go:embed catalog/report.html.tmpl
var reportHTMLTemplate string

//go:embed catalog/chart.html.tmpl
var chartHTMLTemplate string

// GenerateHTML builds a self-contained comparison report.
func GenerateHTML(reportRoot, htmlPath string) error {
	results, err := LoadAllResults(reportRoot)
	if err != nil {
		return err
	}
	catalog, err := LoadCatalog(reportRoot)
	if err != nil {
		return err
	}
	var manifest Manifest
	manifestPath := filepath.Join(reportRoot, ManifestFile)
	if body, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(body, &manifest)
	}
	dataDir := dataDirFromManifest(manifest)
	EnsureOHLC(reportRoot, dataDir, results, DefaultChartIntervalMin)

	payload := struct {
		Results  []Result    `json:"results"`
		Catalog  AlgoCatalog `json:"catalog"`
		Manifest Manifest    `json:"manifest"`
	}{
		Results:  results,
		Catalog:  catalog,
		Manifest: manifest,
	}
	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if htmlPath == "" {
		htmlPath = filepath.Join(reportRoot, DefaultHTML)
	}
	dir := filepath.Dir(htmlPath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	f, err := os.Create(htmlPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	tpl, err := template.New("report").Parse(reportHTMLTemplate)
	if err != nil {
		return err
	}
	if err := tpl.Execute(f, map[string]template.JS{
		"DataJSON": template.JS(dataJSON),
	}); err != nil {
		return err
	}

	chartPath := filepath.Join(reportRoot, DefaultChartHTML)
	cf, err := os.Create(chartPath)
	if err != nil {
		return err
	}
	defer func() { _ = cf.Close() }()
	ctpl, err := template.New("chart").Parse(chartHTMLTemplate)
	if err != nil {
		return err
	}
	return ctpl.Execute(cf, nil)
}

// DefaultHTMLPath returns the default HTML path under report root.
func DefaultHTMLPath(reportRoot string) string {
	return filepath.Join(reportRoot, DefaultHTML)
}

// DefaultChartHTMLPath returns the fullscreen chart page path under report root.
func DefaultChartHTMLPath(reportRoot string) string {
	return filepath.Join(reportRoot, DefaultChartHTML)
}

// ResolveHTMLPath normalizes html output path.
func ResolveHTMLPath(reportRoot, htmlFlag string) string {
	htmlFlag = strings.TrimSpace(htmlFlag)
	if htmlFlag == "" {
		return ""
	}
	if htmlFlag == "true" || htmlFlag == "1" {
		return DefaultHTMLPath(reportRoot)
	}
	if !filepath.IsAbs(htmlFlag) && !strings.Contains(htmlFlag, string(os.PathSeparator)) {
		return filepath.Join(reportRoot, htmlFlag)
	}
	return htmlFlag
}

// ValidateReportSetup checks report dir / html flags.
func ValidateReportSetup(reportDir, htmlFlag string) (string, string, error) {
	reportDir = strings.TrimSpace(reportDir)
	htmlFlag = strings.TrimSpace(htmlFlag)
	if reportDir == "" && htmlFlag != "" {
		reportDir = "reports"
	}
	if reportDir == "" {
		return "", "", nil
	}
	htmlPath := ResolveHTMLPath(reportDir, htmlFlag)
	return reportDir, htmlPath, nil
}
