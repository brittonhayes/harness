package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Anthropic's public OAuth client for first-party CLIs. It is not a secret — the
// PKCE verifier, not a client secret, is what proves possession of the login —
// and is the same well-known client id Claude Code uses, which is what makes a
// Claude Pro/Max subscription usable for inference here.
const anthropicClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"

const (
	// anthropicAuthorizeURL is the consent page opened in the browser. claude.ai
	// is the Pro/Max subscription login; the console host is used for the API-key
	// (developer) variant.
	anthropicAuthorizeURL = "https://claude.ai/oauth/authorize"
	// anthropicRedirectURI is the out-of-band callback that displays the one-time
	// code for the operator to copy back into the terminal — no local web server
	// or open port required.
	anthropicRedirectURI = "https://console.anthropic.com/oauth/code/callback"
	// anthropicScopes request inference plus profile so the token can drive the
	// Messages API on the operator's behalf.
	anthropicScopes = "org:create_api_key user:profile user:inference"
)

// anthropicTokenURL exchanges and refreshes tokens. It is a var, not a const,
// so tests can point the exchange/refresh round-trip at a local server.
var anthropicTokenURL = "https://console.anthropic.com/v1/oauth/token"

// AnthropicAuthorize begins a Claude Pro/Max login. It returns the consent URL
// to open in a browser and the PKCE verifier to pass back to AnthropicExchange.
func AnthropicAuthorize() (Authorization, error) {
	p, err := newPKCE()
	if err != nil {
		return Authorization{}, err
	}
	q := url.Values{
		"code":                  {"true"},
		"client_id":             {anthropicClientID},
		"response_type":         {"code"},
		"redirect_uri":          {anthropicRedirectURI},
		"scope":                 {anthropicScopes},
		"code_challenge":        {p.Challenge},
		"code_challenge_method": {"S256"},
		// The verifier doubles as the state value, echoed back appended to the
		// code so Exchange can recover it from a single pasted string.
		"state": {p.Verifier},
	}
	return Authorization{
		URL:      anthropicAuthorizeURL + "?" + q.Encode(),
		Verifier: p.Verifier,
	}, nil
}

// AnthropicExchange trades the pasted authorization code for a token pair. The
// out-of-band flow hands the operator a value shaped "<code>#<state>"; either
// the full string or the bare code is accepted.
func AnthropicExchange(ctx context.Context, code, verifier string) (Token, error) {
	code = strings.TrimSpace(code)
	state := ""
	if i := strings.IndexByte(code, '#'); i >= 0 {
		state = code[i+1:]
		code = code[:i]
	}
	body := map[string]string{
		"code":          code,
		"state":         state,
		"grant_type":    "authorization_code",
		"client_id":     anthropicClientID,
		"redirect_uri":  anthropicRedirectURI,
		"code_verifier": verifier,
	}
	return anthropicTokenRequest(ctx, body)
}

// AnthropicRefresh renews an access token using its refresh token. The provider
// may rotate the refresh token, so callers must persist whatever is returned.
func AnthropicRefresh(ctx context.Context, refresh string) (Token, error) {
	body := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refresh,
		"client_id":     anthropicClientID,
	}
	return anthropicTokenRequest(ctx, body)
}

// AnthropicBetaHeader is the value the Messages API requires on requests
// authenticated with an OAuth token rather than an API key.
const AnthropicBetaHeader = "oauth-2025-04-20"

// anthropicTokenResponse is the subset of the token endpoint's JSON we use.
type anthropicTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// anthropicTokenRequest posts a JSON body to the token endpoint and decodes the
// token pair, mapping both transport and OAuth-level errors to a clear message.
func anthropicTokenRequest(ctx context.Context, body map[string]string) (Token, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return Token{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicTokenURL, bytes.NewReader(buf))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("oauth token request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Token{}, err
	}

	var tr anthropicTokenResponse
	if err := json.Unmarshal(raw, &tr); err != nil {
		return Token{}, fmt.Errorf("oauth token response (%s): %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	if tr.Error != "" {
		msg := tr.Error
		if tr.ErrorDesc != "" {
			msg += ": " + tr.ErrorDesc
		}
		return Token{}, fmt.Errorf("oauth: %s", msg)
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("oauth token request failed: %s", resp.Status)
	}
	if tr.AccessToken == "" {
		return Token{}, fmt.Errorf("oauth: token endpoint returned no access token")
	}

	expiry := time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	if tr.ExpiresIn == 0 {
		// Be conservative when the server omits a lifetime: assume one hour so the
		// token is refreshed rather than used indefinitely.
		expiry = time.Now().Add(time.Hour)
	}
	return Token{
		Access:  tr.AccessToken,
		Refresh: tr.RefreshToken,
		Expiry:  expiry,
	}, nil
}
