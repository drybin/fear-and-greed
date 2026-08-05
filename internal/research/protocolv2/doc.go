// Package protocolv2 defines shared identifiers, enums, time-range
// conventions, money rounding, output paths, and schema versions for
// research validation protocol v2.
//
// Domain packages (manifest, eligibility, execution, metrics, reporting,
// orchestration) import this package and must not reverse-depend on each
// other through protocolv2.
package protocolv2

const ProtocolVersion = "research-validation-v2"
