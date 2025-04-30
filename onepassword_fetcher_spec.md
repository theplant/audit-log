# 1Password Events API Fetcher Specification

## Overview
Component responsible for fetching audit events from 1Password's Events API and transforming them into the unified log format.

## API Endpoint
- Base URL: `https://events.1password.com`
- Endpoint: `/api/v2/auditevents`
- Method: POST
- Authentication: Bearer token
- Content-Type: application/json

## Request Format
```json
{
  // For first request or resetting cursor
  "reset_cursor": {
    "start_time": "2023-01-01T00:00:00Z",
    "end_time": "2023-01-02T00:00:00Z",
    "limit": 1000
  }
  // OR for subsequent requests
  "cursor": "string"
}
```

## Required Parameters
- `start_time`: ISO8601 timestamp for start of period
- `end_time`: ISO8601 timestamp for end of period
- `cursor`: Pagination cursor
- `limit`: Number of records per request (max 1000)

## 1Password Audit Event Schema
```json
{
  "cursor": "string",
  "has_more": boolean,
  "items": [
    {
      "uuid": "string",
      "timestamp": "ISO8601 datetime",
      "action": "string",
      "type": "string",
      "actor": {
        "uuid": "string",
        "email": "string",
        "name": "string"
      },
      "target": {
        "uuid": "string",
        "type": "string",
        "name": "string"
      },
      "aux_data": {
        "item_uuid": "string",
        "vault_uuid": "string",
        "group_uuid": "string"
      }
    }
  ]
}
```

## Transformation Rules
Map 1Password audit events to unified schema:

```json
{
  "timestamp": "timestamp",
  "source": "1Password",
  "event_type": "type",
  "actor": {
    "id": "actor.uuid",
    "name": "actor.name",
    "email": "actor.email"
  },
  "targets": [
    {
      "id": "target.uuid",
      "type": "target.type",
      "name": "target.name"
    }
  ],
  "action": "action",
  "metadata": {
    "raw_event": "original_event",
    "enrichments": {
      "item_uuid": "aux_data.item_uuid",
      "vault_uuid": "aux_data.vault_uuid",
      "group_uuid": "aux_data.group_uuid"
    }
  }
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
onepassword:
  base_url: "https://events.1password.com"
  token: "string"  # Bearer token for authentication
  rate_limit: 100  # Requests per minute
  fetch_interval: "5m"  # How often to check for new events
  lookback_window: "24h"  # How far back to fetch events on startup
```

## Data Ordering
The 1Password Events API returns events in chronological order (oldest first), which matches the system requirements. No additional sorting or reversal is needed.

For example, when fetching events from 1:00 PM to 2:00 PM, the API returns:
```json
{
  "items": [
    {"timestamp": "2024-01-01T13:00:00Z", ...},
    {"timestamp": "2024-01-01T13:15:00Z", ...},
    {"timestamp": "2024-01-01T13:45:00Z", ...}
  ]
}
```

This ordering is maintained across pagination, with each subsequent page containing events that are newer than the previous page. 