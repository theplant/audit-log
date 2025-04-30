package onepassword

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/theplant/audit-log/schema"
)

// Fetcher implements the log fetching for 1Password Events API
type Fetcher struct {
	config     *Config
	httpClient *http.Client
}

// NewFetcher creates a new Fetcher instance
func NewFetcher(config *Config) *Fetcher {
	return &Fetcher{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// AuditEvent represents a 1Password audit event (V2)
type AuditEvent struct {
	UUID         string    `json:"uuid"`
	Timestamp    time.Time `json:"timestamp"`
	ActorUUID    string    `json:"actor_uuid"`
	Action       string    `json:"action"`
	ObjectType   string    `json:"object_type"`
	ObjectUUID   string    `json:"object_uuid"`
	ActorDetails struct {
		UUID  string `json:"uuid"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"actor_details"`
	AuxID      int    `json:"aux_id"`
	AuxUUID    string `json:"aux_uuid"`
	AuxDetails struct {
		UUID  string `json:"uuid"`
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"aux_details"`
	AuxInfo string `json:"aux_info"`
	Session struct {
		UUID       string `json:"uuid"`
		LoginTime  string `json:"login_time"`
		DeviceUUID string `json:"device_uuid"`
		IP         string `json:"ip"`
	} `json:"session"`
	Location struct {
		Country   string  `json:"country"`
		Region    string  `json:"region"`
		City      string  `json:"city"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

// FetchLogs retrieves audit events from 1Password Events API
func (f *Fetcher) FetchLogs(ctx context.Context, from, to time.Time) ([]schema.LogEvent, error) {
	var allEvents []schema.LogEvent
	cursor := ""
	seenCursors := make(map[string]bool)
	pageCount := 0
	maxPages := 1000 // Prevent infinite loops

	for {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled while fetching logs: %w", err)
		}

		// Check maximum pages
		if pageCount >= maxPages {
			return nil, fmt.Errorf("exceeded maximum number of pages (%d)", maxPages)
		}

		// Fetch page with exponential backoff
		var events []schema.LogEvent
		var nextCursor string
		var hasMore bool
		var err error

		operation := func() error {
			events, nextCursor, hasMore, err = f.fetchPage(ctx, from, to, cursor)
			return err
		}

		b := backoff.NewExponentialBackOff()
		b.InitialInterval = 100 * time.Millisecond
		b.MaxInterval = 10 * time.Second
		b.MaxElapsedTime = 2 * time.Minute
		b.Multiplier = 2
		b.RandomizationFactor = 0.1

		if err := backoff.Retry(operation, b); err != nil {
			return nil, fmt.Errorf("failed to fetch page after retries: %w", err)
		}

		allEvents = append(allEvents, events...)
		pageCount++

		if !hasMore {
			break
		}

		// Check for cursor loop
		if seenCursors[nextCursor] {
			return nil, fmt.Errorf("detected cursor loop: cursor %s seen multiple times", nextCursor)
		}
		seenCursors[nextCursor] = true

		// Check for empty/invalid cursor
		if nextCursor == "" {
			return nil, fmt.Errorf("received empty cursor with hasMore=true")
		}

		cursor = nextCursor

		// Rate limiting
		if f.config.RateLimit > 0 {
			time.Sleep(time.Second / time.Duration(f.config.RateLimit))
		}
	}

	return allEvents, nil
}

func (f *Fetcher) fetchPage(ctx context.Context, from, to time.Time, cursor string) ([]schema.LogEvent, string, bool, error) {
	url := fmt.Sprintf("%s/api/v2/auditevents", f.config.BaseURL)

	// Build request body
	body := map[string]interface{}{
		"limit": 1000,
	}
	if cursor != "" {
		body["cursor"] = cursor
	} else {
		// Put time range fields at top level
		body["start_time"] = from.Format(time.RFC3339)
		body["end_time"] = to.Format(time.RFC3339)
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, "", false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, "", false, err
	}

	req.Header.Set("Authorization", "Bearer "+f.config.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, "", false, fmt.Errorf("unexpected status code %d and failed to read error response: %v", resp.StatusCode, err)
		}
		return nil, "", false, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Cursor  string       `json:"cursor"`
		HasMore bool         `json:"has_more"`
		Items   []AuditEvent `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, "", false, err
	}

	events := make([]schema.LogEvent, 0, len(response.Items))
	for _, item := range response.Items {
		event, err := f.transformEvent(item)
		if err != nil {
			return nil, "", false, err
		}
		events = append(events, event)
	}

	return events, response.Cursor, response.HasMore, nil
}

func (f *Fetcher) transformEvent(event AuditEvent) (schema.LogEvent, error) {
	// Convert raw event to map for metadata
	rawEvent, err := json.Marshal(event)
	if err != nil {
		return schema.LogEvent{}, err
	}

	var rawEventMap map[string]interface{}
	if err := json.Unmarshal(rawEvent, &rawEventMap); err != nil {
		return schema.LogEvent{}, err
	}

	return schema.LogEvent{
		Timestamp: event.Timestamp,
		Source:    "1Password",
		EventType: event.ObjectType,
		Actor: schema.Actor{
			ID:    event.ActorDetails.UUID,
			Name:  event.ActorDetails.Name,
			Email: event.ActorDetails.Email,
		},
		Targets: []schema.Target{
			{
				ID:   event.ObjectUUID,
				Type: event.ObjectType,
				Name: event.ObjectType,
			},
		},
		Action: event.Action,
		Metadata: schema.Metadata{
			RawEvent: rawEventMap,
			Enrichments: map[string]interface{}{
				"item_uuid":  event.ObjectUUID,
				"vault_uuid": event.ObjectUUID,
				"group_uuid": event.ObjectUUID,
			},
		},
	}, nil
}
