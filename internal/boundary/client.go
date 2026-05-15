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
	Username string
	Password string
}

// authorizeSessionResponse mirrors the relevant parts of the JSON output from
// `boundary targets authorize-session -format json`.
type authorizeSessionResponse struct {
	Item struct {
		Credentials []struct {
			Secret struct {
				Decoded map[string]string `json:"decoded"`
			} `json:"secret"`
		} `json:"credentials"`
	} `json:"item"`
	// present when boundary returns an error
	StatusCode int `json:"status_code"`
	APIError   *struct {
		Kind    string `json:"kind"`
		Message string `json:"message"`
	} `json:"api_error"`
}

// GetCredentials authenticates with Boundary if needed and returns ephemeral
// database credentials for the given target, retrying for up to 3 minutes to
// handle Entra PIM propagation delays.
func GetCredentials(server config.BoundaryServerConfig, target config.BoundaryTarget) (*Credentials, error) {
	if _, err := exec.LookPath("boundary"); err != nil {
		return nil, fmt.Errorf("boundary CLI not found — install it from https://developer.hashicorp.com/boundary/install")
	}

	fmt.Println("Authenticating with Boundary...")
	if err := authenticate(server); err != nil {
		return nil, fmt.Errorf("boundary authentication failed: %w", err)
	}

	printedWaiting := false
	deadline := time.Now().Add(3 * time.Minute)

	for {
		spinner := tui.StartSpinner("Fetching credentials...")
		creds, err := authorizeSession(server, target)
		spinner.Stop()

		if err == nil {
			return creds, nil
		}

		if isPermissionDenied(err) && time.Now().Before(deadline) {
			if !printedWaiting {
				fmt.Println("Waiting for Entra permissions to propagate to Boundary...")
				printedWaiting = true
			}
			wait := tui.StartSpinner("Retrying in 10 seconds...")
			time.Sleep(10 * time.Second)
			wait.Stop()
			continue
		}

		return nil, err
	}
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
		// try to get a useful error from the JSON body first
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
			return &Credentials{Username: user, Password: pass}, nil
		}
	}
	return nil, fmt.Errorf("no credentials found in boundary response")
}

func isPermissionDenied(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permissiondenied") ||
		strings.Contains(msg, "forbidden") ||
		strings.Contains(msg, "403")
}
