# GAQL Reference Guide

Google Ads Query Language — used with `gads query "..."`.

## Syntax

```sql
SELECT <fields>
FROM <resource>
WHERE <conditions>
ORDER BY <field> [ASC|DESC]
LIMIT <n>
```

Only `SELECT` and `FROM` are required. All others are optional.

## Resources

Common resources and their primary use:

| Resource | Use |
|---|---|
| `campaign` | Campaign-level data and metrics |
| `ad_group` | Ad group-level data |
| `ad_group_ad` | Individual ads |
| `ad_group_criterion` | Keywords (and other targeting criteria) |
| `keyword_view` | Keyword performance metrics |
| `campaign_budget` | Budget information |
| `customer` | Account-level info |
| `customer_client` | Sub-accounts (MCC only) |
| `search_term_view` | Actual search terms that triggered ads |
| `geographic_view` | Performance by location |
| `gender_view` | Performance by gender |
| `age_range_view` | Performance by age |
| `landing_page_view` | Landing page performance |
| `change_event` | Account change history |

Discover all fields for a resource: `gads schema <resource> --selectable`

## Field Types

Fields come in three categories:

- **Resource fields** — attributes of the resource (e.g. `campaign.name`, `campaign.status`)
- **Metrics** — performance data (e.g. `metrics.clicks`, `metrics.impressions`, `metrics.cost_micros`)
- **Segments** — dimensions that break down metrics (e.g. `segments.date`, `segments.device`)

**Important:** Adding segments to SELECT changes the granularity of results. `segments.date` gives daily rows. `segments.device` gives per-device rows.

## Date Ranges

Use `WHERE segments.date DURING <range>` or explicit dates:

### Predefined Ranges
```sql
WHERE segments.date DURING LAST_7_DAYS
WHERE segments.date DURING LAST_30_DAYS
WHERE segments.date DURING LAST_90_DAYS
WHERE segments.date DURING THIS_MONTH
WHERE segments.date DURING LAST_MONTH
WHERE segments.date DURING THIS_QUARTER
WHERE segments.date DURING LAST_QUARTER
WHERE segments.date DURING THIS_YEAR
WHERE segments.date DURING LAST_YEAR
WHERE segments.date DURING YESTERDAY
WHERE segments.date DURING TODAY
```

### Custom Date Range
```sql
WHERE segments.date BETWEEN '2026-03-01' AND '2026-03-18'
WHERE segments.date >= '2026-03-01' AND segments.date <= '2026-03-18'
```

## Common Metrics

| Metric | Description | Unit |
|---|---|---|
| `metrics.clicks` | Total clicks | integer |
| `metrics.impressions` | Total impressions | integer |
| `metrics.cost_micros` | Total cost | micros (÷1,000,000 for currency) |
| `metrics.average_cpc` | Average cost per click | micros |
| `metrics.ctr` | Click-through rate | decimal (0.05 = 5%) |
| `metrics.conversions` | Conversions | float |
| `metrics.conversions_value` | Conversion value | float |
| `metrics.cost_per_conversion` | Cost per conversion | micros |
| `metrics.average_cpm` | Average cost per 1000 impressions | micros |
| `metrics.interaction_rate` | Interaction rate | decimal |
| `metrics.search_impression_share` | Search impression share | decimal |

## Operators

| Operator | Usage |
|---|---|
| `=` | Equals: `campaign.status = 'ENABLED'` |
| `!=` | Not equals: `campaign.status != 'REMOVED'` |
| `>`, `<`, `>=`, `<=` | Comparison: `metrics.clicks > 100` |
| `IN` | Set membership: `campaign.status IN ('ENABLED', 'PAUSED')` |
| `NOT IN` | Exclusion: `campaign.status NOT IN ('REMOVED')` |
| `LIKE` | Pattern match: `campaign.name LIKE '%brand%'` |
| `NOT LIKE` | Negative pattern: `campaign.name NOT LIKE '%test%'` |
| `CONTAINS ANY` | Array contains: `campaign.labels CONTAINS ANY ('customers/123/labels/456')` |
| `IS NULL` / `IS NOT NULL` | Null checks |
| `BETWEEN` | Range: `segments.date BETWEEN '2026-01-01' AND '2026-03-01'` |
| `DURING` | Date range shorthand: `segments.date DURING LAST_7_DAYS` |

## Common Query Patterns

### Campaign overview (last 7 days)
```sql
SELECT campaign.name, campaign.status,
  metrics.clicks, metrics.impressions, metrics.ctr,
  metrics.cost_micros, metrics.conversions
FROM campaign
WHERE segments.date DURING LAST_7_DAYS
  AND campaign.status != 'REMOVED'
ORDER BY metrics.cost_micros DESC
```

### Daily spend breakdown
```sql
SELECT segments.date, campaign.name,
  metrics.clicks, metrics.impressions, metrics.cost_micros
FROM campaign
WHERE segments.date DURING LAST_30_DAYS
  AND campaign.status = 'ENABLED'
ORDER BY segments.date DESC
```

### Top keywords by spend
```sql
SELECT ad_group_criterion.keyword.text,
  ad_group_criterion.keyword.match_type,
  metrics.clicks, metrics.impressions,
  metrics.cost_micros, metrics.average_cpc, metrics.ctr
FROM keyword_view
WHERE segments.date DURING LAST_30_DAYS
ORDER BY metrics.cost_micros DESC
LIMIT 20
```

### Search terms that triggered ads
```sql
SELECT search_term_view.search_term, campaign.name,
  metrics.clicks, metrics.impressions, metrics.cost_micros, metrics.ctr
FROM search_term_view
WHERE segments.date DURING LAST_30_DAYS
ORDER BY metrics.impressions DESC
LIMIT 50
```

### Ad performance
```sql
SELECT ad_group_ad.ad.responsive_search_ad.headlines,
  campaign.name, ad_group.name,
  metrics.clicks, metrics.impressions, metrics.ctr,
  metrics.cost_micros, metrics.conversions
FROM ad_group_ad
WHERE segments.date DURING LAST_30_DAYS
ORDER BY metrics.clicks DESC
```

### Budget utilisation
```sql
SELECT campaign.name, campaign_budget.amount_micros,
  campaign_budget.total_amount_micros,
  metrics.cost_micros
FROM campaign
WHERE segments.date DURING THIS_MONTH
  AND campaign.status = 'ENABLED'
```

### Geographic performance
```sql
SELECT geographic_view.country_criterion_id,
  geographic_view.location_type,
  metrics.clicks, metrics.impressions, metrics.cost_micros
FROM geographic_view
WHERE segments.date DURING LAST_30_DAYS
ORDER BY metrics.clicks DESC
LIMIT 20
```

### Device breakdown
```sql
SELECT segments.device, campaign.name,
  metrics.clicks, metrics.impressions,
  metrics.cost_micros, metrics.ctr
FROM campaign
WHERE segments.date DURING LAST_7_DAYS
  AND campaign.status = 'ENABLED'
```

### Change history
```sql
SELECT change_event.change_date_time,
  change_event.change_resource_type,
  change_event.changed_fields,
  change_event.old_resource, change_event.new_resource,
  change_event.user_email
FROM change_event
WHERE change_event.change_date_time DURING LAST_14_DAYS
ORDER BY change_event.change_date_time DESC
LIMIT 20
```

## Field Compatibility

Not all fields can be used together. Rules:
- Metrics from different resources can't be mixed in one query
- Some segments are incompatible (e.g. `segments.date` + `segments.hour_of_day` with some resources)
- Use `gads schema <resource> --selectable` to check available fields

If a query fails with a field compatibility error, check the fields against the resource schema.
