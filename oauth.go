package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// oauthLogin walks an OAuth 2.1 + PKCE authorization-code flow against the
// FreeTodoList server and returns the issued access token.
//
// Steps, in order:
//   1. Bind a loopback TCP listener on a random port to host the redirect URI.
//   2. Register a public OAuth client at /oauth/register pinned to that
//      redirect URI. We cache the client_id keyed by redirect_uri so reruns
//      from the same port reuse it; first run on a fresh port creates one.
//   3. Generate a PKCE code_verifier + S256 challenge.
//   4. Open the user's browser to /oauth/authorize, log into the app, consent.
//   5. Loopback handler captures ?code=... and ?state=..., verifies state.
//   6. Exchange the code at /oauth/token using PKCE.
func oauthLogin(ctx context.Context, baseURL string) (token string, clientID string, err error) {
	// 1. Loopback listener on a random free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", "", fmt.Errorf("bind loopback: %w", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// 2. Register (or reuse) a client for this redirect URI.
	clientID, err = registerOauthClient(ctx, baseURL, redirectURI)
	if err != nil {
		return "", "", err
	}

	// 3. PKCE.
	verifier, err := randomURLSafe(32)
	if err != nil {
		return "", "", err
	}
	challenge := s256(verifier)
	state, err := randomURLSafe(16)
	if err != nil {
		return "", "", err
	}

	// 4 + 5. Build the authorize URL and host the callback handler.
	authURL := buildAuthorizeURL(baseURL, clientID, redirectURI, challenge, state)

	type cbResult struct {
		code  string
		state string
		err   error
	}
	resultCh := make(chan cbResult, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			desc := q.Get("error_description")
			fmt.Fprint(w, htmlMessage("Authorization failed", e+": "+desc))
			resultCh <- cbResult{err: fmt.Errorf("authorization error: %s — %s", e, desc)}
			return
		}
		code := q.Get("code")
		gotState := q.Get("state")
		if code == "" {
			fmt.Fprint(w, htmlMessage("Missing code", "No authorization code in callback."))
			resultCh <- cbResult{err: errors.New("callback missing code")}
			return
		}
		fmt.Fprint(w, htmlMessage("You're logged in.", "You can close this tab and return to the terminal."))
		resultCh <- cbResult{code: code, state: gotState}
	})

	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go srv.Serve(ln)
	defer srv.Shutdown(context.Background())

	// 6. Tell the user, open the browser, wait.
	fmt.Println("Opening browser to log in…")
	fmt.Println("If it doesn't open automatically, visit:")
	fmt.Println("  " + authURL)
	if err := openBrowser(authURL); err != nil {
		// non-fatal — user can copy/paste the URL.
		fmt.Println("(could not auto-open browser:", err, ")")
	}

	select {
	case <-ctx.Done():
		return "", "", ctx.Err()
	case res := <-resultCh:
		if res.err != nil {
			return "", "", res.err
		}
		if res.state != state {
			return "", "", fmt.Errorf("state mismatch (CSRF guard): got %q want %q", res.state, state)
		}

		// 7. Exchange the code for a token.
		token, err := exchangeCode(ctx, baseURL, clientID, redirectURI, res.code, verifier)
		if err != nil {
			return "", "", err
		}
		return token, clientID, nil
	case <-time.After(5 * time.Minute):
		return "", "", errors.New("login timed out after 5 minutes")
	}
}

func buildAuthorizeURL(baseURL, clientID, redirectURI, challenge, state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("scope", "api")
	return strings.TrimRight(baseURL, "/") + "/oauth/authorize?" + q.Encode()
}

func registerOauthClient(ctx context.Context, baseURL, redirectURI string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"client_name":   "FreeTodoList CLI",
		"redirect_uris": []string{redirectURI},
		"client_uri":    "https://freetodolist.com",
	})

	req, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimRight(baseURL, "/")+"/oauth/register",
		bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("register client: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("register client: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		ClientID string `json:"client_id"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode register response: %w", err)
	}
	if out.ClientID == "" {
		return "", errors.New("register response missing client_id")
	}
	return out.ClientID, nil
}

func exchangeCode(ctx context.Context, baseURL, clientID, redirectURI, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", verifier)

	req, err := http.NewRequestWithContext(ctx, "POST",
		strings.TrimRight(baseURL, "/")+"/oauth/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token exchange: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var out struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("decode token response: %w (body: %s)", err, string(respBody))
	}
	if out.Error != "" {
		return "", fmt.Errorf("token error: %s — %s", out.Error, out.ErrorDesc)
	}
	if out.AccessToken == "" {
		return "", errors.New("token response missing access_token")
	}
	return out.AccessToken, nil
}

// randomURLSafe returns n bytes of crypto-random as base64url (no padding).
func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// s256 returns the base64url(sha256(verifier)) PKCE S256 challenge.
func s256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// openBrowser opens a URL in the user's default browser.
func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

// htmlMessage renders a tiny self-contained page shown to the user in the
// browser after the redirect lands.
func htmlMessage(title, body string) string {
	return `<!doctype html>
<meta charset="utf-8">
<title>` + htmlEscape(title) + `</title>
<style>
  body { font-family: -apple-system, system-ui, sans-serif; padding: 4em 2em; max-width: 560px; margin: auto; color: #222; }
  h1 { font-size: 1.4em; }
  p { color: #555; }
</style>
<h1>` + htmlEscape(title) + `</h1>
<p>` + htmlEscape(body) + `</p>
`
}

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return r.Replace(s)
}
