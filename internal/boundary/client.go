package boundary

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/webbhalsa/accessboss-cli/internal/config"
	"github.com/webbhalsa/accessboss-cli/internal/tui"
)

type Credentials struct {
	Username    string
	Password    string
	Description string
}

type authorizeSessionResponse struct {
	Item struct {
		Credentials []struct {
			CredentialSource struct {
				Description string `json:"description"`
			} `json:"credential_source"`
			Secret struct {
				Decoded map[string]string `json:"decoded"`
			} `json:"secret"`
		} `json:"credentials"`
	} `json:"item"`
	StatusCode int `json:"status_code"`
	APIError   *struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
	} `json:"api_error"`
}

type listTargetsResponse struct {
	Items []struct {
		Name string `json:"name"`
	} `json:"items"`
}

// GetCredentials authenticates with Boundary and returns ephemeral database
// credentials for the given target, retrying for up to 3 minutes to handle
// Entra PIM propagation delays.
func GetCredentials(server config.BoundaryServerConfig, target config.BoundaryTarget) (*Credentials, error) {
	if _, err := exec.LookPath("boundary"); err != nil {
		return nil, fmt.Errorf("boundary CLI not found — install it from https://developer.hashicorp.com/boundary/install")
	}

	deadline := time.Now().Add(3 * time.Minute)

	for {
		fmt.Println("Authenticating with Boundary...")
		if err := authenticate(server); err != nil {
			return nil, fmt.Errorf("boundary authentication failed: %w", err)
		}

		spinner := tui.StartSpinner("Checking access...")
		accessible := canAccessTarget(server, target.TargetName)
		spinner.Stop()

		if !accessible {
			if time.Now().Before(deadline) {
				wait := tui.StartSpinner("Waiting for Entra permissions to propagate to Boundary...")
				time.Sleep(30 * time.Second)
				wait.Stop()
				continue
			}
			return nil, fmt.Errorf("timed out waiting for Boundary access to be provisioned")
		}

		spinner = tui.StartSpinner("Fetching credentials...")
		creds, err := authorizeSession(server, target)
		spinner.Stop()

		if err != nil {
			return nil, err
		}
		return creds, nil
	}
}

func canAccessTarget(server config.BoundaryServerConfig, targetName string) bool {
	cmd := exec.Command("boundary", "targets", "list",
		"-scope-id", server.ScopeID,
		"-recursive",
		"-format", "json",
	)
	cmd.Env = append(os.Environ(), "BOUNDARY_ADDR="+server.Addr)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false
	}

	var resp listTargetsResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return false
	}

	for _, item := range resp.Items {
		if item.Name == targetName {
			return true
		}
	}
	return false
}

func authenticate(server config.BoundaryServerConfig) error {
	cmd := exec.Command("boundary", "authenticate", "oidc",
		"-auth-method-id", server.AuthMethodID,
		"-scope-id", server.ScopeID,
	)
	cmd.Env = append(os.Environ(), "BOUNDARY_ADDR="+server.Addr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func authorizeSession(server config.BoundaryServerConfig, target config.BoundaryTarget) (*Credentials, error) {
	cmd := exec.Command("boundary", "targets", "authorize-session",
		"-name", target.TargetName,
		"-scope-id", target.ScopeID,
		"-format", "json",
	)
	cmd.Env = append(os.Environ(), "BOUNDARY_ADDR="+server.Addr)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var resp authorizeSessionResponse
		if jsonErr := json.Unmarshal(stdout.Bytes(), &resp); jsonErr == nil && resp.APIError != nil {
			return nil, fmt.Errorf("%s: %s", resp.APIError.Kind, resp.APIError.Message)
		}
		return nil, fmt.Errorf("authorize-session failed: %s", strings.TrimSpace(stderr.String()))
	}

	var resp authorizeSessionResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse boundary response: %w", err)
	}

	for _, c := range resp.Item.Credentials {
		user := c.Secret.Decoded["username"]
		pass := c.Secret.Decoded["password"]
		if user != "" && pass != "" {
			return &Credentials{Username: user, Password: pass, Description: c.CredentialSource.Description}, nil
		}
	}
	return nil, fmt.Errorf("no credentials found in boundary response")
}
