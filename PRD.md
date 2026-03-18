# gads-cli — Product Requirements Document

A cross-platform CLI for the Google Ads API. Built in Go, distributed as single binaries. Three-tier command model: named commands, GAQL queries, raw API escape hatch.

See `SPEC.md` for full technical specification, architecture decisions, and rationale.

---

## Phase 1 — Foundation (MVP)

After this phase, the CLI can do everything the Google Ads API supports via `gads query` and `gads api`. Everything after is convenience.

### 1.1 Project Scaffolding

- [x] Initialize Go module (`github.com/datrics-ltd/gads-cli`)
- [x] Set up Cobra root command with global flags (`--customer-id`, `--output`, `--profile`, `--verbose`, `--quiet`, `--no-color`)
- [x] Set up Viper config integration (config file + env vars + flag binding)
- [x] Create directory structure: `cmd/`, `internal/api/`, `internal/auth/`, `internal/config/`, `internal/output/`
- [x] Add `version` command with build-time version injection via `-ldflags`
- [x] Add `.gitignore` for Go project

### 1.2 Configuration

- [x] Config file at `~/.gads/config.yaml` — auto-create directory on first use
- [x] `gads config set <key> <value>` — write to config file
- [x] `gads config get <key>` — read from config file
- [x] `gads config list` — show all config, redact sensitive values (tokens, secrets)
- [x] `gads config path` — print config file location
- [x] Environment variable overrides with `GADS_` prefix (see SPEC.md for mapping)
- [x] Precedence: flag > env var > config file > default

### 1.3 Authentication — OAuth2

- [x] `gads auth login` — start local HTTP server, open browser to Google OAuth2 consent screen, handle callback, exchange code for tokens, store refresh token
- [x] Credential storage at `~/.gads/credentials.json` with `0600` permissions
- [x] Token refresh — auto-refresh expired access tokens before API calls using stored refresh token
- [x] `gads auth status` — show current auth state (logged in as who, token expiry, developer token configured?)
- [x] `gads auth logout` — revoke tokens and delete local credentials
- [x] `gads auth refresh` — force-refresh the access token
- [x] Support `GADS_ACCESS_TOKEN` env var for CI/non-interactive use (skip OAuth2 flow entirely)

### 1.4 API Client

- [x] HTTP client that injects both auth headers on every request: `developer-token` header + `Authorization: Bearer` header
- [x] Auto-refresh access token if expired before making request
- [x] Retry logic with exponential backoff for transient errors (429 rate limit, 500/503)
- [x] Max 3 retries by default, configurable via `--retries` flag
- [x] API error mapping — parse Google Ads API error responses into human-readable messages (see SPEC.md error table)
- [x] `--verbose` mode — log full request/response to stderr (URL, headers with redacted tokens, body, status, timing)

### 1.5 Output Formatters

- [x] Formatter interface — all commands produce structured data, formatters render it
- [x] **Table formatter** — aligned columns, header row, number formatting (commas, decimals), currency symbols, percentage formatting, terminal width detection, truncation with `...`, color coding (green=enabled, yellow=paused, red=removed), respects `--no-color` and `NO_COLOR` env var, footer row with totals where appropriate
- [x] **JSON formatter** — valid JSON array of objects, snake_case field names, numbers as numbers, pretty-printed by default, `--compact` flag for single-line, metadata envelope in `--verbose` mode
- [x] **CSV formatter** — RFC 4180 compliant, header row, proper escaping, UTF-8, `--bom` flag for Excel compatibility, raw numbers (no formatting/currency symbols)

### 1.6 GAQL Queries (Tier 2)

- [x] `gads query "<GAQL string>"` — execute inline GAQL query against configured customer ID
- [x] `gads query -f <file.gaql>` — read query from file
- [x] POST to `googleads.googleapis.com/v18/customers/{customerId}/googleAds:searchStream`
- [x] Parse streaming response into rows
- [x] Route through output formatters (`--output table|json|csv`)
- [x] Support `--customer-id` flag to override default
- [x] Useful error messages for GAQL syntax errors (pass through Google's error details)

### 1.7 Raw API Escape Hatch (Tier 3)

- [x] `gads api GET <path>` — make authenticated GET request to Google Ads API
- [x] `gads api POST <path> -d '<json>'` — make authenticated POST with inline body
- [x] `gads api POST <path> -d @<file.json>` — make authenticated POST with body from file
- [x] `gads api POST <path>` — read body from stdin when no `-d` flag
- [x] Auto-replace `{customer_id}` in path with configured default
- [x] Auto-prepend `https://googleads.googleapis.com` if path starts with `/`
- [x] Pretty-print JSON response by default, `--raw` for unformatted
- [x] `--dry-run` flag — show full request (URL, headers, body) without sending
- [x] Support custom headers via `-H "key: value"`
- [x] Route response through output formatters when `--output` is specified

### 1.8 Integration Testing

- [x] Test auth flow with mock OAuth2 server
- [x] Test config read/write/precedence
- [x] Test output formatters with sample data (table alignment, JSON validity, CSV escaping)
- [x] Test `gads api` request construction (header injection, path substitution, dry-run output)
- [x] Test error handling and retry logic with mock API responses

---

## Phase 2 — Named Commands (Convenience)

These are Tier 1 ergonomic wrappers. Each is a thin layer over GAQL (for reads) or the mutate API (for writes). They add nice flags, validation, and formatted output.

### 2.1 Campaigns

- [x] `gads campaigns list` — list all campaigns with ID, name, status, budget, basic metrics
- [x] `gads campaigns list --status <ENABLED|PAUSED|REMOVED>` — filter by status
- [x] `gads campaigns get <campaign-id>` — detailed view of a single campaign
- [x] `gads campaigns pause <campaign-id>` — set campaign status to PAUSED
- [x] `gads campaigns enable <campaign-id>` — set campaign status to ENABLED
- [x] `gads campaigns stats <campaign-id> --date-range <range>` — performance metrics with date range support
- [x] `gads campaigns stats <campaign-id> --from <date> --to <date>` — custom date range

### 2.2 Ad Groups

- [x] `gads ad-groups list --campaign <campaign-id>` — list ad groups in a campaign
- [x] `gads ad-groups get <ad-group-id>` — detailed view
- [x] `gads ad-groups pause <ad-group-id>` — pause
- [x] `gads ad-groups enable <ad-group-id>` — enable
- [x] `gads ad-groups stats <ad-group-id> --date-range <range>` — performance metrics

### 2.3 Ads

- [x] `gads ads list --campaign <campaign-id>` — list ads in a campaign
- [x] `gads ads list --ad-group <ad-group-id>` — list ads in an ad group
- [x] `gads ads get <ad-id>` — detailed view
- [x] `gads ads pause <ad-id>` — pause
- [x] `gads ads enable <ad-id>` — enable

### 2.4 Keywords

- [x] `gads keywords list --campaign <campaign-id>` — list keywords
- [x] `gads keywords list --ad-group <ad-group-id>` — list keywords in ad group
- [x] `gads keywords get <keyword-id>` — detailed view
- [x] `gads keywords pause <keyword-id>` — pause
- [x] `gads keywords enable <keyword-id>` — enable
- [x] `gads keywords add --ad-group <id> --text "<keyword>" --match-type <BROAD|PHRASE|EXACT>` — add a keyword

### 2.5 Budgets

- [x] `gads budgets list` — list all budgets
- [x] `gads budgets get <budget-id>` — detailed view
- [x] `gads budgets set <budget-id> --amount <amount>` — update daily budget amount

### 2.6 Account

- [x] `gads account info` — current account details
- [x] `gads account customers` — list accessible customer accounts (useful for MCC)
- [x] `gads account switch <customer-id>` — update default customer ID in config

---

## Phase 3 — Distribution

### 3.1 Install Scripts

- [x] `install.sh` — detect OS/arch, download correct binary from GitHub Releases, verify SHA256 checksum, install to `~/.local/bin/gads` (or `/usr/local/bin` with sudo), print version + next steps
- [x] `install.ps1` — PowerShell equivalent for Windows
- [x] Handle private repo auth (GitHub PAT in header or public release repo)

### 3.2 CI/CD

- [x] `.github/workflows/release.yml` — trigger on `v*` tags
- [x] Cross-compile for: `linux/amd64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- [x] Generate SHA256 checksums file
- [x] Create GitHub Release with all binaries + checksums
- [x] Version embedding via `-ldflags "-X main.version=$TAG"`

### 3.3 Self-Update

- [x] `gads update` — check latest release, download new binary, replace self
- [x] Show current vs latest version before updating
- [x] Verify checksum of downloaded binary

---

## Phase 4 — Schema & Intelligence

### 4.1 Schema Embedding

- [x] `gen/proto_fetch.sh` — script to pull proto definitions from `googleapis/googleapis`
- [x] `gen/codegen.go` — parse protos, extract resource/field metadata
- [x] Embed metadata into binary at build time (Go `embed` package)

### 4.2 Schema Command

- [x] `gads schema <resource>` — show all fields for a resource with types
- [x] `gads schema <resource> --selectable` — only fields usable in GAQL SELECT
- [x] `gads schema <resource> --filterable` — only fields usable in GAQL WHERE
- [x] `gads schema --live <resource>` — fetch from `GoogleAdsFieldService` instead of embedded data

### 4.3 Validation & Completion

- [x] GAQL query validation — check field names and resource compatibility before sending to API
- [x] Shell completions for bash, zsh, fish, PowerShell (Cobra built-in + custom field completions)

---

## Phase 5 — Polish

### 5.1 Saved Queries

- [x] `gads query save <name> -f <file.gaql>` — save a query to config dir
- [x] `gads query save <name> "<GAQL string>"` — save inline query
- [x] `gads query run <name>` — execute a saved query
- [x] `gads query saved` — list all saved queries

### 5.2 Multi-Profile

- [ ] `--profile <name>` flag on all commands — use named profile from config
- [ ] Profile inherits base config, overrides specific values (customer_id, etc.)

### 5.3 UX Polish

- [ ] Terminal width detection for table column sizing
- [ ] Color coding for campaign/ad/keyword statuses
- [ ] `--verbose` debug output on all commands
- [ ] Man page generation (Cobra built-in)
- [ ] Help text polish — examples in every command's `--help`

---

## Constraints

- **Language:** Go (latest stable)
- **No runtime dependencies** — single static binary per platform
- **Config location:** `~/.gads/` (config.yaml, credentials.json, token_cache.json, saved queries)
- **Auth:** Developer token (static, shared) + OAuth2 (per-user refresh token)
- **API protocol:** REST/JSON (not gRPC) for simplicity
- **API version:** v18 (confirm before starting)
- **All output:** stdout for data, stderr for logs/errors/progress
- **Exit codes:** 0 = success, 1 = general error, 2 = auth error, 3 = API error
