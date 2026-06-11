package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNewPKCEChallengeMatchesVerifier(t *testing.T) {
	p, err := newPKCE()
	if err != nil {
		t.Fatalf("newPKCE() error = %v", err)
	}
	if p.Verifier == "" || p.Challenge == "" {
		t.Fatal("expected non-empty verifier and challenge")
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Errorf("challenge = %q, want S256(verifier) = %q", p.Challenge, want)
	}
	// Two calls must not collide.
	p2, _ := newPKCE()
	if p2.Verifier == p.Verifier {
		t.Error("expected distinct verifiers across calls")
	}
}

func TestAnthropicAuthorizeURL(t *testing.T) {
	authz, err := AnthropicAuthorize()
	if err != nil {
		t.Fatalf("AnthropicAuthorize() error = %v", err)
	}
	u, err := url.Parse(authz.URL)
	if err != nil {
		t.Fatalf("authorize URL not parseable: %v", err)
	}
	q := u.Query()
	if got := q.Get("client_id"); got != anthropicClientID {
		t.Errorf("client_id = %q", got)
	}
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Errorf("code_challenge_method = %q, want S256", got)
	}
	if q.Get("code_challenge") == "" {
		t.Error("missing code_challenge")
	}
	// The verifier is echoed as state so Exchange can recover it.
	if q.Get("state") != authz.Verifier {
		t.Errorf("state = %q, want verifier %q", q.Get("state"), authz.Verifier)
	}
	if got := q.Get("redirect_uri"); got != anthropicRedirectURI {
		t.Errorf("redirect_uri = %q", got)
	}
}

func TestAnthropicExchangeParsesCodeAndState(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "acc-123",
			"refresh_token": "ref-456",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()
	swapTokenURL(t, srv.URL)

	tok, err := AnthropicExchange(context.Background(), "the-code#the-state", "the-verifier")
	if err != nil {
		t.Fatalf("AnthropicExchange() error = %v", err)
	}
	if tok.Access != "acc-123" || tok.Refresh != "ref-456" {
		t.Errorf("token = %+v", tok)
	}
	if d := time.Until(tok.Expiry); d <= 0 || d > time.Hour {
		t.Errorf("expiry not ~1h out: %v", d)
	}
	// The "code#state" form must be split into the two fields the server expects.
	if gotBody["code"] != "the-code" || gotBody["state"] != "the-state" {
		t.Errorf("code/state not split: %+v", gotBody)
	}
	if gotBody["code_verifier"] != "the-verifier" || gotBody["grant_type"] != "authorization_code" {
		t.Errorf("unexpected body: %+v", gotBody)
	}
}

func TestAnthropicRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["grant_type"] != "refresh_token" || body["refresh_token"] != "old-ref" {
			t.Errorf("unexpected refresh body: %+v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-acc",
			"refresh_token": "new-ref",
			"expires_in":    7200,
		})
	}))
	defer srv.Close()
	swapTokenURL(t, srv.URL)

	tok, err := AnthropicRefresh(context.Background(), "old-ref")
	if err != nil {
		t.Fatalf("AnthropicRefresh() error = %v", err)
	}
	if tok.Access != "new-acc" || tok.Refresh != "new-ref" {
		t.Errorf("token = %+v", tok)
	}
}

func TestAnthropicExchangeOAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "code expired",
		})
	}))
	defer srv.Close()
	swapTokenURL(t, srv.URL)

	_, err := AnthropicExchange(context.Background(), "x#y", "v")
	if err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("expected oauth error, got %v", err)
	}
}

// swapTokenURL points the token endpoint at a test server for the duration of a
// test, restoring it afterward.
func swapTokenURL(t *testing.T, u string) {
	t.Helper()
	prev := anthropicTokenURL
	anthropicTokenURL = u
	t.Cleanup(func() { anthropicTokenURL = prev })
}
