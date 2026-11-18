package toktokdata

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

// AlertsService wraps the /v1/alerts endpoints (scope webhook:manage).
type AlertsService struct {
	c *Client
}

// AlertRuleParams is a typed body for AlertsService.CreateRule and
// AlertsService.UpdateRule. Nil and empty fields are left out of the JSON, so
// an update only changes the fields you set. To clear a nullable field
// (creatorUniqueId, region, minDiamondCount, minFollowerMilestone) send a
// map[string]any with a nil value instead.
type AlertRuleParams struct {
	// Name is shown in notifications. Required when creating a rule.
	Name string `json:"name,omitempty"`

	// CreatorUniqueID watches one creator (handle without @).
	CreatorUniqueID *string `json:"creatorUniqueId,omitempty"`

	// Scope is "creator", "any", "watchlist" or "gifter_watchlist". The API
	// defaults to "creator" when CreatorUniqueID is set, else "any".
	Scope string `json:"scope,omitempty"`

	// Region limits an "any" rule to one region.
	Region *string `json:"region,omitempty"`

	// EventTypes lists the events: "gift", "envelope", "goodyBag",
	// "live_start", "live_end", "host_quiet", "league_change",
	// "diamond_target", "follower_milestone" or "*".
	EventTypes []string `json:"eventTypes,omitempty"`

	// MinDiamondCount ignores events worth fewer diamonds.
	MinDiamondCount *int `json:"minDiamondCount,omitempty"`

	// MinFollowerMilestone is the follower step of follower_milestone.
	MinFollowerMilestone *int `json:"minFollowerMilestone,omitempty"`

	// Enabled switches the rule on or off (API default true).
	Enabled *bool `json:"enabled,omitempty"`

	// Channels are the notification channels.
	Channels []AlertChannel `json:"channels,omitempty"`
}

// AlertChannel is one notification channel of an alert rule.
type AlertChannel struct {
	// ChannelType is "webhook", "email", "in_app", "telegram", "discord" or
	// "zalo". Required.
	ChannelType string `json:"channelType"`

	// Target is the webhook URL, email address, Discord webhook URL or Zalo
	// user id.
	Target string `json:"target,omitempty"`

	// Enabled switches the channel on or off (API default true).
	Enabled *bool `json:"enabled,omitempty"`
}

// ListRules lists the workspace's alert rules (GET /v1/alerts/rules).
func (s *AlertsService) ListRules(ctx context.Context) (*Response, error) {
	return s.c.get(ctx, "/v1/alerts/rules", nil)
}

// CreateRule creates an alert rule (POST /v1/alerts/rules, 201). body is an
// *AlertRuleParams or any value that encodes to the JSON body.
func (s *AlertsService) CreateRule(ctx context.Context, body any) (*Response, error) {
	if body == nil {
		return nil, errors.New("toktokdata: body is required")
	}
	return s.c.do(ctx, http.MethodPost, "/v1/alerts/rules", nil, body)
}

// UpdateRule changes an alert rule (PATCH /v1/alerts/rules/{rule_id}). body
// is an *AlertRuleParams or any value that encodes to the JSON body.
func (s *AlertsService) UpdateRule(ctx context.Context, ruleID string, body any) (*Response, error) {
	if err := required("ruleID", ruleID); err != nil {
		return nil, err
	}
	if body == nil {
		return nil, errors.New("toktokdata: body is required")
	}
	return s.c.do(ctx, http.MethodPatch, "/v1/alerts/rules/"+segment(ruleID), nil, body)
}

// DeleteRule deletes an alert rule (DELETE /v1/alerts/rules/{rule_id}, 204).
func (s *AlertsService) DeleteRule(ctx context.Context, ruleID string) error {
	if err := required("ruleID", ruleID); err != nil {
		return err
	}
	_, err := s.c.do(ctx, http.MethodDelete, "/v1/alerts/rules/"+segment(ruleID), nil, nil)
	return err
}

// TestRule sends a test notification through a rule's channels
// (POST /v1/alerts/rules/{rule_id}/test).
func (s *AlertsService) TestRule(ctx context.Context, ruleID string) (*Response, error) {
	if err := required("ruleID", ruleID); err != nil {
		return nil, err
	}
	return s.c.do(ctx, http.MethodPost, "/v1/alerts/rules/"+segment(ruleID)+"/test", nil, nil)
}

// AlertFiresParams are the optional parameters of AlertsService.Fires. Zero
// values are omitted.
type AlertFiresParams struct {
	// RuleID keeps one rule (UUID).
	RuleID string

	// Since is an ISO 8601 time; only newer rows are returned.
	Since string

	// Limit is the number of rows, 1-200 (API default 50).
	Limit int
}

// Fires lists alert fires (GET /v1/alerts/fires).
func (s *AlertsService) Fires(ctx context.Context, params *AlertFiresParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "ruleId", params.RuleID)
		setString(q, "since", params.Since)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/alerts/fires", q)
}

// AlertDeliveriesParams are the optional parameters of
// AlertsService.Deliveries. Zero values are omitted.
type AlertDeliveriesParams struct {
	// RuleID keeps one rule (UUID).
	RuleID string

	// Status keeps one outcome: "delivered", "failed" or "capped".
	Status string

	// Since is an ISO 8601 time; only newer rows are returned.
	Since string

	// Limit is the number of rows, 1-200 (API default 50).
	Limit int
}

// Deliveries lists alert deliveries (GET /v1/alerts/deliveries).
func (s *AlertsService) Deliveries(ctx context.Context, params *AlertDeliveriesParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "ruleId", params.RuleID)
		setString(q, "status", params.Status)
		setString(q, "since", params.Since)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/alerts/deliveries", q)
}

// Stats returns alert activity in the last 24 hours (GET /v1/alerts/stats).
func (s *AlertsService) Stats(ctx context.Context) (*Response, error) {
	return s.c.get(ctx, "/v1/alerts/stats", nil)
}
