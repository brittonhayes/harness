// Package auth stores provider credentials outside the project config so secrets
// never land in a version-controlled file. Credentials live in a single
// per-user file (~/.config/vala/auth.json, mode 0600), keyed by provider id, and
// are written by the connect flow. Environment variables still take precedence
// at load time, so CI and scripted runs need no file at all.
package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Credential is one stored provider login. Type is "api" for an API key or
// "oauth" for a subscription login (e.g. Claude Pro/Max) where no raw key is
// ever pasted. BaseURL, when set, overrides the provider's default endpoint —
// this is how a local Ollama/LM Studio server or a custom OpenAI-compatible
// gateway is remembered.
type Credential struct {
	Type    string `json:"type"`
	Key     string `json:"key,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	Model   string `json:"model,omitempty"`

	// OAuth fields, set when Type == "oauth". Access is the short-lived bearer
	// token sent on each request; Refresh renews it without another browser
	// round-trip; Expiry is the access token's expiry in Unix milliseconds, used
	// to refresh proactively. These come from the provider's OAuth flow (see
	// internal/auth/oauth) and let an operator connect with their existing
	// subscription instead of minting an API key.
	Access  string `json:"access,omitempty"`
	Refresh string `json:"refresh,omitempty"`
	Expiry  int64  `json:"expiry,omitempty"`
}

// IsOAuth reports whether the credential is a subscription (OAuth) login rather
// than a raw API key.
func (c Credential) IsOAuth() bool { return c.Type == "oauth" }

// Store is the decoded auth file. Use Load to read it and Set/Remove to mutate
// it (both persist immediately).
type Store struct {
	Providers map[string]Credential `json:"providers"`
	path      string
}

// Path returns the location of the auth file (~/.config/vala/auth.json).
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "vala", "auth.json"), nil
}

// Load reads the auth file. A missing file yields an empty, usable store;
// malformed JSON is an error so a corrupt file is not silently ignored.
func Load() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	s := &Store{Providers: map[string]Credential{}, path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, s); err != nil {
			return nil, err
		}
	}
	if s.Providers == nil {
		s.Providers = map[string]Credential{}
	}
	return s, nil
}

// Get returns the stored credential for a provider, if present.
func (s *Store) Get(provider string) (Credential, bool) {
	c, ok := s.Providers[provider]
	return c, ok
}

// All returns a copy of every stored credential, keyed by provider id.
func (s *Store) All() map[string]Credential {
	out := make(map[string]Credential, len(s.Providers))
	for k, v := range s.Providers {
		out[k] = v
	}
	return out
}

// Set records a credential for a provider and persists the file (mode 0600).
func (s *Store) Set(provider string, c Credential) error {
	if s.Providers == nil {
		s.Providers = map[string]Credential{}
	}
	s.Providers[provider] = c
	return s.save()
}

// Remove deletes a provider's credential and persists the file.
func (s *Store) Remove(provider string) error {
	delete(s.Providers, provider)
	return s.save()
}

// save writes the store to disk with restrictive permissions, creating the
// parent directory as needed.
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, append(data, '\n'), 0o600)
}
