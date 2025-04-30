package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/theplant/audit-log/schema"
)

// JiraFetcher implements the log fetching for Jira audit logs
type JiraFetcher struct {
	config     *JiraConfig
	httpClient *http.Client
	enricher   *UserEnricher
}

// NewFetcher creates a new JiraFetcher instance
func NewFetcher(config *JiraConfig) *JiraFetcher {
	return &JiraFetcher{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		enricher:   NewUserEnricher(config),
	}
}

// FetchLogs retrieves audit logs from Jira API
func (f *JiraFetcher) FetchLogs(ctx context.Context, from, to time.Time) ([]schema.LogEvent, error) {
	var allEvents []schema.LogEvent
	offset := 0
	limit := 1000

	for {
		events, hasMore, err := f.fetchPage(ctx, from, to, offset, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch Jira logs: %w", err)
		}

		allEvents = append(allEvents, events...)

		if !hasMore {
			break
		}

		offset += limit
	}

	return allEvents, nil
}

// fetchPage retrieves a single page of audit logs
func (f *JiraFetcher) fetchPage(ctx context.Context, from, to time.Time, offset, limit int) ([]schema.LogEvent, bool, error) {
	// Format dates in UTC using RFC3339
	fromStr := from.UTC().Format(time.RFC3339)
	toStr := to.UTC().Format(time.RFC3339)

	url := fmt.Sprintf("%s/rest/api/3/auditing/record?from=%s&to=%s&offset=%d&limit=%d",
		f.config.BaseURL,
		fromStr,
		toStr,
		offset,
		limit,
	)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, err
	}

	req.SetBasicAuth(f.config.Username, f.config.Password)
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, false, fmt.Errorf("unexpected status code %d and failed to read error response: %v", resp.StatusCode, err)
		}
		return nil, false, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Read and log the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read response body: %v", err)
	}
	//	fmt.Printf("Raw response: %s\n", string(body))

	var jiraResponse struct {
		Records []JiraAuditRecord `json:"records"`
		Offset  int               `json:"offset"`
		Limit   int               `json:"limit"`
		Total   int               `json:"total"`
	}

	if err := json.Unmarshal(body, &jiraResponse); err != nil {
		return nil, false, fmt.Errorf("failed to parse response: %w, body: %s", err, string(body))
	}

	events := make([]schema.LogEvent, 0, len(jiraResponse.Records))
	for _, record := range jiraResponse.Records {
		event, err := f.transformRecord(record)
		if err != nil {
			return nil, false, err
		}
		events = append(events, event)
	}

	hasMore := offset+limit < jiraResponse.Total
	return events, hasMore, nil
}

// JiraItem represents a Jira item (object or associated)
type JiraItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	TypeName string `json:"typeName"`
}

// JiraAuditRecord represents the raw Jira audit log record
type JiraAuditRecord struct {
	ID              int64     `json:"id"`
	Summary         string    `json:"summary"`
	RemoteAddress   string    `json:"remoteAddress"`
	AuthorAccountID string    `json:"authorAccountId"`
	AuthorKey       string    `json:"authorKey"`
	Created         time.Time `json:"created"`
	Category        string    `json:"category"`
	EventSource     string    `json:"eventSource"`
	Description     string    `json:"description"`
	ObjectItem      JiraItem  `json:"objectItem"`
	ChangedValues   []struct {
		FieldName   string `json:"fieldName"`
		ChangedFrom string `json:"changedFrom"`
		ChangedTo   string `json:"changedTo"`
	} `json:"changedValues"`
	AssociatedItems []JiraItem `json:"associatedItems"`
}

// UnmarshalJSON implements custom JSON unmarshaling for JiraAuditRecord
func (r *JiraAuditRecord) UnmarshalJSON(data []byte) error {
	type Alias JiraAuditRecord
	aux := &struct {
		Created interface{} `json:"created"`
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Handle both string and number formats
	switch v := aux.Created.(type) {
	case string:
		created, err := time.Parse("2006-01-02T15:04:05.999-0700", v)
		if err != nil {
			return err
		}
		r.Created = created
	case float64:
		r.Created = time.Unix(int64(v/1000), 0) // Convert milliseconds to time
	default:
		return fmt.Errorf("unexpected type for created: %T", v)
	}
	return nil
}

func (f *JiraFetcher) enrichTarget(target schema.Target) schema.Target {
	if target.Type == "USER" && target.ID != "" {
		if user, err := f.enricher.EnrichUser(context.Background(), target.ID); err == nil {
			target.Metadata = map[string]interface{}{
				"display_name": user.DisplayName,
				"email":        user.Email,
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: failed to enrich user %s: %v\n", target.ID, err)
		}
	}
	return target
}

func (f *JiraFetcher) transformRecord(record JiraAuditRecord) (schema.LogEvent, error) {
	// Convert raw record to map for metadata
	rawEvent, err := json.Marshal(record)
	if err != nil {
		return schema.LogEvent{}, err
	}

	var rawEventMap map[string]interface{}
	if err := json.Unmarshal(rawEvent, &rawEventMap); err != nil {
		return schema.LogEvent{}, err
	}

	// Use AuthorAccountID, fall back to AuthorKey if not available
	authorID := record.AuthorAccountID
	if authorID == "" {
		authorID = record.AuthorKey
	}

	// Initialize actor with basic info
	actor := schema.Actor{
		ID:        authorID,
		IPAddress: record.RemoteAddress,
	}

	// Only enrich user information if we have an authorAccountID
	if record.AuthorAccountID != "" {
		if user, err := f.enricher.EnrichUser(context.Background(), record.AuthorAccountID); err == nil {
			actor.Name = user.DisplayName
			actor.Email = user.Email
		} else {
			fmt.Fprintf(os.Stderr, "Warning: failed to enrich user %s: %v\n", record.AuthorAccountID, err)
		}
	}

	// Convert ObjectItem and AssociatedItems to targets
	targets := make([]schema.Target, 0, len(record.AssociatedItems))

	// First collect all associated items
	for _, item := range record.AssociatedItems {
		target := schema.Target{
			ID:   item.ID,
			Type: item.TypeName,
			Name: item.Name,
		}
		targets = append(targets, f.enrichTarget(target))
	}

	// Add ObjectItem only if it's not already in AssociatedItems
	isDuplicate := false
	for _, target := range targets {
		if record.ObjectItem.ID != "" && target.ID == record.ObjectItem.ID {
			isDuplicate = true
			break
		}
	}
	if !isDuplicate {
		target := schema.Target{
			ID:   record.ObjectItem.ID, // Can be empty
			Type: record.ObjectItem.TypeName,
			Name: record.ObjectItem.Name,
		}
		targets = append(targets, f.enrichTarget(target))
	}

	return schema.LogEvent{
		Timestamp: record.Created,
		Source:    "Jira",
		EventType: record.Category,
		Actor:     actor,
		Targets:   targets,
		Action:    record.Summary,
		Metadata: schema.Metadata{
			RawEvent: rawEventMap,
			Enrichments: map[string]interface{}{
				"changed_values": record.ChangedValues,
				"description":    record.Description,
				"event_source":   record.EventSource,
				// Remove associated_items since they're now in Targets
			},
		},
	}, nil
}
