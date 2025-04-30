# Jira Audit Log Fetcher Specification

## Overview
Component responsible for fetching audit logs from Jira's REST API and transforming them into the unified log format.

## API Endpoint
- Base URL: `https://{your-domain}.atlassian.net`
- Endpoint: `/rest/api/3/auditing/record`
- Authentication: Basic Auth or OAuth 2.0
- Rate Limits: 100 requests per minute (adjustable based on Jira plan)

## Required Parameters
- `from`: ISO8601 timestamp for start of period
- `to`: ISO8601 timestamp for end of period
- `offset`: For pagination
- `limit`: Number of records per request (max 1000)

## Jira Audit Log Schema
```json
{
  "records": [
    {
      "id": "string",
      "summary": "string",
      "remoteAddress": "string",
      "authorKey": "string",
      "created": "ISO8601 datetime",
      "category": "string",
      "eventSource": "string",
      "description": "string",
      "objectItem": {
        "id": "string",
        "name": "string",
        "typeName": "string"
      },
      "changedValues": [
        {
          "fieldName": "string",
          "changedFrom": "string",
          "changedTo": "string"
        }
      ],
      "associatedItems": [
        {
          "id": "string",
          "name": "string",
          "typeName": "string"
        }
      ]
    }
  ],
  "offset": "integer",
  "limit": "integer",
  "total": "integer"
}
```

## Transformation Rules
Map Jira audit log fields to unified schema:

```json
{
  "timestamp": "created",
  "source": "Jira",
  "event_type": "category",
  "actor": {
    "id": "authorAccountId",
    "name": "authorKey",
    "email": "authorKey",
    "ip_address": "remoteAddress"
  },
  "targets": [
    {
      "id": "item.id",
      "type": "item.typeName",
      "name": "item.name"
    }
  ],
  "action": "summary",
  "metadata": {
    "raw_event": "original_jira_event",
    "enrichments": {
      "changed_values": "changedValues",
      "description": "description",
      "event_source": "eventSource"
    }
  }
}
```

### Target Mapping Rules
The `targets` field combines both `objectItem` and `associatedItems` from the Jira audit log with the following rules:

1. All `associatedItems` are included as targets
2. The `objectItem` is included as a target only if its ID is not already present in any of the `associatedItems`
3. Target order:
   - Associated items are added first, in their original order
   - Object item is added last (if not duplicate)
4. Each target preserves the original item's:
   - `id` → Target.ID
   - `typeName` → Target.Type
   - `name` → Target.Name

This approach ensures:
- No duplicate targets in the output
- All relevant items are captured
- Associated items take precedence over object item when IDs match

### Target Enrichment Rules
When a target has `type: "USER"`, additional user information is added to the target's metadata:
```json
{
  "targets": [
    {
      "id": "user123",
      "type": "USER",
      "name": "username",
      "metadata": {
        "display_name": "User's Full Name",
        "email": "user@example.com"
      }
    }
  ]
}
```

## Error Handling
- API rate limit exceeded
- Authentication failures
- Network timeouts
- Invalid response format
- Missing required fields

## Retry Strategy
- Exponential backoff
- Maximum 3 retries
- Retry on:
  - 429 (Too Many Requests)
  - 500-599 (Server Errors)
  - Network timeouts

## Configuration Requirements
```yaml
jira:
  base_url: "https://{your-domain}.atlassian.net"
  auth:
    type: "basic" # or "oauth2"
    username: "string"
    password: "string" # or api_token
    # OAuth2 specific fields if applicable
  rate_limit:
    requests_per_minute: 100
  fetch_interval: "5m" # How often to check for new logs
  lookback_window: "24h" # How far back to fetch logs on startup
```

## Additional Considerations
- User information enrichment (fetch user details for authorKey)
- Batch processing for large result sets
- State management for incremental fetching
- Logging and monitoring
- Error reporting and alerting 