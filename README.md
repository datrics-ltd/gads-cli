# gads-cli

A command-line interface for the Google Ads API. Built for the Datrics team.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/datrics-ltd/gads-cli/main/install.sh | sh
```

## Quick Start

```bash
# Configure your developer token (shared across the team)
gads config set developer-token "YOUR_DEVELOPER_TOKEN"

# Authenticate your Google account
gads auth login

# Set your default customer ID
gads config set customer-id 123-456-7890

# Start using it
gads campaigns list
gads campaigns list --output json
gads reports query "SELECT campaign.name, metrics.clicks FROM campaign WHERE segments.date DURING LAST_7_DAYS"
```

## Output Formats

Every command supports `--output` (`-o`) with three modes:

- `table` — Human-readable aligned columns (default)
- `json` — Machine-readable JSON
- `csv` — Spreadsheet-friendly CSV

## Documentation

See [SPEC.md](./SPEC.md) for the full technical specification and implementation plan.

## License

Private — Datrics Ltd.
