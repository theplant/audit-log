package schema

import (
	"context"
	"time"
)

// Actor represents the entity performing an action
type Actor struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	IPAddress string `json:"ip_address"`
}

// Target represents the entity being acted upon
type Target struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Metadata contains additional information about the event
type Metadata struct {
	RawEvent    map[string]interface{} `json:"raw_event"`
	Enrichments map[string]interface{} `json:"enrichments"`
}

// LogEvent represents the unified log format
type LogEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	EventType string    `json:"event_type"`
	Actor     Actor     `json:"actor"`
	Targets   []Target  `json:"targets"`
	Action    string    `json:"action"`
	Metadata  Metadata  `json:"metadata"`
}

// SourceConfig represents the configuration for a log source
type SourceConfig struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings"`
}

// DestinationConfig represents the configuration for a log destination
type DestinationConfig struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings"`
}

// EnrichmentConfig represents the configuration for an enrichment
type EnrichmentConfig struct {
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings"`
}

// LogFetcher defines the interface for fetching logs from a source
type LogFetcher interface {
	FetchLogs(ctx context.Context, from, to time.Time) ([]LogEvent, error)
}
