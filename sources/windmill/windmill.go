package windmill

import (
	"context"
	"fmt"
	"time"

	"github.com/theplant/audit-log/schema"
	wmill "github.com/windmill-labs/windmill-go-client"
)

// FetchAndUpdateState handles the common Windmill logic for fetching logs and managing state
func FetchAndUpdateState(fetcher schema.LogFetcher) ([]schema.LogEvent, error) {
	from := time.Now().Add(-time.Hour * 1)

	lastSeen, err := wmill.GetState()
	if err != nil {
		return nil, fmt.Errorf("failed to get windmill state: %w", err)
	}
	if lastSeen != "" {
		from, err = time.Parse(time.RFC3339Nano, lastSeen.(string))
		if err != nil {
			return nil, fmt.Errorf("failed to parse last seen %s: %w", lastSeen, err)
		}
	}

	to := time.Now()

	logs, err := fetcher.FetchLogs(context.Background(), from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}

	if len(logs) > 0 {
		wmill.SetState(logs[len(logs)-1].Timestamp.Format(time.RFC3339Nano))
	}

	return logs, nil
}

// import (
//    "github.com/theplant/audit-log/sources/windmill"
//    "github.com/theplant/audit-log/sources/jira"
//
//
// func main() (baseUrl, username, password string) (interface{}, error) {
// 	config := jira.Config{
// 		BaseURL: baseUrl,
// 		Username: username,
// 		Password: password,
// 	}
// 	fetcher := jira.NewFetcher(config)
// 	return windmill.FetchAndUpdateState(fetcher)
// }
