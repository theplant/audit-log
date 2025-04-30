# Log Aggregation and Transformation System

## Overview
A system that fetches, transforms, and routes audit logs from multiple SaaS platforms and CSV sources to various destinations. The system normalizes different log formats into a unified schema while providing enrichment capabilities.

## Features
- Multi-source log ingestion (SaaS APIs, CSV files)
- Log transformation and normalization
- Data enrichment capabilities
- Multi-destination routing
- Configurable processing pipeline

## Supported Sources
- SaaS APIs:
  - GitHub
  - AWS CloudTrail
  - Google Workspace
  - Slack
  - Jira
  - 1Password
  - (Extensible for additional sources)

- CSV Files:
  - Custom format support
  - Automatic schema detection
  - Configurable field mapping

## Destination Support
- SIEM systems
- Log analytics platforms
- Cloud storage (S3, Azure Blob, etc.)

## Log Schema
```json
{
  "timestamp": "ISO8601 datetime",
  "source": "string",
  "event_type": "string",
  "actor": {
    "id": "string",
    "name": "string",
    "email": "string",
    "ip_address": "string"
  },
  "target": {
    "id": "string",
    "type": "string",
    "name": "string"
  },
  "action": "string",
  "metadata": {
    "raw_event": "object",
    "enrichments": "object"
  }
}
```

## Enrichment Capabilities
- IP geolocation
- User role mapping
- Asset classification
- Threat intelligence lookups
- Custom enrichment plugins

## Configuration
- YAML-based configuration
- Environment variable support
- Source-specific credentials management
- Rate limiting and throttling controls
- Retry policies

## Monitoring
- Health checks
- Performance metrics
- Error tracking
- Processing statistics
- Alerting capabilities

## Security
- Credential encryption
- TLS for all external communications
- Access control
- Audit logging of system operations

## Development Requirements
- Python 3.8+
- Docker support
- CI/CD pipeline integration
- Testing framework
- Documentation requirements

## Deployment Options
- Docker containers
- Kubernetes
- Serverless functions
- On-premise installation

## Future Considerations
- Real-time processing
- Machine learning integration
- Advanced analytics
- Custom plugin framework
- Horizontal scaling

## Log Fetcher Requirements

### Data Ordering
Log fetchers must return events in chronological order (oldest first). This is important for:
1. Consistent processing of events
2. Correct state management in Windmill scripts
3. Reliable incremental fetching

For example, if fetching logs from 1:00 PM to 2:00 PM:
```
[
  {timestamp: "1:00:00", ...},
  {timestamp: "1:00:05", ...},
  {timestamp: "1:15:30", ...},
  {timestamp: "1:59:59", ...}
]
```

This ordering requirement applies both to:
- Events within a single page of results
- Events across multiple pages when pagination is used
