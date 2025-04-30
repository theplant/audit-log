package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/theplant/audit-log/schema"
)

func TestJiraFetcher(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Verify query parameters
		query := r.URL.Query()
		assert.Equal(t, "2023-01-01T00:00:00Z", query.Get("from"))
		assert.Equal(t, "2023-01-02T00:00:00Z", query.Get("to"))
		assert.Equal(t, "0", query.Get("offset"))
		assert.Equal(t, "1000", query.Get("limit"))

		// Return mock response
		response := map[string]interface{}{
			"records": []map[string]interface{}{
				{
					"id":            123,
					"summary":       "User logged in",
					"remoteAddress": "192.168.1.1",
					"authorKey":     "user1",
					"created":       "2023-01-01T12:00:00Z",
					"category":      "login",
					"eventSource":   "web",
					"description":   "User logged in successfully",
					"objectItem": map[string]interface{}{
						"id":       "user1",
						"name":     "User One",
						"typeName": "user",
					},
				},
			},
			"offset": 0,
			"limit":  1000,
			"total":  1,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create fetcher with test configuration
	config := &JiraConfig{
		BaseURL:     server.URL,
		Username:    "testuser",
		Password:    "testpass",
		RateLimit:   100,
		FetchWindow: 24 * time.Hour,
	}

	fetcher := NewFetcher(config)

	// Test fetching logs
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)

	events, err := fetcher.FetchLogs(context.Background(), from, to)
	require.NoError(t, err)
	require.Len(t, events, 1)

	// Verify transformed event
	event := events[0]
	assert.Equal(t, "Jira", event.Source)
	assert.Equal(t, "login", event.EventType)
	assert.Equal(t, "user1", event.Actor.ID)
	assert.Equal(t, "192.168.1.1", event.Actor.IPAddress)
	require.Len(t, event.Targets, 1)
	assert.Equal(t, "user1", event.Targets[0].ID)
	assert.Equal(t, "user", event.Targets[0].Type)
	assert.Equal(t, "User One", event.Targets[0].Name)
	assert.Equal(t, "User logged in", event.Action)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *JiraConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &JiraConfig{
				BaseURL:  "https://test.atlassian.net",
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "missing base url",
			config: &JiraConfig{
				Username: "user",
				Password: "pass",
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: &JiraConfig{
				BaseURL:  "https://test.atlassian.net",
				Password: "pass",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: &JiraConfig{
				BaseURL:  "https://test.atlassian.net",
				Username: "user",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnvironmentConfig(t *testing.T) {
	// Set environment variables
	os.Setenv("JIRA_BASE_URL", "https://test.atlassian.net")
	os.Setenv("JIRA_USERNAME", "envuser")
	os.Setenv("JIRA_PASSWORD", "envpass")
	os.Setenv("JIRA_RATE_LIMIT", "200")
	os.Setenv("JIRA_FETCH_WINDOW", "48h")

	config := NewConfigFromEnv()

	assert.Equal(t, "https://test.atlassian.net", config.BaseURL)
	assert.Equal(t, "envuser", config.Username)
	assert.Equal(t, "envpass", config.Password)
	assert.Equal(t, 200, config.RateLimit)
	assert.Equal(t, 48*time.Hour, config.FetchWindow)

	// Clean up environment variables
	os.Unsetenv("JIRA_BASE_URL")
	os.Unsetenv("JIRA_USERNAME")
	os.Unsetenv("JIRA_PASSWORD")
	os.Unsetenv("JIRA_RATE_LIMIT")
	os.Unsetenv("JIRA_FETCH_WINDOW")
}

func TestJiraTargets(t *testing.T) {
	tests := []struct {
		name            string
		objectItem      JiraItem
		associatedItems []JiraItem
		wantTargets     []schema.Target
	}{
		{
			name: "object_item_only",
			objectItem: JiraItem{
				ID:       "OBJ-1",
				Name:     "Object One",
				TypeName: "object",
			},
			associatedItems: nil,
			wantTargets: []schema.Target{
				{
					ID:   "OBJ-1",
					Name: "Object One",
					Type: "object",
				},
			},
		},
		{
			name:       "associated_items_only",
			objectItem: JiraItem{},
			associatedItems: []JiraItem{
				{
					ID:       "ASSOC-1",
					Name:     "Associated One",
					TypeName: "associated",
				},
				{
					ID:       "ASSOC-2",
					Name:     "Associated Two",
					TypeName: "associated",
				},
			},
			wantTargets: []schema.Target{
				{
					ID:   "ASSOC-1",
					Name: "Associated One",
					Type: "associated",
				},
				{
					ID:   "ASSOC-2",
					Name: "Associated Two",
					Type: "associated",
				},
			},
		},
		{
			name: "duplicate_object_item",
			objectItem: JiraItem{
				ID:       "ASSOC-1", // Same ID as associated item
				Name:     "Object One",
				TypeName: "object",
			},
			associatedItems: []JiraItem{
				{
					ID:       "ASSOC-1",
					Name:     "Associated One",
					TypeName: "associated",
				},
			},
			wantTargets: []schema.Target{
				{
					ID:   "ASSOC-1",
					Name: "Associated One",
					Type: "associated",
				},
			},
		},
		{
			name: "both_unique",
			objectItem: JiraItem{
				ID:       "OBJ-1",
				Name:     "Object One",
				TypeName: "object",
			},
			associatedItems: []JiraItem{
				{
					ID:       "ASSOC-1",
					Name:     "Associated One",
					TypeName: "associated",
				},
			},
			wantTargets: []schema.Target{
				{
					ID:   "ASSOC-1",
					Name: "Associated One",
					Type: "associated",
				},
				{
					ID:   "OBJ-1",
					Name: "Object One",
					Type: "object",
				},
			},
		},
		{
			name:            "both_null",
			objectItem:      JiraItem{},        // Empty object item
			associatedItems: nil,               // Null associated items
			wantTargets:     []schema.Target{}, // Expect empty targets
		},
		{
			name: "object_item_without_id",
			objectItem: JiraItem{
				Name:     "Object One",
				TypeName: "object",
				// ID intentionally empty
			},
			associatedItems: nil,
			wantTargets: []schema.Target{
				{
					// ID intentionally empty
					Name: "Object One",
					Type: "object",
				},
			},
		},
		{
			name: "object_item_with_associated_items_no_id",
			objectItem: JiraItem{
				Name:     "Object One",
				TypeName: "object",
				// ID intentionally empty
			},
			associatedItems: []JiraItem{
				{
					ID:       "ASSOC-1",
					Name:     "Associated One",
					TypeName: "associated",
				},
			},
			wantTargets: []schema.Target{
				{
					ID:   "ASSOC-1",
					Name: "Associated One",
					Type: "associated",
				},
				{
					// ID intentionally empty
					Name: "Object One",
					Type: "object",
				},
			},
		},
		{
			name: "user_target_enrichment",
			objectItem: JiraItem{
				ID:       "USER-1",
				Name:     "username",
				TypeName: "USER",
			},
			associatedItems: nil,
			wantTargets: []schema.Target{
				{
					ID:   "USER-1",
					Name: "username",
					Type: "USER",
					Metadata: map[string]interface{}{
						"display_name": "Test User",
						"email":        "test@example.com",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := JiraAuditRecord{
				ObjectItem:      tt.objectItem,
				AssociatedItems: tt.associatedItems,
			}

			config := &JiraConfig{} // Minimal config for test
			fetcher := NewFetcher(config)
			event, err := fetcher.transformRecord(record)
			require.NoError(t, err)

			assert.Equal(t, tt.wantTargets, event.Targets)
		})
	}
}
