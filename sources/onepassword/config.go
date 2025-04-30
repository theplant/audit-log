package onepassword

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds the 1Password source configuration
type Config struct {
	BaseURL     string        // Events API URL
	Token       string        // Bearer token for authentication
	RateLimit   int           // API rate limit (requests per minute)
	FetchWindow time.Duration // How far back to look for events
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() *Config {
	rateLimit, _ := strconv.Atoi(os.Getenv("ONEPASSWORD_RATE_LIMIT"))
	if rateLimit == 0 {
		rateLimit = 100 // Default rate limit
	}

	fetchWindow, _ := time.ParseDuration(os.Getenv("ONEPASSWORD_FETCH_WINDOW"))
	if fetchWindow == 0 {
		fetchWindow = 24 * time.Hour // Default to 24 hours
	}

	baseURL := os.Getenv("ONEPASSWORD_BASE_URL")
	if baseURL == "" {
		baseURL = "https://events.1password.com"
	}

	return &Config{
		BaseURL:     baseURL,
		Token:       os.Getenv("ONEPASSWORD_TOKEN"),
		RateLimit:   rateLimit,
		FetchWindow: fetchWindow,
	}
}

// Validate checks if the config has all required fields
func (c *Config) Validate() error {
	if c.Token == "" {
		return fmt.Errorf("1password token is required")
	}
	return nil
}
