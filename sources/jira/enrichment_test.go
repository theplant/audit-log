package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserEnrichment(t *testing.T) {
	// Create a test server for user information
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		username, password, ok := r.BasicAuth()
		if !ok || username != "testuser" || password != "testpass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Return mock user response
		user := UserInfo{
			AccountID:   "user1",
			DisplayName: "Test User",
			Email:       "test@example.com",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	}))
	defer server.Close()

	// Create enricher with test configuration
	config := &JiraConfig{
		BaseURL:  server.URL,
		Username: "testuser",
		Password: "testpass",
	}

	enricher := NewUserEnricher(config)

	// Test user enrichment
	user, err := enricher.EnrichUser(context.Background(), "user1")
	require.NoError(t, err)
	assert.Equal(t, "Test User", user.DisplayName)
	assert.Equal(t, "test@example.com", user.Email)

	// Test caching
	// Change the server response to verify caching
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return different user information
		user := UserInfo{
			AccountID:   "user1",
			DisplayName: "Different User",
			Email:       "different@example.com",
		}
		json.NewEncoder(w).Encode(user)
	})

	// Should get cached result
	cachedUser, err := enricher.EnrichUser(context.Background(), "user1")
	require.NoError(t, err)
	assert.Equal(t, "Test User", cachedUser.DisplayName) // Should still be the original value
}

func TestUserCache(t *testing.T) {
	cache := NewUserCache(50 * time.Millisecond)

	// Test setting and getting
	user := UserInfo{
		AccountID:   "user1",
		DisplayName: "Test User",
		Email:       "test@example.com",
	}

	cache.Set("user1", user)
	retrieved, exists := cache.Get("user1")
	assert.True(t, exists)
	assert.Equal(t, user, retrieved)

	// Test cache expiration
	time.Sleep(100 * time.Millisecond) // Double the TTL to ensure expiration
	_, exists = cache.Get("user1")
	assert.False(t, exists)
}
