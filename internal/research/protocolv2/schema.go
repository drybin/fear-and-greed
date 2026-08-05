package protocolv2

// Schema versions for protocol-v2 artifacts.
// Bump the corresponding constant when a serialized shape changes incompatibly.
const (
	ManifestSchemaVersion   = "manifest.v1"
	CheckpointSchemaVersion = "checkpoint.v1"
	ReportSchemaVersion     = "report.v1"
	FreezeSchemaVersion     = "freeze.v1"
)

// ArtifactHeader is embedded at the top of every versioned JSON artifact.
type ArtifactHeader struct {
	SchemaVersion   string `json:"schema_version"`
	ProtocolVersion string `json:"protocol_version"`
}
