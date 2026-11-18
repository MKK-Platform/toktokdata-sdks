// Command leaderboard prints the top 10 of an official TikTok LIVE board for one region.
//
// Usage: go run ./cmd/leaderboard [REGION] [BOARD]     e.g. go run ./cmd/leaderboard VN hourly
// Scope: rank:read
package main

import (
	"context"
	"fmt"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"

	"github.com/MKK-Platform/toktokdata-sdks/examples/go/internal/sample"
)

type officialBoard struct {
	Data struct {
		Board struct {
			Board       string `json:"board"`
			Region      string `json:"region"`
			PeriodKey   string `json:"periodKey"`
			Publishable bool   `json:"publishable"`
			Coverage    struct {
				Ratio     float64 `json:"ratio"`
				Threshold float64 `json:"threshold"`
			} `json:"coverage"`
			Entries []struct {
				Rank       int     `json:"rank"`
				UniqueID   *string `json:"uniqueId"`
				Nickname   *string `json:"nickname"`
				Score      float64 `json:"score"`
				IsLive     bool    `json:"isLive"`
				Masked     bool    `json:"masked"`
				UnmaskTier *string `json:"unmaskTier"`
			} `json:"entries"`
		} `json:"board"`
	} `json:"data"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func main() {
	region := sample.Arg(1, "VN")
	boardKey := sample.Arg(2, "hourly")
	ctx := context.Background()

	client, err := toktokdata.NewClient() // reads TOKTOKDATA_API_KEY
	if err != nil {
		sample.Exit(err)
	}
	res, err := client.Rankings.Official(ctx, boardKey, &toktokdata.RankingsOfficialParams{Region: region, Limit: 10})
	if err != nil {
		sample.Exit(err)
	}
	var out officialBoard
	if err := res.DecodeBody(&out); err != nil {
		sample.Exit(err)
	}

	b := out.Data.Board
	fmt.Printf("%s board, %s, period %s\n", b.Board, b.Region, b.PeriodKey)
	if !b.Publishable {
		// Coverage below the gate: the order may still change, so label it as partial.
		fmt.Printf("(partial: coverage %.0f%% of the %.0f%% gate)\n", b.Coverage.Ratio*100, b.Coverage.Threshold*100)
	}
	for _, e := range b.Entries {
		who := fmt.Sprintf("%s @%s", deref(e.Nickname), deref(e.UniqueID))
		if e.Masked {
			who = fmt.Sprintf("(masked, plan %s)", deref(e.UnmaskTier))
		}
		live := ""
		if e.IsLive {
			live = "  LIVE"
		}
		fmt.Printf("%3d  %-40s %12.0f%s\n", e.Rank, who, e.Score, live)
	}
}
