package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// UserInfo represents Jira user information
type UserInfo struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

// UserCache handles caching of user information
type UserCache struct {
	mu    sync.RWMutex
	users map[string]UserInfo
	ttl   time.Duration
}

// NewUserCache creates a new UserCache instance
func NewUserCache(ttl time.Duration) *UserCache {
	return &UserCache{
		users: make(map[string]UserInfo),
		ttl:   ttl,
	}
}

// Get retrieves user information from cache
func (c *UserCache) Get(userKey string) (UserInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	user, exists := c.users[userKey]
	return user, exists
}

// Set stores user information in cache
func (c *UserCache) Set(userKey string, user UserInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.users[userKey] = user
}

// UserEnricher handles fetching and enriching user information
type UserEnricher struct {
	config     *JiraConfig
	httpClient *http.Client
	cache      *UserCache
}

// NewUserEnricher creates a new UserEnricher instance
func NewUserEnricher(config *JiraConfig) *UserEnricher {
	return &UserEnricher{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		cache: NewUserCache(24 * time.Hour), // Cache users for 24 hours
	}
}

// EnrichUser fetches and enriches user information
func (e *UserEnricher) EnrichUser(ctx context.Context, userKey string) (UserInfo, error) {
	// Check cache first
	if user, exists := e.cache.Get(userKey); exists {
		return user, nil
	}

	// Fetch from Jira API
	url := fmt.Sprintf("%s/rest/api/3/user?accountId=%s", e.config.BaseURL, userKey)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return UserInfo{}, err
	}

	req.SetBasicAuth(e.config.Username, e.config.Password)

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return UserInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return UserInfo{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var user UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return UserInfo{}, err
	}

	// Cache the result
	e.cache.Set(userKey, user)

	return user, nil
}
