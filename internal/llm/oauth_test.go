package llm

import (
	"testing"
	"time"

	"github.com/brittonhayes/vala/internal/auth"
)

func TestFreshOAuthTokenUsesValidToken(t *testing.T) {
	// A token comfortably ahead of the refresh window is returned as-is, with no
	// network call. store is nil to prove the refresh path is never taken.
	cred := auth.Credential{
		Type:    "oauth",
		Access:  "still-good",
		Refresh: "r",
		Expiry:  time.Now().Add(time.Hour).UnixMilli(),
	}
	tok, err := freshOAuthToken(nil, "anthropic", cred)
	if err != nil {
		t.Fatalf("freshOAuthToken() error = %v", err)
	}
	if tok != "still-good" {
		t.Errorf("token = %q, want still-good", tok)
	}
}

func TestFreshOAuthTokenNoRefreshFallsBackToAccess(t *testing.T) {
	// Expired but no refresh token: fall back to whatever access token we have
	// rather than failing, leaving the API to reject it if truly dead.
	cred := auth.Credential{
		Type:   "oauth",
		Access: "expired-but-present",
		Expiry: time.Now().Add(-time.Hour).UnixMilli(),
	}
	tok, err := freshOAuthToken(nil, "anthropic", cred)
	if err != nil {
		t.Fatalf("freshOAuthToken() error = %v", err)
	}
	if tok != "expired-but-present" {
		t.Errorf("token = %q", tok)
	}
}

func TestFreshOAuthTokenNoCredentials(t *testing.T) {
	// No access and no refresh is the "not connected" condition.
	_, err := freshOAuthToken(nil, "anthropic", auth.Credential{Type: "oauth"})
	if err == nil {
		t.Fatal("expected error for empty oauth credential")
	}
}

func TestCredentialIsOAuth(t *testing.T) {
	if !(auth.Credential{Type: "oauth"}).IsOAuth() {
		t.Error("oauth credential should report IsOAuth")
	}
	if (auth.Credential{Type: "api"}).IsOAuth() {
		t.Error("api credential should not report IsOAuth")
	}
}

func TestNewAnthropicOAuthPrependsIdentity(t *testing.T) {
	// The OAuth client must present Claude Code's identity ahead of vala's own
	// system prompt; the API-key client must not.
	oauthP := newAnthropicOAuth("tok", "", "claude-opus-4-8", 1000, 200000)
	blocks := oauthP.systemBlocks("vala instructions")
	if len(blocks) != 2 || blocks[0].Text != anthropicCodeIdentity || blocks[1].Text != "vala instructions" {
		t.Errorf("oauth system blocks = %+v", blocks)
	}

	keyP := newAnthropic("sk-test", "", "claude-opus-4-8", 1000, 200000)
	if b := keyP.systemBlocks("vala instructions"); len(b) != 1 || b[0].Text != "vala instructions" {
		t.Errorf("api-key system blocks = %+v", b)
	}
}
