package onepassword

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnePasswordFetcher(t *testing.T) {
	// Define nested structs to match AuditEvent
	type ActorInfo struct {
		UUID     string `json:"uuid"`
		Email    string `json:"email"`
		Name     string `json:"name"`
		SignInAt string `json:"sign_in_at,omitempty"`
		IPAddr   string `json:"ip_addr,omitempty"`
	}

	type TargetInfo struct {
		UUID     string `json:"uuid"`
		Type     string `json:"type"`
		Name     string `json:"name"`
		SignInAt string `json:"sign_in_at,omitempty"`
		IPAddr   string `json:"ip_addr,omitempty"`
		Version  int    `json:"version,omitempty"`
		State    string `json:"state,omitempty"`
	}

	type ClientInfo struct {
		AppName      string `json:"app_name,omitempty"`
		AppVersion   string `json:"app_version,omitempty"`
		PlatformName string `json:"platform_name,omitempty"`
		PlatformURL  string `json:"platform_url,omitempty"`
		OSName       string `json:"os_name,omitempty"`
		OSVersion    string `json:"os_version,omitempty"`
		IPAddr       string `json:"ip_addr,omitempty"`
	}

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Verify path
		if r.URL.Path != "/api/v2/auditevents" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Verify Content-Type header
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		// Parse request body
		var reqBody struct {
			Cursor      string `json:"cursor,omitempty"`
			ResetCursor *struct {
				StartTime string `json:"start_time"`
				EndTime   string `json:"end_time"`
				Limit     int    `json:"limit"`
			} `json:"reset_cursor,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Verify auth header
		token := r.Header.Get("Authorization")
		if token != "Bearer testtoken" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return mock response based on cursor
		cursor := reqBody.Cursor
		var response struct {
			Cursor  string       `json:"cursor"`
			HasMore bool         `json:"has_more"`
			Items   []AuditEvent `json:"items"`
		}

		if cursor == "" {
			// First page
			response = struct {
				Cursor  string       `json:"cursor"`
				HasMore bool         `json:"has_more"`
				Items   []AuditEvent `json:"items"`
			}{
				Cursor:  "page2",
				HasMore: true,
				Items: []AuditEvent{
					{
						UUID:       "event1",
						Timestamp:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
						Action:     "item.view",
						ObjectType: "item",
						ObjectUUID: "item1",
						ActorDetails: struct {
							UUID  string `json:"uuid"`
							Name  string `json:"name"`
							Email string `json:"email"`
						}{
							UUID:  "user1",
							Name:  "Test User",
							Email: "user1@example.com",
						},
						Session: struct {
							UUID       string `json:"uuid"`
							LoginTime  string `json:"login_time"`
							DeviceUUID string `json:"device_uuid"`
							IP         string `json:"ip"`
						}{
							UUID:      "session1",
							LoginTime: "2023-01-01T11:00:00Z",
							IP:        "192.168.1.1",
						},
					},
				},
			}
		} else {
			// Second page
			response = struct {
				Cursor  string       `json:"cursor"`
				HasMore bool         `json:"has_more"`
				Items   []AuditEvent `json:"items"`
			}{
				HasMore: false,
				Items: []AuditEvent{
					{
						UUID:       "event2",
						Timestamp:  time.Date(2023, 1, 1, 13, 0, 0, 0, time.UTC),
						Action:     "vault.create",
						ObjectType: "vault",
						ObjectUUID: "vault2",
						ActorDetails: struct {
							UUID  string `json:"uuid"`
							Name  string `json:"name"`
							Email string `json:"email"`
						}{
							UUID:  "user1",
							Name:  "Test User",
							Email: "user1@example.com",
						},
					},
				},
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create fetcher with test configuration
	config := &Config{
		BaseURL: server.URL,
		Token:   "testtoken",
	}

	fetcher := NewFetcher(config)

	// Test fetching logs
	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)

	events, err := fetcher.FetchLogs(context.Background(), from, to)
	require.NoError(t, err)
	require.Len(t, events, 2)

	// Verify first event
	event1 := events[0]
	assert.Equal(t, "1Password", event1.Source)
	assert.Equal(t, "item", event1.EventType)
	assert.Equal(t, "user1", event1.Actor.ID)
	assert.Equal(t, "Test User", event1.Actor.Name)
	assert.Equal(t, "user1@example.com", event1.Actor.Email)
	require.Len(t, event1.Targets, 1)
	assert.Equal(t, "item1", event1.Targets[0].ID)
	assert.Equal(t, "item", event1.Targets[0].Type)
	assert.Equal(t, "item", event1.Targets[0].Name)
	assert.Equal(t, "item.view", event1.Action)

	// Verify raw event is included
	assert.Equal(t, "event1", event1.Metadata.RawEvent["uuid"])
	assert.Equal(t, "item.view", event1.Metadata.RawEvent["action"])
	assert.Equal(t, "item", event1.Metadata.RawEvent["object_type"])
	assert.Equal(t, "user1", event1.Metadata.RawEvent["actor_details"].(map[string]interface{})["uuid"])

	// Verify enrichments
	assert.Equal(t, "item1", event1.Metadata.Enrichments["item_uuid"])
	assert.Equal(t, "item1", event1.Metadata.Enrichments["vault_uuid"])
	assert.Equal(t, "item1", event1.Metadata.Enrichments["group_uuid"])

	// Verify second event
	event2 := events[1]
	assert.Equal(t, "vault", event2.EventType)
	assert.Equal(t, "vault.create", event2.Action)
	require.Len(t, event2.Targets, 1)
	assert.Equal(t, "vault2", event2.Targets[0].ID)
	assert.Equal(t, "vault", event2.Targets[0].Type)
	assert.Equal(t, "vault", event2.Targets[0].Name)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Token: "testtoken",
			},
			wantErr: false,
		},
		{
			name: "missing token",
			config: &Config{
				BaseURL: "https://events.1password.com",
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

func TestAuditEventDecoding(t *testing.T) {
	// Example response from 1Password API documentation
	jsonData := `{
			"uuid": "56YE2TYN2VFYRLNSHKPW5NVT5E",
			"timestamp": "2023-03-15T16:33:50-03:00",
			"actor_uuid": "4HCGRGYCTRQFBMGVEGTABYDU2V",
			"actor_details": {
				"uuid": "4HCGRGYCTRQFBMGVEGTABYDU2V",
				"name": "Jeff Shiner",
				"email": "jeff_shiner@agilebits.com"
			},
			"action": "join",
			"object_type": "gm",
			"object_uuid": "pf8soyakgngrphytsyjed4ae3u",
			"aux_id": 9277034,
			"aux_uuid": "K6VFYDCJKHGGDI7QFAXX65LCDY",
			"aux_details": {
				"uuid": "K6VFYDCJKHGGDI7QFAXX65LCDY",
				"name": "Wendy Appleseed",
				"email": "wendy_appleseed@agilebits.com"
			},
			"aux_info": "R",
			"session": {
				"uuid": "A5K6COGVRVEJXJW3XQZGS7VAMM",
				"login_time": "2023-03-15T16:33:50-03:00",
				"device_uuid": "lc5fqgbrcm4plajd8mwncv2b3u",
				"ip": "192.0.2.254"
			},
			"location": {
				"country": "Canada",
				"region": "Ontario",
				"city": "Toronto",
				"latitude": 43.5991,
				"longitude": -79.4988
			}
		}`

	var event AuditEvent
	err := json.Unmarshal([]byte(jsonData), &event)
	require.NoError(t, err)

	// Verify all fields were decoded correctly
	assert.Equal(t, "56YE2TYN2VFYRLNSHKPW5NVT5E", event.UUID)
	assert.Equal(t, "2023-03-15T16:33:50-03:00", event.Timestamp.Format(time.RFC3339))
	assert.Equal(t, "join", event.Action)
	assert.Equal(t, "gm", event.ObjectType)

	// Actor fields
	assert.Equal(t, "4HCGRGYCTRQFBMGVEGTABYDU2V", event.ActorUUID)
	assert.Equal(t, "jeff_shiner@agilebits.com", event.ActorDetails.Email)
	assert.Equal(t, "Jeff Shiner", event.ActorDetails.Name)

	// Object fields
	assert.Equal(t, "pf8soyakgngrphytsyjed4ae3u", event.ObjectUUID)
	assert.Equal(t, "gm", event.ObjectType)

	// Session fields
	assert.Equal(t, "A5K6COGVRVEJXJW3XQZGS7VAMM", event.Session.UUID)
	assert.Equal(t, "2023-03-15T16:33:50-03:00", event.Session.LoginTime)
	assert.Equal(t, "lc5fqgbrcm4plajd8mwncv2b3u", event.Session.DeviceUUID)
	assert.Equal(t, "192.0.2.254", event.Session.IP)

	// Location fields
	assert.Equal(t, "Canada", event.Location.Country)
	assert.Equal(t, "Ontario", event.Location.Region)
	assert.Equal(t, "Toronto", event.Location.City)
	assert.Equal(t, 43.5991, event.Location.Latitude)
	assert.Equal(t, -79.4988, event.Location.Longitude)
}

func TestFetchLogsErrorHandling(t *testing.T) {
	tests := []struct {
		name           string
		serverBehavior func(w http.ResponseWriter, r *http.Request, callCount *int)
		wantErr        string
	}{
		{
			name: "cursor_loop",
			serverBehavior: func(w http.ResponseWriter, r *http.Request, callCount *int) {
				*callCount++
				json.NewEncoder(w).Encode(map[string]interface{}{
					"cursor":   "same_cursor",
					"has_more": true,
					"items":    []interface{}{},
				})
			},
			wantErr: "detected cursor loop",
		},
		{
			name: "empty_cursor",
			serverBehavior: func(w http.ResponseWriter, r *http.Request, callCount *int) {
				*callCount++
				json.NewEncoder(w).Encode(map[string]interface{}{
					"cursor":   "",
					"has_more": true,
					"items":    []interface{}{},
				})
			},
			wantErr: "received empty cursor",
		},
		{
			name: "too_many_pages",
			serverBehavior: func(w http.ResponseWriter, r *http.Request, callCount *int) {
				*callCount++
				json.NewEncoder(w).Encode(map[string]interface{}{
					"cursor":   fmt.Sprintf("page_%d", *callCount),
					"has_more": true,
					"items":    []interface{}{},
				})
			},
			wantErr: "exceeded maximum number of pages",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				tt.serverBehavior(w, r, &callCount)
			}))
			defer server.Close()

			config := &Config{
				BaseURL: server.URL,
				Token:   "test",
			}
			fetcher := NewFetcher(config)

			_, err := fetcher.FetchLogs(context.Background(), time.Now().Add(-1*time.Hour), time.Now())
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestResetCursor(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture and verify request body
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Return empty response
		json.NewEncoder(w).Encode(map[string]interface{}{
			"cursor":   "next_page",
			"has_more": false,
			"items":    []interface{}{},
		})
	}))
	defer server.Close()

	config := &Config{
		BaseURL: server.URL,
		Token:   "test",
	}
	fetcher := NewFetcher(config)

	from := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC)

	_, err := fetcher.FetchLogs(context.Background(), from, to)
	require.NoError(t, err)

	// Verify reset_cursor was sent correctly
	resetCursor, ok := capturedBody["reset_cursor"].(map[string]interface{})
	require.True(t, ok, "reset_cursor not found in request body")
	assert.Equal(t, "2023-01-01T00:00:00Z", resetCursor["start_time"])
	assert.Equal(t, "2023-01-02T00:00:00Z", resetCursor["end_time"])
	assert.Equal(t, float64(1000), resetCursor["limit"])
}
