package tools

import (
	"crypto/rand"
	"fmt"
	"path/filepath"
)

// newUUIDv4 returns a random RFC 4122 version-4 UUID, used to mint Sigma rule
// ids without pulling in a dependency.
func newUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// byteCount renders a human-friendly byte size, e.g. "812 bytes" / "3.2 KB".
func byteCount(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d bytes", n)
	}
	return fmt.Sprintf("%.1f KB", float64(n)/1024)
}

// resolve turns a possibly-relative path into one anchored at the tool's
// working directory. Absolute paths are returned unchanged.
func resolve(dir, path string) string {
	if filepath.IsAbs(path) || dir == "" {
		return path
	}
	return filepath.Join(dir, path)
}
