// Command creator-report prints a creator's profile, content analysis and recent LIVE sessions.
//
// Usage: go run ./cmd/creator-report [HANDLE]     e.g. go run ./cmd/creator-report tiktok
// Scopes: creator:read (profile, analysis), live:read (sessions)
package main

import (
	"context"
	"fmt"
	"strings"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"

	"github.com/MKK-Platform/toktokdata-sdks/examples/go/internal/sample"
)

const maxSessions = 30

type profile struct {
	Data struct {
		Creator struct {
			UniqueID        string   `json:"uniqueId"`
			DisplayName     *string  `json:"displayName"`
			Region          *string  `json:"region"`
			Verified        bool     `json:"verified"`
			FollowerCount   *float64 `json:"followerCount"`
			AwemeCount      *float64 `json:"awemeCount"`
			TotalFavorited  *float64 `json:"totalFavorited"`
			LastRefreshedAt *string  `json:"lastRefreshedAt"`
		} `json:"creator"`
	} `json:"data"`
}

type analysis struct {
	Data struct {
		Sample struct {
			VideosAnalyzed int     `json:"videosAnalyzed"`
			SpanDays       float64 `json:"spanDays"`
		} `json:"sample"`
		Engagement *struct {
			AvgPlays       float64 `json:"avgPlays"`
			MedianPlays    float64 `json:"medianPlays"`
			EngagementRate float64 `json:"engagementRate"`
		} `json:"engagement"`
		Cadence *struct {
			PostsPerWeek float64 `json:"postsPerWeek"`
		} `json:"cadence"`
		Limits []struct {
			Message string `json:"message"`
		} `json:"limits"`
	} `json:"data"`
}

type sessionsPage struct {
	Data struct {
		Items []struct {
			StartedAt         string   `json:"startedAt"`
			DurationSeconds   *float64 `json:"durationSeconds"`
			PeakViewerCount   *float64 `json:"peakViewerCount"`
			TotalGiftDiamonds *float64 `json:"totalGiftDiamonds"`
		} `json:"items"`
	} `json:"data"`
}

func num(n *float64) string {
	if n == nil {
		return "-"
	}
	return fmt.Sprintf("%.0f", *n)
}

func str(s *string, def string) string {
	if s == nil || *s == "" {
		return def
	}
	return *s
}

func main() {
	handle := strings.TrimPrefix(sample.Arg(1, "tiktok"), "@")
	ctx := context.Background()

	client, err := toktokdata.NewClient()
	if err != nil {
		sample.Exit(err)
	}

	// A creator seen for the first time answers 202 "resolving"; Get waits and asks again.
	res, err := client.Creators.Get(ctx, handle, nil)
	if err != nil {
		sample.Exit(err)
	}
	var p profile
	if err := res.DecodeBody(&p); err != nil {
		sample.Exit(err)
	}
	c := p.Data.Creator
	verified := ""
	if c.Verified {
		verified = " [verified]"
	}
	fmt.Printf("%s (@%s)%s\n", str(c.DisplayName, c.UniqueID), c.UniqueID, verified)
	fmt.Printf("  region %s, %s followers, %s videos, %s likes\n", str(c.Region, "?"), num(c.FollowerCount), num(c.AwemeCount), num(c.TotalFavorited))
	fmt.Printf("  last refreshed %s\n", str(c.LastRefreshedAt, "-"))

	res, err = client.Creators.Analysis(ctx, handle)
	if err != nil {
		sample.Exit(err)
	}
	var a analysis
	if err := res.DecodeBody(&a); err != nil {
		sample.Exit(err)
	}
	fmt.Printf("\nContent (%d videos over %.0f days)\n", a.Data.Sample.VideosAnalyzed, a.Data.Sample.SpanDays)
	if e := a.Data.Engagement; e != nil {
		fmt.Printf("  plays avg %.0f, median %.0f, engagement %.1f%%\n", e.AvgPlays, e.MedianPlays, e.EngagementRate*100)
	}
	if a.Data.Cadence != nil {
		fmt.Printf("  %.0f posts per week\n", a.Data.Cadence.PostsPerWeek)
	}
	for _, l := range a.Data.Limits {
		fmt.Printf("  note: %s\n", l.Message)
	}

	fmt.Printf("\nRecent LIVE sessions (up to %d)\n", maxSessions)
	shown := 0
	// The iterator follows data.nextCursor page by page until it is null.
	it := client.Live.SessionsIter(ctx, &toktokdata.LiveSessionsParams{CreatorUniqueID: c.UniqueID, Limit: 20})
pages:
	for it.Next() {
		var page sessionsPage
		if err := it.Page().DecodeBody(&page); err != nil {
			sample.Exit(err)
		}
		for _, s := range page.Data.Items {
			minutes := "-"
			if s.DurationSeconds != nil {
				minutes = fmt.Sprintf("%.0f", *s.DurationSeconds/60)
			}
			fmt.Printf("  %s  %4s min  peak %7s viewers  %9s diamonds\n", s.StartedAt, minutes, num(s.PeakViewerCount), num(s.TotalGiftDiamonds))
			shown++
			if shown >= maxSessions {
				break pages
			}
		}
	}
	if err := it.Err(); err != nil {
		sample.Exit(err)
	}
	if shown == 0 {
		fmt.Println("  no recorded sessions")
	}
}
