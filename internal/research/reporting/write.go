package reporting

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

// CompletedArtifact records the byte checksum of an atomically completed file.
type CompletedArtifact struct {
	Path   string               `json:"path"`
	SHA256 protocolv2.SHA256Hex `json:"sha256"`
}

// WriteJSON validates and atomically commits one complete JSON report followed
// by its checksum sidecar (<path>.sha256). Existing files are only replaced as
// complete artifacts; a crash cannot expose a partial report.
func WriteJSON(path string, artifact any) (CompletedArtifact, error) {
	if err := Validate(artifact); err != nil {
		return CompletedArtifact{}, err
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return CompletedArtifact{}, fmt.Errorf("reporting: marshal artifact: %w", err)
	}
	data = append(data, '\n')
	sum := sha256.Sum256(data)
	completed := CompletedArtifact{Path: path, SHA256: protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))}
	if err := writeAtomic(path, data); err != nil {
		return CompletedArtifact{}, err
	}
	if err := writeAtomic(path+".sha256", []byte(completed.SHA256+"\n")); err != nil {
		return CompletedArtifact{}, err
	}
	return completed, nil
}

// VerifyCompletedArtifact validates an artifact's checksum sidecar.
func VerifyCompletedArtifact(path string) (CompletedArtifact, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CompletedArtifact{}, fmt.Errorf("reporting: read artifact: %w", err)
	}
	expected, err := os.ReadFile(path + ".sha256")
	if err != nil {
		return CompletedArtifact{}, fmt.Errorf("reporting: read artifact checksum: %w", err)
	}
	sum := sha256.Sum256(data)
	actual := protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))
	if string(actual) != string(bytesTrimSpace(expected)) {
		return CompletedArtifact{}, fmt.Errorf("reporting: checksum mismatch for %s", path)
	}
	return CompletedArtifact{Path: path, SHA256: actual}, nil
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("reporting: create artifact directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("reporting: create temporary artifact: %w", err)
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("reporting: write temporary artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("reporting: sync temporary artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("reporting: close temporary artifact: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("reporting: commit artifact: %w", err)
	}
	return nil
}

func bytesTrimSpace(v []byte) []byte {
	start, end := 0, len(v)
	for start < end && (v[start] == ' ' || v[start] == '\n' || v[start] == '\r' || v[start] == '\t') {
		start++
	}
	for end > start && (v[end-1] == ' ' || v[end-1] == '\n' || v[end-1] == '\r' || v[end-1] == '\t') {
		end--
	}
	return v[start:end]
}
