---
name: gads
description: Query, manage, and mutate Google Ads campaigns, ad groups, ads, keywords, and budgets via the gads CLI. Use when asked about Google Ads performance, campaign management, ad spend, keyword analysis, or any Google Ads API operations. Triggers on "Google Ads", "campaigns", "ad spend", "keywords", "GAQL", "search ads", "display ads", "ad groups", "ad budget", "impressions", "clicks", "CTR", "CPC", "conversions".
---

# gads — Google Ads CLI

CLI tool for the Google Ads REST API (v23). Single binary, no runtime dependencies.

## Three Tiers

Choose the right tier for the task:

### Tier 1 — Named Commands (common operations)
```bash
gads campaigns list --status ENABLED
gads campaigns pause 12345
gads keywords add --ad-group 123 --text "running shoes" --match-type BROAD
gads budgets set 456 --amount 75.00
```

Use for: listing, pausing, enabling, basic stats. Available for campaigns, ad-groups, ads, keywords, budgets, account.

### Tier 2 — GAQL Queries (all reads)
```bash
gads query "SELECT campaign.name, metrics.clicks, metrics.cost_micros FROM campaign WHERE segments.date DURING LAST_7_DAYS" --output json
gads query -f query.gaql --output csv > report.csv
```

Use for: any data retrieval. GAQL covers 100% of readable resources. Prefer `--output json` for programmatic parsing.

### Tier 3 — Raw API (escape hatch, 100% coverage)
```bash
gads api POST /v23/customers/{customer_id}/campaigns:mutate -d '{"operations": [...]}'
gads api GET /v23/customers/{customer_id}/campaigns/789
```

Use for: mutations not covered by named commands. `{customer_id}` auto-replaces from config. Handles all auth headers automatically.

**Decision flow:** Named command exists? → Use it. Read-only? → Use GAQL. Mutation without named command? → Use `gads api`.

## Auth Model

Every request sends three headers automatically:
- `developer-token` — static, from config (proves API access)
- `Authorization: Bearer` — OAuth2 access token, auto-refreshed (proves user identity)
- `login-customer-id` — only set when the authenticated Google account accesses the ad account *through* a Manager (MCC) account. The value is the MCC's customer ID. Leave unset for direct account access — setting it when not needed causes authorization errors.

Config lives at `~/.gads/config.yaml`. Credentials at `~/.gads/credentials.json`.

## Output Formats

All commands support `--output table|json|csv`:
- `table` — human-readable (default)
- `json` — use for parsing results programmatically. Add `--compact` for single-line.
- `csv` — for spreadsheet export. Add `--bom` for Excel.

**Always use `--output json` when processing results programmatically.** Parse with `jq` when piping.

## Key Patterns

### Campaign performance
```bash
gads query "SELECT campaign.name, campaign.status, metrics.clicks, metrics.impressions, metrics.cost_micros, metrics.conversions FROM campaign WHERE segments.date DURING LAST_7_DAYS AND campaign.status != 'REMOVED'" --output json
```

### Keyword analysis
```bash
gads query "SELECT ad_group_criterion.keyword.text, ad_group_criterion.keyword.match_type, metrics.clicks, metrics.impressions, metrics.cost_micros, metrics.average_cpc FROM keyword_view WHERE segments.date DURING LAST_30_DAYS ORDER BY metrics.cost_micros DESC" --output json
```

### Pausing/enabling resources
```bash
gads campaigns pause <campaign-id>
gads campaigns enable <campaign-id>
gads ad-groups pause <ad-group-id>
gads keywords pause <keyword-id>
```

### Budget changes
```bash
gads budgets list --output json
gads budgets set <budget-id> --amount 50.00
```

### Discover schema for GAQL queries
```bash
gads schema campaign                    # all fields
gads schema campaign --selectable       # fields usable in SELECT
gads schema keyword_view --filterable   # fields usable in WHERE
```

## Money Values

Google Ads API returns money in **micros** (1 currency unit = 1,000,000 micros). To convert:
- `cost_micros: 1500000` = £1.50
- `average_cpc: 720000` = £0.72
- `amount_micros: 50000000` = £50.00 daily budget

Divide by 1,000,000 for display. When setting budgets via mutations, multiply by 1,000,000.

## GAQL Reference

For GAQL syntax, available resources, fields, date ranges, and common query patterns, read [references/gaql-guide.md](references/gaql-guide.md).

## Mutations Reference

For constructing mutations via `gads api` (create campaigns, add keywords, update budgets, etc.), read [references/mutations.md](references/mutations.md).

## Named Commands Reference

For all Tier 1 commands with flags and examples, read [references/commands.md](references/commands.md).

## Troubleshooting

- **Auth errors?** Check if `login_customer_id` is set when it shouldn't be. If you have direct account access (not via MCC), this field must be empty. Remove it with: `gads config set login_customer_id ''`
- **Unexpected 403/authorization errors?** The most common cause is a `login_customer_id` set for an account you access directly — remove it.

## Error Handling

- Exit 0 = success, 1 = general error, 2 = auth error, 3 = API error
- Auth errors → suggest `gads auth login`
- Rate limits → automatic retry with backoff (3 attempts)
- GAQL errors → check field names with `gads schema <resource>`
- Use `--verbose` to see full request/response for debugging
