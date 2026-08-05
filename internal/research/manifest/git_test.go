package manifest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/stretchr/testify/require"
)

func TestGitRevisionIgnoresUntrackedArtifactsButDetectsSource(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		command := exec.Command("git", args...)
		command.Dir = dir
		require.NoError(t, command.Run())
	}
	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("frozen\n"), 0o644))
	git("add", "tracked.txt")
	git("commit", "-qm", "fixture")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "report.csv"), []byte("local\n"), 0o644))
	revision, err := manifest.GitRevision(dir)
	require.NoError(t, err)
	require.False(t, revision.Dirty)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "local.go"), []byte("package local\n"), 0o644))
	revision, err = manifest.GitRevision(dir)
	require.NoError(t, err)
	require.True(t, revision.Dirty)

	require.NoError(t, os.Remove(filepath.Join(dir, "local.go")))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("changed\n"), 0o644))
	revision, err = manifest.GitRevision(dir)
	require.NoError(t, err)
	require.True(t, revision.Dirty)
}
