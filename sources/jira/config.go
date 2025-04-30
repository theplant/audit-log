package jira

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents the configuration for Jira audit log fetching
type Config struct {
	BaseURL     string
	Username    string
	Password    string
	RateLimit   int
	FetchWindow time.Duration
}

// NewConfigFromEnv creates a new Config from environment variables
func NewConfigFromEnv() *Config {
	return &Config{
		BaseURL:     os.Getenv("JIRA_BASE_URL"),
		Username:    os.Getenv("JIRA_USERNAME"),
		Password:    os.Getenv("JIRA_PASSWORD"),
		RateLimit:   getEnvAsInt("JIRA_RATE_LIMIT", 100),
		FetchWindow: getEnvAsDuration("JIRA_FETCH_WINDOW", 24*time.Hour),
	}
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
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

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
