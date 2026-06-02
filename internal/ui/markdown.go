package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// markdownRenderer turns the model's markdown into a styled terminal document.
// Without it, the raw source (headings, blank lines, **bold**, list markers)
// gets dumped verbatim and reads as a wall of text with stray newlines.
type markdownRenderer struct {
	r *glamour.TermRenderer
}

// newMarkdownRenderer builds a renderer wrapped to width columns. A zero or
// oversized width is clamped to a comfortable reading measure.
func newMarkdownRenderer(width int) *markdownRenderer {
	if width <= 0 || width > 96 {
		width = 96
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return &markdownRenderer{}
	}
	return &markdownRenderer{r: r}
}

// render returns styled output for s, falling back to trimmed plain text if the
// renderer is unavailable or errors.
func (m *markdownRenderer) render(s string) string {
	if m.r == nil {
		return strings.TrimSpace(s)
	}
	out, err := m.r.Render(s)
	if err != nil {
		return strings.TrimSpace(s)
	}
	// Glamour pads with leading/trailing blank lines; trim them so spacing
	// between blocks stays under our control.
	return strings.Trim(out, "\n")
}
