package toktokdata_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"
)

// These examples call the real API, so they have no Output comment: go test
// compiles them but does not run them.

func ExampleNewClient() {
	// The key comes from TOKTOKDATA_API_KEY unless WithAPIKey is given.
	client, err := toktokdata.NewClient(
		toktokdata.WithTimeout(10*time.Second),
		toktokdata.WithMaxRetries(3),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleRankingsService_Official() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Rankings.Official(context.Background(), "hourly", &toktokdata.RankingsOfficialParams{
		Region:   "VN",
		Limit:    10,
		LiveOnly: toktokdata.Bool(true),
	})
	if err != nil {
		log.Fatal(err)
	}
	var data struct {
		Board struct {
			Entries []struct {
				Rank     int    `json:"rank"`
				UniqueID string `json:"uniqueId"`
			} `json:"entries"`
		} `json:"board"`
	}
	if err := resp.Decode(&data); err != nil {
		log.Fatal(err)
	}
	for _, e := range data.Board.Entries {
		fmt.Println(e.Rank, e.UniqueID)
	}
	fmt.Println("source:", resp.Provenance["sourceKind"])
}

func ExampleCreatorsService_Get() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	// Waits through 202 "resolving" answers (up to 180 s by default).
	resp, err := client.Creators.Get(context.Background(), "linh.live", nil)
	if errors.Is(err, toktokdata.ErrPendingTimeout) {
		log.Fatal("the profile is still being collected; try again later")
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(resp.Data))

	// Or get the 202 body back at once.
	resp, err = client.Creators.Get(context.Background(), "linh.live", &toktokdata.CreatorGetParams{Wait: toktokdata.Bool(false)})
	if err != nil {
		log.Fatal(err)
	}
	if info := resp.PendingInfo(); info != nil {
		fmt.Println("still", info.Status, "- ask again in", info.RetryAfterSeconds, "s")
	}
}

func ExampleLiveService_SessionsIter() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	it := client.Live.SessionsIter(context.Background(), &toktokdata.LiveSessionsParams{
		CreatorUniqueID: "linh.live",
		Limit:           50,
	})
	for it.Next() {
		page := it.Page()
		fmt.Println(len(page.Items()), "sessions on this page")
	}
	if err := it.Err(); err != nil {
		log.Fatal(err)
	}
}

func ExampleExportsService_WaitUntilReady() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	created, err := client.Exports.Create(ctx, &toktokdata.ExportCreateParams{
		Dataset:   "rankings",
		Board:     "daily",
		Region:    "VN",
		Format:    "csv",
		RangeDays: 7,
	})
	if err != nil {
		log.Fatal(err)
	}
	var job toktokdata.ExportJob
	if err := created.Decode(&job); err != nil {
		log.Fatal(err)
	}
	ready, err := client.Exports.WaitUntilReady(ctx, job.ID, toktokdata.WaitTimeout(2*time.Minute))
	if err != nil {
		log.Fatal(err)
	}
	if err := ready.Decode(&job); err != nil {
		log.Fatal(err)
	}
	if job.Status == "failed" {
		log.Fatal("export failed: ", job.Error)
	}
	file, err := client.Exports.Download(ctx, job.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(file.Body), "bytes of CSV")
}

func ExampleAlertsService_CreateRule() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Alerts.CreateRule(context.Background(), &toktokdata.AlertRuleParams{
		Name:            "Big gifts on linh.live",
		CreatorUniqueID: toktokdata.String("linh.live"),
		EventTypes:      []string{"gift"},
		MinDiamondCount: toktokdata.Int(1000),
		Channels: []toktokdata.AlertChannel{
			{ChannelType: "webhook", Target: "https://example.com/hooks/toktokdata"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.StatusCode, string(resp.Body))
}

func ExampleIsRateLimited() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Gifters.List(context.Background(), &toktokdata.GiftersListParams{Window: "30d", Segment: "whale"})
	switch {
	case toktokdata.IsRateLimited(err):
		apiErr, _ := toktokdata.AsAPIError(err)
		fmt.Println("rate limited, retry after", apiErr.RetryAfter)
	case toktokdata.IsPaymentRequired(err):
		fmt.Println("out of credits")
	case toktokdata.IsForbidden(err):
		fmt.Println("the key lacks gifter:read or the plan lacks gifters")
	case err != nil:
		log.Fatal(err)
	}
}

func ExampleClient_LastRateLimit() {
	client, err := toktokdata.NewClient()
	if err != nil {
		log.Fatal(err)
	}
	if _, err := client.Rankings.Regions(context.Background()); err != nil {
		log.Fatal(err)
	}
	if rl := client.LastRateLimit(); rl != nil {
		fmt.Printf("%d of %d requests left this second, %d this minute\n", rl.Remaining, rl.Limit, rl.MinuteRemaining)
	}
}

func ExampleVerifyWebhook() {
	secret := "whsec_..." // shown once when the webhook channel was created
	http.HandleFunc("/hooks/toktokdata", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			http.Error(w, "cannot read body", http.StatusBadRequest)
			return
		}
		payload, err := toktokdata.VerifyWebhook(body, r.Header, secret, 5*time.Minute)
		if err != nil {
			var werr *toktokdata.WebhookError
			if errors.As(err, &werr) {
				log.Println("rejected webhook:", werr.Code) // e.g. webhook_invalid_signature
			}
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
		// Deduplicate on the x-toktokdata-alert-id header (or payload["alertId"]).
		fmt.Println("alert", payload["alertId"])
		w.WriteHeader(http.StatusNoContent)
	})
}
