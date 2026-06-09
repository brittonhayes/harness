package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/brittonhayes/vala/internal/brain"
)

// SaveNotion merges the provisioned Notion data-source IDs into the project's
// .vala.json, setting only the "notion" key and preserving every other key
// (model, mcp, detections_dir, …) byte-for-byte. The file is created if absent
// and pretty-printed. Secrets are never written here — they stay in the
// environment, as the config comments require.
func SaveNotion(cwd string, ids brain.DBIDs) error {
	path := filepath.Join(cwd, ".vala.json")

	// Decode into ordered-agnostic raw messages so unrelated keys round-trip
	// unchanged regardless of whether config knows about them.
	raw := map[string]json.RawMessage{}
	if data, err := os.ReadFile(path); err == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &raw); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	notion, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	raw["notion"] = notion

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}
