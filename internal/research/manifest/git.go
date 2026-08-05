package manifest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitRevision reads HEAD and whether tracked files or untracked Go source can
// change the built evaluator. Untracked datasets and local reports are inputs
// fingerprinted elsewhere and do not make the source revision dirty.
func GitRevision(dir string) (SourceRevision, error) {
	rev, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		return SourceRevision{}, err
	}
	status, err := gitOutput(dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return SourceRevision{}, err
	}
	untrackedSource, err := gitOutput(dir, "ls-files", "--others", "--exclude-standard", "--", "*.go", "go.mod", "go.sum")
	if err != nil {
		return SourceRevision{}, err
	}
	dirty := strings.TrimSpace(status) != "" || strings.TrimSpace(untrackedSource) != ""
	return SourceRevision{GitRevision: strings.TrimSpace(rev), Dirty: dirty}, nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("manifest: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// WriteFile creates a manifest atomically. An existing file can only be reused
// if it has the identical derived identity; a changed experiment never replaces
// a previous run's manifest.
func WriteFile(path string, m Manifest) error {
	data, err := m.MarshalCanonical()
	if err != nil {
		return err
	}
	if existing, err := os.ReadFile(path); err == nil {
		old, err := Decode(existing)
		if err != nil {
			return fmt.Errorf("manifest: existing file %q is invalid: %w", path, err)
		}
		if old.ID != m.ID || old.Hash != m.Hash {
			return fmt.Errorf("manifest: refusing to overwrite %q: existing experiment %s differs from %s", path, old.ID, m.ID)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("manifest: read existing %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("manifest: create directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".manifest-*.json")
	if err != nil {
		return fmt.Errorf("manifest: create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("manifest: write temporary file: %w", err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("manifest: chmod temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("manifest: close temporary file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("manifest: install manifest: %w", err)
	}
	return nil
}
