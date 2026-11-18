package toktokdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// DefaultExportWaitTimeout is how long Exports.WaitUntilReady polls when no
// WaitTimeout option is given (300 s).
const DefaultExportWaitTimeout = 300 * time.Second

// DefaultExportPollInterval is the pause between two Exports.Get calls in
// Exports.WaitUntilReady when no WaitPollInterval option is given and the
// API sends no Retry-After.
const DefaultExportPollInterval = 2 * time.Second

// ExportsService wraps the /v1/exports endpoints (scope export).
type ExportsService struct {
	c *Client
}

// ExportJob is the typed shape of an export job, as returned by
// Exports.Create, Exports.Get and Exports.WaitUntilReady (decode it with
// Response.Decode). Status is "pending", "ready" or "failed"; Error holds the
// reason of a failure.
type ExportJob struct {
	ID          string `json:"id"`
	Dataset     string `json:"dataset"`
	Format      string `json:"format"`
	Status      string `json:"status"`
	RowCount    int    `json:"rowCount"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"downloadUrl"`
	Error       string `json:"error"`
	CreatedAt   string `json:"createdAt"`
	CompletedAt string `json:"completedAt"`
}

// ExportCreateParams is the body of ExportsService.Create. Empty optional
// fields are left out of the JSON.
type ExportCreateParams struct {
	// Dataset is required: "live_sessions", "rankings", "watchlist",
	// "recruit_pipeline" or "gifters".
	Dataset string `json:"dataset"`

	// Format is required: "csv" or "xlsx" (xlsx needs a plan that includes
	// Excel).
	Format string `json:"format"`

	// Board is the board key for the rankings dataset, as in
	// RankingsService.Official.
	Board string `json:"board,omitempty"`

	// Region is the region for rankings (and the creators for gifters); API
	// default VN.
	Region string `json:"region,omitempty"`

	// LeagueTier filters rankings by league tier, A1 ... D5.
	LeagueTier string `json:"leagueTier,omitempty"`

	// RangeDays is the number of days covered, 1-366 (API default 30).
	RangeDays int `json:"rangeDays,omitempty"`
}

// List lists the workspace's exports (GET /v1/exports).
func (s *ExportsService) List(ctx context.Context) (*Response, error) {
	return s.c.get(ctx, "/v1/exports", nil)
}

// Create starts an export (POST /v1/exports). The API answers 201 when the
// file is ready at once or 202 while it is built; the body (an ExportJob) is
// returned as is. Use WaitUntilReady to wait for a pending job.
func (s *ExportsService) Create(ctx context.Context, params *ExportCreateParams) (*Response, error) {
	if params == nil {
		return nil, errors.New("toktokdata: params with Dataset and Format are required")
	}
	if err := required("Dataset", params.Dataset); err != nil {
		return nil, err
	}
	if err := required("Format", params.Format); err != nil {
		return nil, err
	}
	return s.c.do(ctx, http.MethodPost, "/v1/exports", nil, params)
}

// Get returns one export job (GET /v1/exports/{job_id}).
func (s *ExportsService) Get(ctx context.Context, jobID string) (*Response, error) {
	if err := required("jobID", jobID); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/exports/"+segment(jobID), nil)
}

// Download returns the file of a ready export
// (GET /v1/exports/{job_id}/download). The file is in Response.Body (CSV text
// or XLSX bytes); the Content-Disposition header carries the filename. A job
// that is not ready yet fails with a 409 *APIError (code export_not_ready).
func (s *ExportsService) Download(ctx context.Context, jobID string) (*Response, error) {
	if err := required("jobID", jobID); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/exports/"+segment(jobID)+"/download", nil)
}

// WaitOption configures ExportsService.WaitUntilReady.
type WaitOption func(*waitConfig)

type waitConfig struct {
	timeout  time.Duration
	interval time.Duration
}

// WaitTimeout sets how long WaitUntilReady polls before it returns an error
// matching ErrPendingTimeout (default 300 s).
func WaitTimeout(d time.Duration) WaitOption {
	return func(c *waitConfig) {
		c.timeout = d
	}
}

// WaitPollInterval sets the pause between two polls (default 2 s). A longer
// Retry-After sent by the API wins.
func WaitPollInterval(d time.Duration) WaitOption {
	return func(c *waitConfig) {
		c.interval = d
	}
}

// WaitUntilReady polls Get every 2 s (or the API's longer Retry-After) until
// the job's status is "ready" or "failed" and returns that job's response
// without error in both cases: decode it into an ExportJob and check Status
// (Error holds the reason of a failure). It gives up with an error matching
// ErrPendingTimeout after the wait timeout (default 300 s), and stops early
// when ctx is done.
func (s *ExportsService) WaitUntilReady(ctx context.Context, jobID string, opts ...WaitOption) (*Response, error) {
	if err := required("jobID", jobID); err != nil {
		return nil, err
	}
	cfg := waitConfig{timeout: DefaultExportWaitTimeout, interval: DefaultExportPollInterval}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	if cfg.timeout <= 0 {
		cfg.timeout = DefaultExportWaitTimeout
	}
	if cfg.interval <= 0 {
		cfg.interval = DefaultExportPollInterval
	}
	deadline := s.c.now().Add(cfg.timeout)
	for {
		resp, err := s.Get(ctx, jobID)
		if err != nil {
			return nil, err
		}
		var job ExportJob
		if derr := resp.Decode(&job); derr != nil {
			return nil, fmt.Errorf("toktokdata: decode export %s: %w", jobID, derr)
		}
		if job.Status == "ready" || job.Status == "failed" {
			return resp, nil
		}
		delay := cfg.interval
		if ra, ok := parseRetryAfter(resp.Header.Get("Retry-After"), s.c.now()); ok && ra > delay {
			delay = ra
		}
		if delay > maxRetryAfter {
			delay = maxRetryAfter
		}
		if s.c.now().Add(delay).After(deadline) {
			return nil, &PendingTimeoutError{Path: "/v1/exports/" + segment(jobID), Timeout: cfg.timeout, Last: resp}
		}
		if serr := s.c.sleep(ctx, delay); serr != nil {
			return nil, serr
		}
	}
}
