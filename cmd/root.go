package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/webbhalsa/accessboss-cli/internal/auth"
	"github.com/webbhalsa/accessboss-cli/internal/boundary"
	"github.com/webbhalsa/accessboss-cli/internal/config"
	"github.com/webbhalsa/accessboss-cli/internal/lambda"
	"github.com/webbhalsa/accessboss-cli/internal/tui"
)

var currentVersion string

var rootCmd = &cobra.Command{
	Use:   "accessboss",
	Short: "Ephemeral AWS access management from the command line",
	CompletionOptions: cobra.CompletionOptions{
		DisableDefaultCmd: true,
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		var dbItems, awsItems []tui.ScopeItem
		for name, scope := range cfg.Scopes {
			item := tui.ScopeItem{Name: name, Desc: scope.Description, IsDatabase: scope.IsDatabase()}
			if scope.IsDatabase() {
				dbItems = append(dbItems, item)
			} else {
				awsItems = append(awsItems, item)
			}
		}
		sort.Slice(dbItems, func(i, j int) bool { return dbItems[i].Name < dbItems[j].Name })
		sort.Slice(awsItems, func(i, j int) bool { return awsItems[i].Name < awsItems[j].Name })
		scopes := append(dbItems, awsItems...)

		chosen, err := tui.PickScope(scopes, currentVersion, fetchLatestVersion)
		if err != nil {
			return err
		}
		if chosen == "" {
			return nil
		}

		duration, err := tui.PickDuration()
		if err != nil {
			return err
		}
		if duration == "" {
			return nil
		}

		reason, err := promptReason()
		if err != nil {
			return err
		}
		if reason == "" {
			return fmt.Errorf("reason cannot be empty")
		}

		token, claims, err := auth.GetToken(cfg.OIDC)
		if err != nil {
			return fmt.Errorf("authenticate: %w", err)
		}

		scopeDef := cfg.Scopes[chosen]

		spinner := tui.StartSpinner(fmt.Sprintf("Requesting %s access to %q as %s... 🐌", duration, chosen, claims.Name))
		result, postErr := lambda.Post(cfg.LambdaURL, token, lambda.RequestBody{
			Duration:       duration,
			MessageID:      "N/A",
			Reason:         reason,
			Requester:      claims.Name,
			RequesterEmail: claims.Email,
			Scope:          chosen,
		})
		spinner.Stop()

		if postErr != nil {
			if errors.Is(postErr, lambda.ErrTimeout) {
				return fmt.Errorf("timed out waiting for Entra — access may still be granted, keep an eye on #aws-access-requests in Teams")
			}
			return postErr
		}

		if result.AlreadyMember {
			fmt.Printf("Access granted: %s\n", scopeDef.Description)
		} else {
			if err := pollForGroup(cfg.StatusURL, token, "AWS_SSO_"+chosen); err != nil {
				return err
			}
			fmt.Printf("Access granted: %s\n", scopeDef.Description)
		}

		if scopeDef.IsDatabase() {
			creds, err := boundary.GetCredentials(cfg.Boundary, *scopeDef.Boundary)
			if err != nil {
				return fmt.Errorf("boundary: %w", err)
			}
			printDBCredentials(creds)
		}

		return nil
	},
}

func pollForGroup(statusURL, token, expectedGroup string) error {
	spinner := tui.StartSpinner("Syncing access from Entra to AWS...")
	defer spinner.Stop()

	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		status, err := lambda.GetStatus(statusURL, token)
		if err == nil {
			for _, g := range status.Groups {
				if g == expectedGroup {
					return nil
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("timed out waiting for AWS provisioning — check #aws-access-requests in Teams")
}

func printDBCredentials(creds *boundary.Credentials) {
	fmt.Println()
	fmt.Println("Database credentials (valid for 1 hour):")
	fmt.Printf("  Username: %s\n", creds.Username)
	fmt.Printf("  Password: %s\n", creds.Password)
	fmt.Println()
}

func promptReason() (string, error) {
	fmt.Print("Reason: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read reason: %w", err)
	}
	return strings.TrimSpace(line), nil
}

func Execute(version string) {
	currentVersion = version
	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
