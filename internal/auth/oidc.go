package auth

import (
	"errors"
	"os"
	"time"

	"github.com/webbhalsa/accessboss-cli/internal/config"
)

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	IDToken          string `json:"id_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// GetToken returns a valid access token and the user's claims, using the
// cache when available and falling back to the PKCE browser flow.
func GetToken(cfg config.OIDCConfig) (string, *Claims, error) {
	if t, err := loadCached(); err == nil && time.Now().Before(t.ExpiresAt) {
		if claims, err := parseClaims(t.IDToken); err == nil {
			return t.AccessToken, claims, nil
		}
	}
	return pkceFlow(cfg)
}

// ClearCache removes the cached token, forcing re-authentication on next use.
func ClearCache() error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	err = os.Remove(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
