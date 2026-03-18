# Named Commands Reference

All Tier 1 commands. These are convenience wrappers — they construct GAQL queries (reads) or mutate API calls (writes) under the hood.

## Global Flags

Apply to all commands:

| Flag | Short | Description |
|---|---|---|
| `--customer-id` | `-c` | Override customer ID |
| `--output` | `-o` | `table` (default), `json`, `csv` |
| `--profile` | `-p` | Use named profile from config |
| `--verbose` | `-v` | Debug output (full request/response) |
| `--quiet` | `-q` | Suppress non-data output |
| `--no-color` | | Disable colors |
| `--compact` | | JSON: single-line output |
| `--bom` | | CSV: prepend UTF-8 BOM for Excel |
| `--retries` | | Max retry attempts (default 3) |

## campaigns

```bash
gads campaigns list                                    # All campaigns
gads campaigns list --status ENABLED                   # Filter by status
gads campaigns list --status PAUSED --output json      # JSON output
gads campaigns get <id>                                # Single campaign detail
gads campaigns pause <id>                              # Set status to PAUSED
gads campaigns enable <id>                             # Set status to ENABLED
gads campaigns stats <id> --date-range LAST_7_DAYS     # Performance metrics
gads campaigns stats <id> --from 2026-03-01 --to 2026-03-18  # Custom range
```

## ad-groups

```bash
gads ad-groups list --campaign <campaign-id>           # List ad groups in campaign
gads ad-groups get <id>                                # Detail view
gads ad-groups pause <id>
gads ad-groups enable <id>
gads ad-groups stats <id> --date-range LAST_30_DAYS
```

## ads

```bash
gads ads list --campaign <campaign-id>                 # List ads in campaign
gads ads list --ad-group <ad-group-id>                 # List ads in ad group
gads ads get <id>
gads ads pause <id>
gads ads enable <id>
```

## keywords

```bash
gads keywords list --campaign <campaign-id>
gads keywords list --ad-group <ad-group-id>
gads keywords get <id>
gads keywords pause <id>
gads keywords enable <id>
gads keywords add --ad-group <id> --text "keyword" --match-type BROAD
```

Match types: `BROAD`, `PHRASE`, `EXACT`.

## budgets

```bash
gads budgets list
gads budgets get <id>
gads budgets set <id> --amount 75.00                   # Set daily budget in £
```

## account

```bash
gads account info                                      # Current account details
gads account customers                                 # List accessible accounts
gads account switch <customer-id>                      # Change default customer ID
```

## query

```bash
gads query "<GAQL>"                                    # Inline query
gads query -f <file.gaql>                              # From file
gads query save <name> -f <file.gaql>                  # Save a query
gads query save <name> "<GAQL>"                        # Save inline
gads query run <name>                                  # Run saved query
gads query saved                                       # List saved queries
```

## api

```bash
gads api GET <path>                                    # Authenticated GET
gads api POST <path> -d '<json>'                       # POST with inline body
gads api POST <path> -d @<file.json>                   # POST with body from file
gads api POST <path>                                   # POST with body from stdin
gads api POST <path> -d '...' --dry-run                # Preview without sending
gads api GET <path> -H "key: value"                    # Custom headers
```

`{customer_id}` in path auto-replaces from config.

## schema

```bash
gads schema <resource>                                 # All fields
gads schema <resource> --selectable                    # GAQL SELECT-able fields
gads schema <resource> --filterable                    # GAQL WHERE-able fields
gads schema --live <resource>                          # Fetch from API (not embedded)
```

## auth

```bash
gads auth login                                        # OAuth2 browser flow
gads auth status                                       # Current auth state
gads auth logout                                       # Revoke and delete tokens
gads auth refresh                                      # Force token refresh
```

## config

```bash
gads config set <key> <value>
gads config get <key>
gads config list                                       # Shows all, redacts secrets
gads config path                                       # Print config file location
```

## init

```bash
gads init                                              # Interactive setup wizard
```

## Other

```bash
gads version
gads update                                            # Self-update to latest
gads completion bash|zsh|fish|powershell               # Shell completions
```
