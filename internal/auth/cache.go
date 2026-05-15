package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type cachedToken struct {
	AccessToken string    `json:"access_token"`
	IDToken     string    `json:"id_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".accessboss", "token.json"), nil
}

func loadCached() (*cachedToken, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var t cachedToken
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func saveCache(t *cachedToken) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
