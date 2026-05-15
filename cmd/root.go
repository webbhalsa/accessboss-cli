package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

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
			return nil // user cancelled
		}

		duration, err := tui.PickDuration()
		if err != nil {
			return err
		}
		if duration == "" {
			return nil // user cancelled
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
		postErr := lambda.Post(cfg.LambdaURL, token, lambda.RequestBody{
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
				return fmt.Errorf("timed out waiting for Entra — access may still be granted, check AWS SSO")
			}
			return postErr
		}

		fmt.Printf("Access granted: %s\n", scopeDef.Description)

		if scopeDef.IsDatabase() {
			fmt.Println("Fetching database credentials from Boundary...")
			creds, err := boundary.GetCredentials(cfg.Boundary, *scopeDef.Boundary)
			if err != nil {
				return fmt.Errorf("boundary: %w", err)
			}
			printDBCredentials(creds)
		}

		return nil
	},
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

