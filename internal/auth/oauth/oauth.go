// Package oauth implements the browser-based authorization flows that let an
// operator connect a model provider with an existing subscription instead of a
// raw API key. The first supported flow is Anthropic's Claude Pro/Max login,
// the same PKCE flow Claude Code and opencode use: vala opens a consent page in
// the browser, the operator pastes back the one-time code, and vala exchanges
// it for an access/refresh token pair that is stored (mode 0600) and refreshed
// automatically as it nears expiry.
//
// The package is deliberately provider-shaped — each provider that supports a
// login flow exposes an Authorize/Exchange/Refresh trio — so additional
// providers can be added without touching the credential store or the agent.
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// Token is the result of an OAuth exchange or refresh: a short-lived access
// token used for inference, a refresh token to renew it, and the access token's
// absolute expiry.
type Token struct {
	Access  string
	Refresh string
	Expiry  time.Time
}

// Authorization is a pending login: the URL to open in a browser and the PKCE
// verifier that must be supplied back to Exchange to complete it. Verifier is a
// secret — it is held only in memory for the duration of the connect flow.
type Authorization struct {
	URL      string
	Verifier string
}

// pkce holds a freshly generated PKCE verifier/challenge pair (RFC 7636, S256).
type pkce struct {
	Verifier  string
	Challenge string
}

// newPKCE generates a high-entropy verifier and its S256 challenge. Both are
// URL-safe, unpadded base64 so they pass through query strings untouched.
func newPKCE() (pkce, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return pkce{}, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return pkce{Verifier: verifier, Challenge: challenge}, nil
}
