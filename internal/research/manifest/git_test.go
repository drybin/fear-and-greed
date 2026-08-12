package manifest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func TestVerifyOrchestrationOnlyUpgrade(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) string {
		command := exec.Command("git", args...)
		command.Dir = dir
		output, err := command.Output()
		require.NoError(t, err)
		return strings.TrimSpace(string(output))
	}
	git("init", "-q")
	git("config", "user.email", "test@example.com")
	git("config", "user.name", "Test")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal/research/orchestration"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal/research/orchestration/orchestration.go"), []byte("package orchestration\n"), 0o644))
	git("add", ".")
	git("commit", "-qm", "frozen")
	frozen := git("rev-parse", "HEAD")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal/research/orchestration/review.go"), []byte("package orchestration\n"), 0o644))
	git("add", ".")
	git("commit", "-qm", "operational recovery")
	require.NoError(t, manifest.VerifyOrchestrationOnlyUpgrade(dir, frozen))

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal/strategy"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "internal/strategy/changed.go"), []byte("package strategy\n"), 0o644))
	git("add", ".")
	git("commit", "-qm", "evaluator change")
	require.Error(t, manifest.VerifyOrchestrationOnlyUpgrade(dir, frozen))
}
