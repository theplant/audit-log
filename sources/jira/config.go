package jira

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// JiraConfig holds the Jira source configuration
type JiraConfig struct {
	BaseURL     string
	Username    string
	Password    string
	RateLimit   int
	FetchWindow time.Duration
}

// NewConfigFromEnv creates a new JiraConfig from environment variables
func NewConfigFromEnv() *JiraConfig {
	return &JiraConfig{
		BaseURL:     getEnvOrDefault("JIRA_BASE_URL", ""),
		Username:    getEnvOrDefault("JIRA_USERNAME", ""),
		Password:    getEnvOrDefault("JIRA_PASSWORD", ""),
		RateLimit:   getEnvIntOrDefault("JIRA_RATE_LIMIT", 100),
		FetchWindow: getEnvDurationOrDefault("JIRA_FETCH_WINDOW", 24*time.Hour),
	}
}

// Validate checks if the configuration is valid
func (c *JiraConfig) Validate() error {
	if c.BaseURL == "" {
		return fmt.Errorf("JIRA_BASE_URL is required")
	}
	if c.Username == "" {
		return fmt.Errorf("JIRA_USERNAME is required")
	}
	if c.Password == "" {
		return fmt.Errorf("JIRA_PASSWORD is required")
	}
	return nil
}

// Helper functions for environment variables
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
