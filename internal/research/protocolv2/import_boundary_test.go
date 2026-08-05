package protocolv2_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocolV2DoesNotImportSiblingResearchPackages(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	forbidden := []string{
		"github.com/drybin/fear-and-greed/internal/research/manifest",
		"github.com/drybin/fear-and-greed/internal/research/eligibility",
		"github.com/drybin/fear-and-greed/internal/research/execution",
		"github.com/drybin/fear-and-greed/internal/research/metrics",
		"github.com/drybin/fear-and-greed/internal/research/reporting",
		"github.com/drybin/fear-and-greed/internal/research/orchestration",
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			path := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				require.NotEqual(t, bad, path, "protocolv2 must not import %s", bad)
			}
		}
	}
}
