package scanreport

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
)

//go:embed catalog/algorithms.json
var embeddedCatalog []byte

// EnsureCatalog copies the default algorithms.json into root if absent.
func EnsureCatalog(root string) error {
	path := filepath.Join(root, CatalogFile)
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return os.WriteFile(path, embeddedCatalog, 0o644)
}

// LoadCatalog reads algorithms.json from report root.
func LoadCatalog(root string) (AlgoCatalog, error) {
	path := filepath.Join(root, CatalogFile)
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			var cat AlgoCatalog
			if err := json.Unmarshal(embeddedCatalog, &cat); err != nil {
				return AlgoCatalog{}, err
			}
			return cat, nil
		}
		return AlgoCatalog{}, err
	}
	var cat AlgoCatalog
	if err := json.Unmarshal(body, &cat); err != nil {
		return AlgoCatalog{}, err
	}
	return cat, nil
}
