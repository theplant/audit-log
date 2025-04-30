package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/theplant/audit-log/sources/jira"
)

func main() {
	// Parse command line flags
	var (
		lookbackHours = flag.Int("lookback", 24, "Number of hours to look back for audit logs")
		prettyPrint   = flag.Bool("pretty", false, "Pretty print the JSON output")
	)
	flag.Parse()

	// Create config from environment variables
	config := jira.NewConfigFromEnv()
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create fetcher
	fetcher := jira.NewFetcher(config)

	// Calculate time range
	now := time.Now()
	from := now.Add(time.Duration(-*lookbackHours) * time.Hour)

	// Fetch logs
	ctx := context.Background()
	logs, err := fetcher.FetchLogs(ctx, from, now)
	if err != nil {
		log.Fatalf("Failed to fetch logs: %v", err)
	}

	// Create encoder
	encoder := json.NewEncoder(os.Stdout)
	if *prettyPrint {
		encoder.SetIndent("", "  ")
	}

	// Print logs
	if err := encoder.Encode(logs); err != nil {
		log.Fatalf("Failed to encode logs: %v", err)
	}

	// Print summary to stderr
	fmt.Fprintf(os.Stderr, "\nFetched %d audit log records\n", len(logs))
}
