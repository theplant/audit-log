package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"time"

	"github.com/theplant/audit-log/sources/onepassword"
)

func main() {
	var (
		lookbackHours = flag.Int("lookback", 24, "Number of hours to look back for audit logs")
		prettyPrint   = flag.Bool("pretty", true, "Pretty print the JSON output")
	)
	flag.Parse()

	config := onepassword.NewConfigFromEnv()
	if err := config.Validate(); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	fetcher := onepassword.NewFetcher(config)

	to := time.Now()
	from := to.Add(-time.Duration(*lookbackHours) * time.Hour)

	events, err := fetcher.FetchLogs(context.Background(), from, to)
	if err != nil {
		log.Fatalf("Failed to fetch logs: %v", err)
	}

	encoder := json.NewEncoder(os.Stdout)
	if *prettyPrint {
		encoder.SetIndent("", "  ")
	}
	if err := encoder.Encode(events); err != nil {
		log.Fatalf("Failed to output JSON: %v", err)
	}
}
