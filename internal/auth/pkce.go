package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/webbhalsa/accessboss-cli/internal/config"
)

type authResult struct {
	accessToken string
	idToken     string
	expiresIn   int
	err         error
}

func pkceFlow(cfg config.OIDCConfig) (string, *Claims, error) {
	verifier, err := generateVerifier()
	if err != nil {
		return "", nil, fmt.Errorf("generate PKCE verifier: %w", err)
	}
	state, err := generateState()
	if err != nil {
		return "", nil, fmt.Errorf("generate state: %w", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("start local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d", port)

	ch := make(chan authResult, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		if errCode := q.Get("error"); errCode != "" {
			desc := q.Get("error_description")
			writeHTML(w, "Authentication failed", desc)
			ch <- authResult{err: fmt.Errorf("%s", friendlyAuthError(errCode, desc))}
			return
		}
		if q.Get("state") != state {
			writeHTML(w, "Authentication failed", "Invalid state parameter.")
			ch <- authResult{err: fmt.Errorf("invalid state — possible CSRF")}
			return
		}
		code := q.Get("code")
		if code == "" {
			writeHTML(w, "Authentication failed", "No authorization code received.")
			ch <- authResult{err: fmt.Errorf("no authorization code in callback")}
			return
		}

		tr, err := exchangeCode(cfg, code, redirectURI, verifier)
		if err != nil {
			writeHTML(w, "Authentication failed", err.Error())
			ch <- authResult{err: err}
			return
		}

		writeHTML(w, "Authentication successful", "You can close this tab and return to the terminal.")
		ch <- authResult{accessToken: tr.AccessToken, idToken: tr.IDToken, expiresIn: tr.ExpiresIn}
	})

	server := &http.Server{Handler: mux}
	go server.Serve(listener) //nolint:errcheck

	authURL := buildAuthURL(cfg, generateChallenge(verifier), redirectURI, state)
	fmt.Printf("Opening browser for authentication...\n")
	fmt.Printf("If your browser didn't open: %s\n\n", authURL)
	openBrowser(authURL)

	select {
	case result := <-ch:
		server.Close()
		if result.err != nil {
			return "", nil, result.err
		}
		claims, err := parseClaims(result.idToken)
		if err != nil {
			return "", nil, err
		}
		_ = saveCache(&cachedToken{
			AccessToken: result.accessToken,
			IDToken:     result.idToken,
			ExpiresAt:   time.Now().Add(time.Duration(result.expiresIn) * time.Second),
		})
		return result.accessToken, claims, nil
	case <-time.After(5 * time.Minute):
		server.Close()
		return "", nil, fmt.Errorf("authentication timed out after 5 minutes")
	}
}

func buildAuthURL(cfg config.OIDCConfig, challenge, redirectURI, state string) string {
	params := url.Values{
		"client_id":             {cfg.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {redirectURI},
		"scope":                 {"openid profile email " + cfg.APIScope},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
		"state":                 {state},
	}
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/authorize?%s",
		cfg.TenantID, params.Encode())
}

func exchangeCode(cfg config.OIDCConfig, code, redirectURI, verifier string) (*tokenResponse, error) {
	base := fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", cfg.TenantID)
	resp, err := http.PostForm(base, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {cfg.ClientID},
		"code_verifier": {verifier},
	})
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if tr.Error != "" {
		return nil, fmt.Errorf("%s", friendlyAuthError(tr.Error, tr.ErrorDescription))
	}
	return &tr, nil
}

func generateVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func generateChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(rawURL string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "linux":
		cmd = exec.Command("xdg-open", rawURL)
	default:
		return
	}
	_ = cmd.Start()
}

func writeHTML(w http.ResponseWriter, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><title>%s</title></head>`+
		`<body><h2>%s</h2><p>%s</p></body></html>`, title, title, body)
}

func friendlyAuthError(code, desc string) string {
	if desc != "" {
		return desc
	}
	return code
}
