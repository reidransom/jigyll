// Package themearchive downloads and extracts bounded, wrapped theme archives.
// Callers own source policy, disposable working paths, and final publication.
package themearchive

// Limits bounds compressed bytes, regular-file bytes, and archive entries.
type Limits struct {
	CompressedBytes int64
	ExtractedBytes  int64
	FileBytes       int64
	Entries         int64
}

// DefaultLimits returns the theme archive resource limits.
func DefaultLimits() Limits {
	return Limits{
		CompressedBytes: 128 << 20,
		ExtractedBytes:  512 << 20,
		FileBytes:       64 << 20,
		Entries:         20_000,
	}
}
