# accessboss

A CLI for requesting ephemeral AWS access — pick a scope, give a reason, and get access provisioned in seconds.

## Installation

```bash
brew tap webbhalsa/tap
brew install accessboss
```

To upgrade to the latest version:

```bash
brew upgrade accessboss
```

## Usage

Run `accessboss` to open an interactive scope picker:

```
accessboss
```

This will:
1. Show a filterable list of available access scopes — type to filter, arrow keys to navigate, Enter to select
2. Pick a duration (1 hour, 4 hours, or 24 hours)
3. Prompt for a reason
4. Open a browser to authenticate with your Kry account
5. Request access via the AccessBoss Lambda
6. For database scopes: authenticate with Boundary and print ephemeral credentials

### Update notifications

If a newer version is available, a banner is shown as a footer in the scope picker:

```
Update available: v1.0.0 → v1.1.0  Run: brew upgrade accessboss
```

## Database access

For database scopes, accessboss provisions ephemeral credentials via [HashiCorp Boundary](https://www.boundaryproject.io/) after granting the Entra PIM access. The flow is:

1. The Lambda grants access by adding you to the relevant Entra group
2. `boundary authenticate oidc` opens a browser — sign in with your Kry account to get a Boundary token with your current group memberships
3. accessboss checks if the requested target is visible to you (`boundary targets list`) — if not, Entra hasn't propagated to Boundary yet
4. If the target isn't visible yet, accessboss waits 30 seconds, re-authenticates (fresh browser login to pick up the new membership), and retries — giving up after 3 minutes
5. Once the target is visible, `boundary targets authorize-session` fetches an ephemeral username and password (valid for 1 hour)

The re-authentication on each retry is necessary because the Boundary token is a snapshot of group memberships at login time — a stale token won't reflect newly granted access.

Boundary must be installed: `brew install boundary`

## Access scopes

Scopes are baked into the binary. The list includes both plain AWS resource scopes (`kms`, `s3`, `compute`, etc.) and database scopes (`prod_main_db_read_only`, `prod_fr_db_read_write`, etc.) which additionally provision Boundary credentials.

## Requirements

Your Kry account must be assigned to the `accessboss-cli` app in Entra ID. If you get an authentication error, ask in **#platform-tools** to be added.

---

## Releasing a new version

1. Merge changes to `main`.
2. Tag and push:
   ```bash
   git tag v1.0.0
   git push origin main --tags
   ```
3. The [Release workflow](https://github.com/webbhalsa/accessboss-cli/actions/workflows/release.yml) builds binaries for macOS and Linux and publishes a GitHub Release. The Homebrew formula in [webbhalsa/homebrew-tap](https://github.com/webbhalsa/homebrew-tap) is updated automatically.

Tags must follow [semver](https://semver.org/) and start with `v`. The repository needs a `HOMEBREW_TAP_GITHUB_TOKEN` secret with write access to the tap repo.
