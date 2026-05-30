package core

// VerboseIDs enables optional ID attributes in structured logs.
var VerboseIDs bool

// AppendVerboseIDs returns kv when VerboseIDs is enabled, otherwise nil.
// kv must be alternating key, value pairs suitable for slog.
func AppendVerboseIDs(kv ...any) []any {
	if !VerboseIDs {
		return nil
	}
	return kv
}
