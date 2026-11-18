// Command webhook-server receives toktokdata.live alert webhooks and verifies their signature.
//
// Usage: TOKTOKDATA_WEBHOOK_SECRET=... go run ./cmd/webhook-server [PORT]
// No API key needed here; creating the alert rule that points at this server needs webhook:manage.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"

	"github.com/MKK-Platform/toktokdata-sdks/examples/go/internal/sample"
)

type alertPayload struct {
	SchemaVersion string `json:"schemaVersion"`
	AlertID       string `json:"alertId"`
	Rule          struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"rule"`
	Event       json.RawMessage `json:"event"`
	OccurredAt  *string         `json:"occurredAt"`
	DeliveredAt string          `json:"deliveredAt"`
}

func main() {
	secret := os.Getenv("TOKTOKDATA_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel.")
	}
	port := sample.Arg(1, "8080")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// Verify the exact bytes that were signed: read the raw body, do not re-encode JSON.
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		// Checks x-toktokdata-signature (HMAC-SHA256 of "<timestamp>.<body>") and a 300 s timestamp tolerance.
		verified, err := toktokdata.VerifyWebhook(body, r.Header, secret, 0) // 0 = default 300 s tolerance
		if err != nil {
			log.Printf("rejected webhook: %v", err)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		var p alertPayload
		if err := sample.Decode(verified, &p); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		when := p.DeliveredAt
		if p.OccurredAt != nil {
			when = *p.OccurredAt
		}
		fmt.Printf("alert %s from rule %q at %s\n%s\n", p.AlertID, p.Rule.Name, when, p.Event)
		// Answer 2xx quickly and do slow work after responding, so the delivery does not time out.
		w.WriteHeader(http.StatusNoContent)
	})

	fmt.Printf("listening on http://localhost:%s/ (POST only)\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
