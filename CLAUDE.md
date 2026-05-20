# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`accessboss` is a Go CLI for ephemeral AWS access management, mirroring what the [accessboss Slack bot](../accessboss) does but from the terminal. It targets the same backend (Entra ID PIM + AWS SSO) and is distributed via Homebrew from the `webbhalsa/homebrew-tap` repository.

**GitHub Repository:** git@github.com:webbhalsa/accessboss-cli.git

## Commands

```bash
go build -o accessboss .   # build binary
go build ./...             # verify all packages compile
go vet ./...               # lint
go test ./...              # run all tests
```

## Architecture

```
accessboss-cli/
├── main.go                      # entry point; declares version (overridden by GoReleaser)
├── cmd/
│   ├── root.go                  # single cobra command; full interactive flow in RunE
│   └── update.go                # fetchLatestVersion() — polls GitHub releases API
├── internal/
│   ├── auth/
│   │   ├── oidc.go              # GetToken(): checks cache, falls back to pkceFlow()
│   │   ├── pkce.go              # PKCE browser flow: local HTTP server, opens browser
│   │   ├── cache.go             # token cache at ~/.accessboss/token.json (0600)
│   │   └── claims.go            # parseClaims(): extracts name+email from ID token JWT
│   ├── boundary/
│   │   └── client.go            # GetCredentials(): re-authenticates, retries on 403
│   ├── lambda/
│   │   └── client.go            # Post(): Bearer-authenticated HTTP POST; GetStatus(): polls group membership
│   ├── config/
│   │   ├── config.go            # Load(): unmarshals embedded scopes.yaml
│   │   └── scopes.yaml          # embedded config: OIDC, Lambda URL, Boundary, scopes
│   └── tui/
│       ├── picker.go            # full-screen scope picker with type-to-filter
│       ├── duration.go          # inline duration picker (1h / 4h / 24h)
│       └── spinner.go           # animated braille spinner for blocking calls
├── .github/workflows/
│   ├── ci.yml                   # vet + build on push/PR to main
│   └── release.yml              # GoReleaser on v* tags → GitHub release + Homebrew tap
└── .goreleaser.yaml             # multi-platform builds (darwin/linux, amd64/arm64)
```

## Key Patterns

- **Version injection:** GoReleaser sets `-X main.version={{.Version}}`; defaults to `"dev"` so the update check is skipped locally.
- **Update notification:** `PickScope()` receives `fetchLatestVersion` as a callback, fires it in `Init()` as a `tea.Cmd`, and renders the result as a yellow footer inside the picker.
- **Embedded config:** `//go:embed scopes.yaml` in `internal/config/config.go` bakes the YAML into the binary at compile time. No config file on disk.
- **PKCE auth:** starts `net.Listen("tcp", "127.0.0.1:0")` for a random port, builds the Microsoft auth URL, opens the browser, and waits for the callback. Tokens cached by expiry from Entra.
- **Async Lambda:** the Lambda returns 200 (already a member) or 202 (newly granted, AWS sync running in background). On 202, `cmd/root.go` polls `GET /access/status` every 5 seconds until `AWS_SSO_{scope}` appears in the user's Identity Center groups (up to 5 minutes).
- **Boundary re-auth:** always re-authenticates before `authorize-session` to pick up freshly granted Entra group memberships. Uses `boundary targets list` to confirm access before attempting `authorize-session`; retries with fresh auth every 30 seconds for up to 3 minutes.
- **API Gateway:** the HTTP API uses JWT auth (Entra issuer v2.0, audience = CLI client ID). Lambda integration must use `payload_format_version = "1.0"` since the Lambda handler reads `event['resource']`.

## Release

Tag with `vX.Y.Z` to trigger the release workflow. GoReleaser builds for darwin/linux × amd64/arm64, publishes to GitHub Releases, and pushes a formula to `webbhalsa/homebrew-tap` using `HOMEBREW_TAP_GITHUB_TOKEN` (set as a repo secret).
