// Package reference embeds a curated set of gold-standard Sigma detections,
// adapted from SigmaHQ and augmented with an inline `runbook:` and executable
// `tests:`. They are surfaced to the agent on demand (via the reference_detection
// tool) as worked examples of what a respondable, review-proof, testable rule
// looks like — so authoring guidance is shown, not just told.
package reference

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed sigma/*.yml
var fsys embed.FS

// Meta is a compact summary of one reference detection for the index listing.
type Meta struct {
	Name        string // file stem, e.g. "aws_cloudtrail_disable_logging"
	Title       string
	Level       string
	Tags        []string
	Description string // first line of the description
}

// header is the subset of fields parsed for the index.
type header struct {
	Title       string   `yaml:"title"`
	Level       string   `yaml:"level"`
	Tags        []string `yaml:"tags"`
	Description string   `yaml:"description"`
}

// List returns the metadata for every embedded reference detection, sorted by
// name.
func List() ([]Meta, error) {
	entries, err := fs.ReadDir(fsys, "sigma")
	if err != nil {
		return nil, err
	}
	var out []Meta
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fsys.ReadFile("sigma/" + e.Name())
		if err != nil {
			return nil, err
		}
		var h header
		if err := yaml.Unmarshal(data, &h); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		out = append(out, Meta{
			Name:        stem(e.Name()),
			Title:       h.Title,
			Level:       h.Level,
			Tags:        h.Tags,
			Description: firstLine(h.Description),
		})
	}
	return out, nil
}

// Get returns the raw YAML for a reference detection by name (file stem, with
// or without extension).
func Get(name string) ([]byte, error) {
	name = stem(name)
	data, err := fsys.ReadFile("sigma/" + name + ".yml")
	if err != nil {
		return nil, fmt.Errorf("no reference detection named %q", name)
	}
	return data, nil
}

// Files returns every embedded reference path and its bytes (used by the guard
// test to validate and run all of them).
func Files() (map[string][]byte, error) {
	entries, err := fs.ReadDir(fsys, "sigma")
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := fsys.ReadFile("sigma/" + e.Name())
		if err != nil {
			return nil, err
		}
		out["sigma/"+e.Name()] = data
	}
	return out, nil
}

func stem(name string) string {
	name = strings.TrimPrefix(name, "sigma/")
	name = strings.TrimSuffix(name, ".yml")
	name = strings.TrimSuffix(name, ".yaml")
	return name
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
