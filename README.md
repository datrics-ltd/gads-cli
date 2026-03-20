# gads

A command-line interface for the Google Ads API. Single binary, zero dependencies, cross-platform.

Query any data with GAQL. Manage campaigns, ads, keywords, and budgets. Hit any API endpoint directly when you need full control. Designed for both humans and AI agents.

## Install

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.ps1 | iex
```

### Manual

Download the binary for your platform from [Releases](https://github.com/datrics-ltd/gads-cli/releases), move it somewhere in your PATH, and `chmod +x gads`.

## Prerequisites

Before running `gads init`, you need a few things set up in Google Ads and Google Cloud.

### 1. Google Ads Developer Token

You need a Google Ads account with API access. In your Google Ads account, go to **Tools & Settings → API Center** and apply for a developer token.

New tokens start as **test tokens** — they only work against test accounts. Apply for **Standard access** when you're ready to use it with real accounts. This review can take a few days.

→ [Get a developer token](https://developers.google.com/google-ads/api/docs/get-started/dev-token)

### 2. Google Cloud Project

Create a project (or use an existing one) at [console.cloud.google.com](https://console.cloud.google.com). This is where you'll enable the API and create OAuth credentials.

### 3. Enable the Google Ads API

In your Cloud project, go to **APIs & Services → Library**, search for **Google Ads API**, and enable it.

→ [Enable Google Ads API](https://console.cloud.google.com/apis/library/googleads.googleapis.com)

### 4. OAuth Consent Screen

Go to **APIs & Services → OAuth consent screen** and configure it:

- **Internal** — if your org uses Google Workspace and you only need access for internal users (no review required)
- **External** — for personal Google accounts or external users; you'll need to add yourself as a test user while in testing mode

Required scope: `https://www.googleapis.com/auth/adwords`

→ [Configure OAuth consent screen](https://developers.google.com/google-ads/api/docs/oauth/cloud-project#configure_the_oauth_consent_screen)

### 5. Create OAuth Desktop Credentials

Go to **APIs & Services → Credentials → Create Credentials → OAuth client ID**. Set application type to **Desktop app**. Copy the **client ID** and **client secret** — you'll need these in `gads init`.

→ [Create credentials](https://console.cloud.google.com/apis/credentials)

---

## Setup

Run the interactive setup wizard:

```bash
gads init
```

This walks you through:

1. **Developer token** — from Google Ads → Tools & Settings → API Center
2. **OAuth2 client ID + secret** — from [Google Cloud Console](https://console.cloud.google.com/apis/credentials) (create a Desktop app credential)
3. **Customer ID** — the 10-digit number from your Google Ads dashboard
4. **Login Customer ID** — leave blank for most users. Only set this if your Google account does not have direct access to the ad account and must go through a Manager (MCC) account. The value is the MCC's customer ID (not the ad account's). Setting this incorrectly causes authorization errors that are hard to diagnose.
5. **OAuth2 login** — opens your browser to authenticate with Google
6. **Verification** — runs a test query to confirm everything works

All config is stored in `~/.gads/config.yaml`. Credentials in `~/.gads/credentials.json`.

### Manual Setup

If you prefer to configure manually:

```bash
gads config set developer_token "YOUR_DEVELOPER_TOKEN"
gads config set client_id "YOUR_CLIENT_ID.apps.googleusercontent.com"
gads config set client_secret "YOUR_CLIENT_SECRET"
gads config set customer_id "YOUR_CUSTOMER_ID"
# login_customer_id: omit unless your account is accessed through a Manager (MCC) account
# gads config set login_customer_id "YOUR_MCC_CUSTOMER_ID"

gads auth login
```

### Team Onboarding

Everyone on the team shares the same developer token, client ID, and client secret. Each person runs `gads auth login` to authenticate with their own Google account. Their refresh token is stored locally — nothing is shared.

```bash
# New team member
gads init              # paste the shared values when prompted
gads campaigns list    # verify it works
```

The team member's Google account needs access to the ad account — either direct access (invite via Google Ads → Admin → Access and security) or through the MCC.

## Usage

### Three Ways to Use It

**Named commands** — for common operations:

```bash
gads campaigns list
gads campaigns list --status ENABLED
gads campaigns pause 12345
gads keywords add --ad-group 456 --text "running shoes" --match-type BROAD
gads budgets set 789 --amount 50.00
```

**GAQL queries** — for any data retrieval (covers 100% of readable resources):

```bash
gads query "SELECT campaign.name, metrics.clicks, metrics.cost_micros
  FROM campaign
  WHERE segments.date DURING LAST_7_DAYS"
```

**Raw API calls** — escape hatch for anything (covers 100% of the API):

```bash
gads api POST /v23/customers/{customer_id}/campaigns:mutate \
  -d '{"operations": [{"create": {...}}]}'
```

### Output Formats

Every command supports `--output table|json|csv`:

```bash
gads campaigns list                          # table (default, human-readable)
gads campaigns list --output json            # JSON (for scripts and AI agents)
gads campaigns list --output csv > report.csv  # CSV (for spreadsheets)
```

### Examples

```bash
# Campaign performance this week
gads query "SELECT campaign.name, campaign.status,
  metrics.clicks, metrics.impressions, metrics.cost_micros
  FROM campaign
  WHERE segments.date DURING LAST_7_DAYS
  AND campaign.status != 'REMOVED'
  ORDER BY metrics.cost_micros DESC"

# Top keywords by spend
gads query "SELECT ad_group_criterion.keyword.text,
  metrics.clicks, metrics.cost_micros, metrics.ctr
  FROM keyword_view
  WHERE segments.date DURING LAST_30_DAYS
  ORDER BY metrics.cost_micros DESC
  LIMIT 20"

# Daily spend breakdown as CSV
gads query "SELECT segments.date, campaign.name, metrics.cost_micros
  FROM campaign
  WHERE segments.date DURING LAST_30_DAYS" --output csv > daily-spend.csv

# Search terms that triggered your ads
gads query "SELECT search_term_view.search_term,
  metrics.clicks, metrics.impressions
  FROM search_term_view
  WHERE segments.date DURING LAST_30_DAYS
  ORDER BY metrics.impressions DESC
  LIMIT 50"

# Pause a campaign
gads campaigns pause 12345

# Adjust budget
gads budgets set 67890 --amount 75.00

# Explore available fields for a resource
gads schema campaign --selectable

# Save a query you run often
gads query save weekly-report "SELECT campaign.name, metrics.clicks,
  metrics.cost_micros FROM campaign
  WHERE segments.date DURING LAST_7_DAYS"
gads query run weekly-report
```

## Commands

| Command | Description |
|---|---|
| `gads campaigns` | List, get, pause, enable, stats |
| `gads ad-groups` | List, get, pause, enable, stats |
| `gads ads` | List, get, pause, enable |
| `gads keywords` | List, get, pause, enable, add |
| `gads budgets` | List, get, set amount |
| `gads account` | Info, list customers, switch default |
| `gads query` | Run GAQL queries, save/run saved queries |
| `gads api` | Raw authenticated GET/POST to any endpoint |
| `gads schema` | Explore resource fields and types |
| `gads auth` | Login, logout, status, refresh |
| `gads config` | Set, get, list, path |
| `gads init` | Interactive setup wizard |
| `gads update` | Self-update to latest version |
| `gads completion` | Shell completions (bash, zsh, fish, PowerShell) |

Run `gads <command> --help` for detailed usage and examples.

## Shell Completions

```bash
# bash
gads completion bash >> ~/.bashrc

# zsh
gads completion zsh >> ~/.zshrc

# fish
gads completion fish > ~/.config/fish/completions/gads.fish
```

## Money Values

The Google Ads API returns money in **micros** (1 currency unit = 1,000,000 micros):

| Micros | Value |
|---|---|
| `1500000` | £1.50 |
| `50000000` | £50.00 |
| `720000` | £0.72 |

Divide by 1,000,000 for display. Multiply by 1,000,000 when setting values.

## AI Agent Skill

The `skill/` directory contains an [AgentSkill](https://agentskills.io) for use with AI coding agents (Claude Code, OpenClaw, etc.):

```
skill/
├── SKILL.md                    # Entry point — how to use gads, decision flow, key patterns
└── references/
    ├── gaql-guide.md           # GAQL syntax, operators, date ranges, 10+ query templates
    ├── mutations.md            # JSON payloads for common mutations via gads api
    └── commands.md             # Every named command with flags and examples
```

Install the skill by copying or symlinking `skill/` into your agent's skills directory. The agent reads `SKILL.md` first, then loads only the reference it needs for the current task.

## Configuration

Config file: `~/.gads/config.yaml`

```yaml
developer_token: "your-developer-token"
client_id: "your-client-id.apps.googleusercontent.com"
client_secret: "your-client-secret"
customer_id: "1234567890"
login_customer_id: ""  # omit unless accessing via a Manager (MCC) account — setting this incorrectly causes auth errors
```

### Environment Variables

All config values can be overridden with `GADS_` prefixed env vars:

```bash
export GADS_DEVELOPER_TOKEN="..."
export GADS_CLIENT_ID="..."
export GADS_CLIENT_SECRET="..."
export GADS_CUSTOMER_ID="..."
export GADS_ACCESS_TOKEN="..."    # skip OAuth2 entirely (CI/automation)
```

### Profiles

Manage multiple accounts with named profiles:

```yaml
# ~/.gads/config.yaml
profiles:
  client-a:
    customer_id: "1111111111"
  client-b:
    customer_id: "2222222222"
```

```bash
gads campaigns list --profile client-a
```

### Precedence

Flag → Environment variable → Config file → Default

## Architecture

```
~/.gads/
├── config.yaml           # Shared settings (developer token, client ID, customer ID)
├── credentials.json      # Per-user OAuth2 refresh token (never share)
└── saved-queries/        # Saved GAQL queries
```

The binary contains zero secrets. All auth is configured per-user after install.

### Auth Flow

1. `developer-token` header — proves API access (static, from config)
2. `Authorization: Bearer` header — OAuth2 access token, auto-refreshed from stored refresh token
3. `login-customer-id` header — optional, only when routing through an MCC

### API Version

Currently targets Google Ads API **v23**.

## Development

```bash
# Build
go build -o gads .

# Run tests
go test ./...

# Build with version
go build -ldflags "-s -w -X main.version=v0.1.0" -o gads .
```

## Technical Specification

See [SPEC.md](./SPEC.md) for the full technical specification including architecture decisions, three-tier command model, auth flow details, and code generation strategy.

## License

MIT License
