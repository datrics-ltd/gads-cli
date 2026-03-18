# gads-cli — Technical Specification

## Overview

A cross-platform CLI for the Google Ads API, built in Go. Distributed as single binaries for Linux, macOS, and Windows. Designed for both human operators (formatted tables) and automation/AI agents (JSON, CSV output).

---

## Table of Contents

1. [Architecture](#architecture)
2. [Authentication](#authentication)
3. [Configuration](#configuration)
4. [Commands](#commands)
5. [Output Formatting](#output-formatting)
6. [Error Handling](#error-handling)
7. [Distribution & Installation](#distribution--installation)
8. [CI/CD](#cicd)
9. [Project Structure](#project-structure)
10. [Dependencies](#dependencies)
11. [Implementation Phases](#implementation-phases)

---

## Architecture

### Language & Framework

- **Language:** Go (latest stable)
- **CLI Framework:** [Cobra](https://github.com/spf13/cobra) — industry standard (used by kubectl, gh, docker)
- **Config:** [Viper](https://github.com/spf13/viper) — pairs with Cobra, handles YAML config + env vars + flags

### Design Principles

- Every command is stateless — reads config/credentials, makes API call(s), outputs result, exits
- No daemon, no background processes, no local database
- All output goes to stdout (data) or stderr (logs, errors, progress)
- Exit codes: 0 = success, 1 = general error, 2 = auth error, 3 = API error
- Composable with Unix tools (`jq`, `awk`, `grep`, pipes)

---

## Authentication

Google Ads API requires **two layers** of auth on every request:

### 1. Developer Token (Static)

- Issued by Google after API access approval
- Tied to the Manager (MCC) account
- Sent as `developer-token` header on every API request
- Stored in config file — shared across the team
- Set via: `gads config set developer-token "TOKEN"`
- Env var override: `GADS_DEVELOPER_TOKEN`

### 2. OAuth2 (Per-User)

- Each team member authenticates their own Google account
- OAuth2 client ID + secret are shared (baked into the binary or set via config)
- Refresh token stored locally per user

#### OAuth2 Flow

```
gads auth login
```

1. CLI starts a local HTTP server on a random port (e.g. `localhost:9876`)
2. Opens the browser to Google's OAuth2 consent screen with:
   - `client_id` from config or embedded
   - `redirect_uri=http://localhost:9876/callback`
   - `scope=https://www.googleapis.com/auth/adwords`
   - `access_type=offline` (to get refresh token)
   - `prompt=consent` (force consent to ensure refresh token is returned)
3. User approves in browser
4. Google redirects to `localhost:9876/callback?code=XXX`
5. CLI exchanges code for access + refresh tokens
6. Refresh token stored in `~/.gads/credentials.json`
7. Access token cached in memory (or `~/.gads/token_cache.json` with expiry)

#### Token Refresh

- Before each API call, check if access token is expired
- If expired, use refresh token to get a new access token
- If refresh token is revoked, prompt user to re-run `gads auth login`

#### Credential Storage

```json
// ~/.gads/credentials.json
{
  "refresh_token": "1//0xxx...",
  "token_type": "Bearer",
  "created_at": "2026-03-18T12:00:00Z"
}
```

File permissions: `0600` (owner read/write only).

#### Auth for CI/Robots

For non-interactive environments, support direct token injection:

```bash
export GADS_ACCESS_TOKEN="ya29.xxx"
export GADS_DEVELOPER_TOKEN="AbCdEf123456"
gads campaigns list --customer-id 123-456-7890
```

Or a service account flow if Google Ads supports it for the account type.

#### Auth Commands

| Command | Description |
|---------|-------------|
| `gads auth login` | Interactive OAuth2 login via browser |
| `gads auth status` | Show current auth state (who's logged in, token expiry) |
| `gads auth logout` | Revoke tokens and delete local credentials |
| `gads auth refresh` | Force-refresh the access token |

---

## Configuration

### Config File

Location: `~/.gads/config.yaml`

```yaml
# Shared team settings
developer_token: "AbCdEf123456"
client_id: "123456789.apps.googleusercontent.com"
client_secret: "GOCSPX-xxx"

# User defaults
default_customer_id: "123-456-7890"
default_output: "table"

# Profiles (optional)
profiles:
  client-a:
    customer_id: "111-222-3333"
  client-b:
    customer_id: "444-555-6666"
```

### Config Commands

| Command | Description |
|---------|-------------|
| `gads config set <key> <value>` | Set a config value |
| `gads config get <key>` | Get a config value |
| `gads config list` | Show all config (redact secrets) |
| `gads config path` | Print config file path |

### Environment Variable Overrides

All config values can be overridden via env vars (prefixed with `GADS_`):

| Config Key | Env Var |
|-----------|---------|
| `developer_token` | `GADS_DEVELOPER_TOKEN` |
| `client_id` | `GADS_CLIENT_ID` |
| `client_secret` | `GADS_CLIENT_SECRET` |
| `default_customer_id` | `GADS_CUSTOMER_ID` |
| `default_output` | `GADS_OUTPUT` |

### Precedence

Flag > Env var > Config file > Default

### Global Flags (All Commands)

| Flag | Short | Description |
|------|-------|-------------|
| `--customer-id` | `-c` | Customer ID (overrides default) |
| `--output` | `-o` | Output format: `table`, `json`, `csv` |
| `--profile` | `-p` | Use a named profile from config |
| `--verbose` | `-v` | Show debug output (API requests/responses) |
| `--quiet` | `-q` | Suppress all non-data output |
| `--no-color` | | Disable colored output |

---

## Commands

### Campaigns

```bash
# List all campaigns
gads campaigns list
gads campaigns list --status ENABLED
gads campaigns list --status PAUSED --output json

# Get a specific campaign
gads campaigns get <campaign-id>
gads campaigns get <campaign-id> --output json

# Pause/enable a campaign
gads campaigns pause <campaign-id>
gads campaigns enable <campaign-id>

# Get campaign performance summary
gads campaigns stats <campaign-id> --date-range LAST_7_DAYS
gads campaigns stats <campaign-id> --from 2026-03-01 --to 2026-03-18
```

#### campaigns list — Table Output

```
ID           NAME                    STATUS    BUDGET/DAY   CLICKS   IMPR     COST      CTR
12345678901  Brand - Search          ENABLED   €50.00       1,234    45,678   €892.34   2.70%
12345678902  Generic - Display       PAUSED    €25.00       0        0        €0.00     0.00%
12345678903  Remarketing - Shopping  ENABLED   €100.00      3,456    89,012   €2,345.67 3.88%
```

### Ad Groups

```bash
gads ad-groups list --campaign <campaign-id>
gads ad-groups get <ad-group-id>
gads ad-groups pause <ad-group-id>
gads ad-groups enable <ad-group-id>
gads ad-groups stats <ad-group-id> --date-range LAST_30_DAYS
```

### Ads

```bash
gads ads list --campaign <campaign-id>
gads ads list --ad-group <ad-group-id>
gads ads get <ad-id>
gads ads pause <ad-id>
gads ads enable <ad-id>
```

### Keywords

```bash
gads keywords list --campaign <campaign-id>
gads keywords list --ad-group <ad-group-id>
gads keywords get <keyword-id>
gads keywords pause <keyword-id>
gads keywords enable <keyword-id>
gads keywords add --ad-group <ad-group-id> --text "running shoes" --match-type BROAD
```

### Reports / GAQL Queries

The power feature — run arbitrary Google Ads Query Language (GAQL) queries:

```bash
# Inline query
gads reports query "SELECT campaign.name, metrics.clicks, metrics.impressions FROM campaign WHERE segments.date DURING LAST_7_DAYS"

# Query from file
gads reports query -f ./queries/weekly-spend.gaql

# With output formatting
gads reports query -f ./queries/weekly-spend.gaql --output csv > report.csv
gads reports query -f ./queries/weekly-spend.gaql --output json | jq '.[].metrics.clicks'

# Saved queries (stored in config)
gads reports save weekly-spend -f ./queries/weekly-spend.gaql
gads reports run weekly-spend
gads reports run weekly-spend --output csv
gads reports saved  # List saved queries
```

#### GAQL Query Output — Table

```
CAMPAIGN              CLICKS   IMPRESSIONS   COST       CTR     CPC
Brand - Search        1,234    45,678        €892.34    2.70%   €0.72
Remarketing - Shop    3,456    89,012        €2,345.67  3.88%   €0.68
Generic - Display     567      123,456       €234.56    0.46%   €0.41
─────────────────────────────────────────────────────────────────────
TOTAL                 5,257    258,146       €3,472.57  2.04%   €0.66
```

### Account

```bash
gads account info                    # Current account details
gads account customers               # List accessible customer accounts
gads account switch <customer-id>    # Change default customer ID
```

### Budgets

```bash
gads budgets list
gads budgets get <budget-id>
gads budgets set <budget-id> --amount 75.00
```

---

## Output Formatting

### Table (Default)

- Aligned columns with headers
- Numbers formatted with commas/decimals appropriate to locale
- Currency symbols included
- Percentages calculated and formatted
- Color coding where useful (green = enabled, yellow = paused, red = removed) — respects `--no-color` and `NO_COLOR` env var
- Footer row with totals for numeric columns where it makes sense
- Truncation with `...` for long values, respects terminal width

### JSON

- Valid JSON array of objects
- Field names are snake_case
- Numbers are numbers (not strings)
- No trailing commas, no comments
- Pretty-printed by default, `--compact` for single-line
- Includes metadata envelope when `--verbose`:

```json
{
  "meta": {
    "customer_id": "123-456-7890",
    "query": "SELECT ...",
    "rows": 3,
    "timestamp": "2026-03-18T12:00:00Z"
  },
  "data": [...]
}
```

### CSV

- RFC 4180 compliant
- Header row included
- Proper escaping of commas, quotes, newlines
- UTF-8 with optional BOM for Excel compatibility (`--bom`)
- Raw numbers (no formatting, no currency symbols) — designed for spreadsheet import

---

## Error Handling

### User-Facing Errors

All errors go to stderr, formatted clearly:

```
Error: authentication failed — refresh token expired
  Run `gads auth login` to re-authenticate
```

```
Error: campaign 12345 not found
  Check the campaign ID with `gads campaigns list`
```

```
Error: Google Ads API rate limit exceeded
  Retry in 30 seconds (attempt 1/3)
  Retrying...
```

### API Error Mapping

Map Google Ads API error codes to human-readable messages:

| API Error | CLI Message |
|-----------|-------------|
| `AUTHENTICATION_ERROR` | "Authentication failed — run `gads auth login`" |
| `AUTHORIZATION_ERROR` | "Not authorized for customer ID {id}" |
| `QUOTA_ERROR` | "API quota exceeded — try again later" |
| `REQUEST_ERROR` | "Invalid request: {details}" |
| `QUERY_ERROR` | "GAQL syntax error: {details}" |

### Retry Logic

- Automatic retry with exponential backoff for transient errors (rate limits, 500s)
- Max 3 retries by default (`--retries` flag to override)
- Show retry progress on stderr: `Retrying in 2s (attempt 1/3)...`

### Verbose / Debug Mode

`--verbose` or `-v` prints to stderr:

```
[DEBUG] POST https://googleads.googleapis.com/v18/customers/1234567890/googleAds:searchStream
[DEBUG] Request headers: developer-token=***, Authorization=Bearer ***
[DEBUG] Request body: {"query": "SELECT ..."}
[DEBUG] Response: 200 OK (234ms)
```

---

## Distribution & Installation

### Install Script (Primary)

Hosted at `https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh`

```bash
curl -fsSL https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
```

The script:

1. Detects OS (`uname -s`) and architecture (`uname -m`)
2. Maps to binary name: `gads-{os}-{arch}` (e.g. `gads-darwin-arm64`, `gads-linux-amd64`)
3. Fetches the latest release tag from GitHub API
4. Downloads the binary from GitHub Releases
5. Verifies checksum (SHA256)
6. Installs to `~/.local/bin/gads` (or `/usr/local/bin/gads` if run with sudo)
7. Makes executable (`chmod +x`)
8. Prints version and next steps

#### Windows

Provide a PowerShell install script (`install.ps1`) or just direct download instructions:

```powershell
irm https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.ps1 | iex
```

### Private Repo Access

Since this is a private Datrics repo, the install script needs auth for GitHub Releases:

**Option A:** Make releases public (simplest — binaries don't contain secrets)
**Option B:** Use a GitHub PAT in the install command:

```bash
curl -fsSL -H "Authorization: token ghp_xxx" https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
```

**Option C:** Host binaries on a simple file server / S3 bucket with a shared URL

Recommendation: **Option A** — create a separate public repo just for releases, or use a public GitHub Pages site to host the binaries. The source repo stays private.

### Manual Install

For users who don't trust piping to shell:

1. Go to [Releases](https://github.com/datrics-ltd/gads-cli/releases)
2. Download the binary for your platform
3. Move to a directory in your PATH
4. `chmod +x gads`

### Updating

```bash
gads update          # Self-update to latest version
gads version         # Print current version
```

The `update` command runs the same logic as the install script — downloads the latest binary and replaces itself.

---

## CI/CD

### GitHub Actions Workflow

On push to `main` with a version tag (`v*`):

1. **Build** — Cross-compile for all targets:
   - `linux/amd64`
   - `darwin/amd64` (Intel Mac)
   - `darwin/arm64` (Apple Silicon)
   - `windows/amd64`
2. **Checksum** — Generate SHA256 checksums file
3. **Release** — Create GitHub Release with all binaries + checksums

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: darwin
            goarch: amd64
          - goos: darwin
            goarch: arm64
          - goos: windows
            goarch: amd64
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: |
          GOOS=${{ matrix.goos }} GOARCH=${{ matrix.goarch }} \
          go build -ldflags "-s -w -X main.version=${{ github.ref_name }}" \
          -o gads-${{ matrix.goos }}-${{ matrix.goarch }}${{ matrix.goos == 'windows' && '.exe' || '' }}
      - uses: softprops/action-gh-release@v2
        with:
          files: gads-*
```

### Version Embedding

Version baked in at build time via `-ldflags`:

```go
var version = "dev" // overridden by CI

func main() {
    rootCmd.Version = version
    rootCmd.Execute()
}
```

---

## Project Structure

```
gads-cli/
├── cmd/
│   ├── root.go              # Root command, global flags
│   ├── auth.go              # auth login|status|logout|refresh
│   ├── config.go            # config set|get|list|path
│   ├── campaigns.go         # campaigns list|get|pause|enable|stats
│   ├── ad_groups.go         # ad-groups list|get|pause|enable|stats
│   ├── ads.go               # ads list|get|pause|enable
│   ├── keywords.go          # keywords list|get|pause|enable|add
│   ├── reports.go           # reports query|save|run|saved
│   ├── budgets.go           # budgets list|get|set
│   ├── account.go           # account info|customers|switch
│   ├── update.go            # update (self-update)
│   └── version.go           # version
├── internal/
│   ├── api/
│   │   ├── client.go        # HTTP client, auth headers, retry logic
│   │   ├── gaql.go          # GAQL query builder/parser
│   │   └── errors.go        # API error mapping
│   ├── auth/
│   │   ├── oauth.go         # OAuth2 flow (browser + local server)
│   │   ├── token.go         # Token storage, refresh, caching
│   │   └── credentials.go   # Credential file management
│   ├── config/
│   │   ├── config.go        # Viper config management
│   │   └── profiles.go      # Multi-profile support
│   └── output/
│       ├── formatter.go     # Formatter interface
│       ├── table.go         # Table formatter
│       ├── json.go          # JSON formatter
│       └── csv.go           # CSV formatter
├── install.sh               # Unix install script
├── install.ps1              # Windows PowerShell install script
├── go.mod
├── go.sum
├── main.go                  # Entry point
├── SPEC.md                  # This file
├── README.md
├── LICENSE
└── .github/
    └── workflows/
        └── release.yml      # CI build + release
```

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Configuration management |
| `golang.org/x/oauth2` | OAuth2 client |
| `google.golang.org/api` | Google APIs client (or raw HTTP) |
| `github.com/olekukonko/tablewriter` | Table formatting |
| `github.com/fatih/color` | Terminal colors |
| `github.com/pkg/browser` | Open browser cross-platform |

Keep dependencies minimal. The Google Ads API can be called via raw HTTP + protobuf/JSON — evaluate whether the official client library adds value or just complexity.

---

## Implementation Phases

### Phase 1 — Foundation

- [ ] Project scaffolding (Go module, Cobra setup, directory structure)
- [ ] Config management (`config set/get/list/path`)
- [ ] OAuth2 auth flow (`auth login/status/logout`)
- [ ] API client with auth headers (developer token + OAuth2)
- [ ] Output formatters (table, JSON, CSV)
- [ ] `campaigns list` as the first real command
- [ ] Error handling framework
- [ ] `version` command

### Phase 2 — Core Commands

- [ ] `campaigns get/pause/enable/stats`
- [ ] `ad-groups list/get/pause/enable/stats`
- [ ] `ads list/get/pause/enable`
- [ ] `keywords list/get/pause/enable/add`
- [ ] `reports query` (inline GAQL + file input)
- [ ] `account info/customers/switch`
- [ ] `budgets list/get/set`

### Phase 3 — Distribution

- [ ] Install script (`install.sh`, `install.ps1`)
- [ ] GitHub Actions release workflow
- [ ] SHA256 checksums
- [ ] `update` command (self-update)
- [ ] Version embedding at build time

### Phase 4 — Polish

- [ ] Saved queries (`reports save/run/saved`)
- [ ] Multi-profile support (`--profile`)
- [ ] Shell completions (bash, zsh, fish, PowerShell)
- [ ] `--verbose` debug output
- [ ] Retry logic with backoff
- [ ] Terminal width detection for table truncation
- [ ] Color coding for statuses
- [ ] Man page / help text polish

---

## Open Questions

1. **Google Ads API version** — Which version are we targeting? (v18 is latest as of early 2026)
2. **Currency** — Should we auto-detect from the account or default to EUR?
3. **Rate limiting** — What's our quota? Affects retry strategy.
4. **Service accounts** — Do we need service account auth for automated/CI usage, or is OAuth2 + refresh token sufficient?
5. **Scoping** — Do we need Manager (MCC) account features, or just single-account operations?
6. **Protobuf vs REST** — Google Ads API supports both gRPC (protobuf) and REST (JSON). REST is simpler to implement; gRPC is more efficient. Recommendation: start with REST.
