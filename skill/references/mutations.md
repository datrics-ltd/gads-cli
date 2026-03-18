# Mutations Reference

All mutations use `gads api POST` with resource-specific endpoints. Auth headers are injected automatically.

## Endpoint Pattern

```
POST /v23/customers/{customer_id}/<resource>:mutate
```

`{customer_id}` auto-replaces from config.

## Operation Types

Every mutation body follows this structure:

```json
{
  "operations": [
    {
      "create": { ... },
      "update": { ... },
      "updateMask": "field1,field2",
      "remove": "customers/123/resource/456"
    }
  ]
}
```

Each operation has exactly ONE of: `create`, `update` (with `updateMask`), or `remove`.

Multiple operations can be batched in one request.

## Resource Names

Resources reference each other by resource name string:

```
customers/{customer_id}/campaigns/{campaign_id}
customers/{customer_id}/adGroups/{ad_group_id}
customers/{customer_id}/adGroupAds/{ad_group_id}~{ad_id}
customers/{customer_id}/adGroupCriteria/{ad_group_id}~{criterion_id}
customers/{customer_id}/campaignBudgets/{budget_id}
```

## Common Mutations

### Create a Campaign Budget

```bash
gads api POST /v23/customers/{customer_id}/campaignBudgets:mutate -d '{
  "operations": [{
    "create": {
      "name": "My Budget",
      "amountMicros": "50000000",
      "deliveryMethod": "STANDARD"
    }
  }]
}'
```

50000000 micros = £50/day.

### Create a Campaign

Requires an existing budget resource name.

```bash
gads api POST /v23/customers/{customer_id}/campaigns:mutate -d '{
  "operations": [{
    "create": {
      "name": "My Search Campaign",
      "status": "PAUSED",
      "advertisingChannelType": "SEARCH",
      "campaignBudget": "customers/{customer_id}/campaignBudgets/BUDGET_ID",
      "targetSpend": {},
      "geoTargetTypeSetting": {
        "positiveGeoTargetType": "PRESENCE_OR_INTEREST",
        "negativeGeoTargetType": "PRESENCE_OR_INTEREST"
      }
    }
  }]
}'
```

Start campaigns as `PAUSED` — enable after review.

### Update Campaign Status

```bash
gads api POST /v23/customers/{customer_id}/campaigns:mutate -d '{
  "operations": [{
    "update": {
      "resourceName": "customers/{customer_id}/campaigns/CAMPAIGN_ID",
      "status": "ENABLED"
    },
    "updateMask": "status"
  }]
}'
```

Or use the named command: `gads campaigns enable CAMPAIGN_ID`

### Update Budget Amount

```bash
gads api POST /v23/customers/{customer_id}/campaignBudgets:mutate -d '{
  "operations": [{
    "update": {
      "resourceName": "customers/{customer_id}/campaignBudgets/BUDGET_ID",
      "amountMicros": "75000000"
    },
    "updateMask": "amount_micros"
  }]
}'
```

Or use: `gads budgets set BUDGET_ID --amount 75.00`

### Create an Ad Group

```bash
gads api POST /v23/customers/{customer_id}/adGroups:mutate -d '{
  "operations": [{
    "create": {
      "name": "My Ad Group",
      "campaign": "customers/{customer_id}/campaigns/CAMPAIGN_ID",
      "status": "ENABLED",
      "type": "SEARCH_STANDARD",
      "cpcBidMicros": "1000000"
    }
  }]
}'
```

`cpcBidMicros: 1000000` = £1.00 default CPC bid.

### Add Keywords

```bash
gads api POST /v23/customers/{customer_id}/adGroupCriteria:mutate -d '{
  "operations": [
    {
      "create": {
        "adGroup": "customers/{customer_id}/adGroups/AD_GROUP_ID",
        "status": "ENABLED",
        "keyword": {
          "text": "running shoes",
          "matchType": "BROAD"
        }
      }
    },
    {
      "create": {
        "adGroup": "customers/{customer_id}/adGroups/AD_GROUP_ID",
        "status": "ENABLED",
        "keyword": {
          "text": "best running shoes",
          "matchType": "PHRASE"
        }
      }
    }
  ]
}'
```

Match types: `BROAD`, `PHRASE`, `EXACT`.

Or use: `gads keywords add --ad-group AD_GROUP_ID --text "running shoes" --match-type BROAD`

### Add Negative Keywords (Campaign Level)

```bash
gads api POST /v23/customers/{customer_id}/campaignCriteria:mutate -d '{
  "operations": [{
    "create": {
      "campaign": "customers/{customer_id}/campaigns/CAMPAIGN_ID",
      "negative": true,
      "keyword": {
        "text": "free",
        "matchType": "BROAD"
      }
    }
  }]
}'
```

### Create a Responsive Search Ad

```bash
gads api POST /v23/customers/{customer_id}/adGroupAds:mutate -d '{
  "operations": [{
    "create": {
      "adGroup": "customers/{customer_id}/adGroups/AD_GROUP_ID",
      "status": "ENABLED",
      "ad": {
        "responsiveSearchAd": {
          "headlines": [
            {"text": "Buy Running Shoes", "pinnedField": "HEADLINE_1"},
            {"text": "Free Delivery Available"},
            {"text": "Shop Now"}
          ],
          "descriptions": [
            {"text": "Wide range of running shoes. Free delivery on orders over £50."},
            {"text": "Top brands at great prices. Shop our collection today."}
          ],
          "path1": "shoes",
          "path2": "running"
        },
        "finalUrls": ["https://example.com/running-shoes"]
      }
    }
  }]
}'
```

Minimum 3 headlines and 2 descriptions required.

### Remove a Resource

```bash
gads api POST /v23/customers/{customer_id}/campaigns:mutate -d '{
  "operations": [{
    "remove": "customers/{customer_id}/campaigns/CAMPAIGN_ID"
  }]
}'
```

**Note:** Removing a campaign sets it to REMOVED status — it cannot be deleted permanently. Prefer pausing instead.

### Partial Failure

Add `"partialFailure": true` to continue processing even if some operations fail:

```json
{
  "operations": [...],
  "partialFailure": true
}
```

Response will include both successful results and error details for failed operations.

## Update Mask

When updating, `updateMask` specifies which fields to change. Use snake_case field names, comma-separated:

```json
{
  "update": { "resourceName": "...", "status": "PAUSED", "name": "New Name" },
  "updateMask": "status,name"
}
```

Only fields listed in `updateMask` are modified. Omitted fields are left unchanged.

## Dry Run

Preview any mutation without executing:

```bash
gads api POST /v23/customers/{customer_id}/campaigns:mutate -d '...' --dry-run
```

Shows the full request (URL, headers, body) without sending it.
