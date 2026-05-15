package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type Claims struct {
	Name  string
	Email string
}

func parseClaims(idToken string) (*Claims, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("parse JWT claims: %w", err)
	}

	c := &Claims{}
	if v, ok := raw["name"].(string); ok {
		c.Name = v
	}
	// Azure AD puts the UPN in preferred_username; fall back to email claim
	if v, ok := raw["preferred_username"].(string); ok {
		c.Email = v
	} else if v, ok := raw["email"].(string); ok {
		c.Email = v
	}
	return c, nil
}
