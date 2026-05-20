package config

import (
	_ "embed"
	"fmt"

	"gopkg.in/yaml.v3"
)

//go:embed scopes.yaml
var raw []byte

type Config struct {
	LambdaURL string               `yaml:"lambda_url"`
	StatusURL string               `yaml:"status_url"`
	OIDC      OIDCConfig           `yaml:"oidc"`
	Boundary  BoundaryServerConfig `yaml:"boundary"`
	Scopes    map[string]Scope     `yaml:"scopes"`
}

type OIDCConfig struct {
	ClientID string `yaml:"client_id"`
	TenantID string `yaml:"tenant_id"`
	APIScope string `yaml:"api_scope"`
}

type BoundaryServerConfig struct {
	Addr         string `yaml:"addr"`
	AuthMethodID string `yaml:"auth_method_id"`
	ScopeID      string `yaml:"scope_id"`
}

type Scope struct {
	Description string          `yaml:"description"`
	Boundary    *BoundaryTarget `yaml:"boundary,omitempty"`
}

func (s Scope) IsDatabase() bool {
	return s.Boundary != nil
}

type BoundaryTarget struct {
	TargetName string `yaml:"target_name"`
	ScopeID    string `yaml:"scope_id"`
}

var loaded *Config

func Load() (*Config, error) {
	if loaded != nil {
		return loaded, nil
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse embedded config: %w", err)
	}
	loaded = &cfg
	return loaded, nil
}
