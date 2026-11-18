// Command export-csv exports an official board as CSV and saves it to a file.
//
// Usage: go run ./cmd/export-csv [REGION] [BOARD] [OUT_FILE]     e.g. go run ./cmd/export-csv VN daily
// Scope: export
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"

	"github.com/MKK-Platform/toktokdata-sdks/examples/go/internal/sample"
)

type exportJob struct {
	ID       string  `json:"id"`
	Status   string  `json:"status"`
	RowCount *int    `json:"rowCount"`
	Filename *string `json:"filename"`
	Error    *string `json:"error"`
}

func main() {
	region := sample.Arg(1, "VN")
	board := sample.Arg(2, "daily")
	// The context bounds the whole run, including waiting for the export.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	client, err := toktokdata.NewClient()
	if err != nil {
		sample.Exit(err)
	}

	// 201 when the file is ready at once, 202 while it is being built.
	res, err := client.Exports.Create(ctx, &toktokdata.ExportCreateParams{
		Dataset: "rankings",
		Board:   board,
		Region:  region,
		Format:  "csv",
	})
	if err != nil {
		sample.Exit(err)
	}
	var job exportJob
	if err := res.DecodeBody(&job); err != nil {
		sample.Exit(err)
	}
	fmt.Printf("export %s: %s\n", job.ID, job.Status)

	if job.Status != "ready" && job.Status != "failed" {
		ready, err := client.Exports.WaitUntilReady(ctx, job.ID)
		if err != nil {
			sample.Exit(err)
		}
		if err := ready.DecodeBody(&job); err != nil {
			sample.Exit(err)
		}
	}
	if job.Status == "failed" {
		msg := "unknown error"
		if job.Error != nil {
			msg = *job.Error
		}
		fmt.Fprintf(os.Stderr, "export failed: %s\n", msg)
		os.Exit(1)
	}

	file, err := client.Exports.Download(ctx, job.ID)
	if err != nil {
		sample.Exit(err)
	}
	out := fmt.Sprintf("toktokdata-%s-%s.csv", board, region)
	if job.Filename != nil {
		out = *job.Filename
	}
	out = sample.Arg(3, out)
	if err := os.WriteFile(out, file.Body, 0o644); err != nil {
		sample.Exit(err)
	}
	rows := "?"
	if job.RowCount != nil {
		rows = fmt.Sprint(*job.RowCount)
	}
	fmt.Printf("saved %s rows to %s\n", rows, out)
}
