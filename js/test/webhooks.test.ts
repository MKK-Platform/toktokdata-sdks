import { createHmac } from "node:crypto";
import { describe, expect, it } from "vitest";
import { TokTokDataError } from "../src/errors.js";
import { WebhookVerificationError, signWebhook, verifyWebhook } from "../src/webhooks.js";

const secret = "whsec_test";
const body = JSON.stringify({ schemaVersion: 1, alertId: "a1", event: { type: "envelope" } });
const now = 1_793_970_000;

function sign(timestamp: number, raw = body) {
  return `sha256=${createHmac("sha256", secret).update(`${timestamp}.${raw}`).digest("hex")}`;
}

function reason(fn: () => unknown): string | undefined {
  try {
    fn();
  } catch (error) {
    expect(error).toBeInstanceOf(WebhookVerificationError);
    expect(error).toBeInstanceOf(TokTokDataError);
    return (error as WebhookVerificationError).code;
  }
  return undefined;
}

describe("verifyWebhook", () => {
  it("accepts a valid signature (Fetch Headers) and returns the payload", () => {
    const headers = new Headers({ "x-toktokdata-signature": sign(now), "x-toktokdata-timestamp": String(now) });
    const payload = verifyWebhook(body, headers, secret, { now });
    expect(payload.alertId).toBe("a1");
  });

  it("accepts Node-style header records in any case and a Uint8Array body", () => {
    const headers = { "X-TokTokData-Signature": sign(now), "X-TokTokData-Timestamp": [String(now)] };
    const payload = verifyWebhook(new TextEncoder().encode(body), headers, secret, { now: now + 299 });
    expect(payload.event).toEqual({ type: "envelope" });
  });

  it("matches signWebhook", () => {
    expect(signWebhook(secret, now, body)).toBe(sign(now));
  });

  it("rejects a wrong signature or a tampered body", () => {
    const headers = { "x-toktokdata-signature": sign(now), "x-toktokdata-timestamp": String(now) };
    expect(reason(() => verifyWebhook(body + " ", headers, secret, { now }))).toBe("webhook_invalid_signature");
    expect(reason(() => verifyWebhook(body, headers, "other", { now }))).toBe("webhook_invalid_signature");
    expect(
      reason(() => verifyWebhook(body, { ...headers, "x-toktokdata-signature": "sha256=abc" }, secret, { now })),
    ).toBe("webhook_invalid_signature");
  });

  it("rejects an expired or future timestamp", () => {
    const old = now - 301;
    const headers = { "x-toktokdata-signature": sign(old), "x-toktokdata-timestamp": String(old) };
    expect(reason(() => verifyWebhook(body, headers, secret, { now }))).toBe("webhook_timestamp_expired");
    expect(verifyWebhook(body, headers, secret, { now, toleranceSeconds: 600 })).toBeTruthy();
    const future = now + 400;
    expect(
      reason(() =>
        verifyWebhook(body, { "x-toktokdata-signature": sign(future), "x-toktokdata-timestamp": String(future) }, secret, { now }),
      ),
    ).toBe("webhook_timestamp_expired");
  });

  it("rejects missing headers", () => {
    expect(reason(() => verifyWebhook(body, { "x-toktokdata-timestamp": String(now) }, secret, { now }))).toBe(
      "webhook_missing_header",
    );
    expect(reason(() => verifyWebhook(body, { "x-toktokdata-signature": sign(now) }, secret, { now }))).toBe(
      "webhook_missing_header",
    );
  });

  it("rejects a non-numeric timestamp and a signed non-JSON body", () => {
    expect(
      reason(() => verifyWebhook(body, { "x-toktokdata-signature": sign(now), "x-toktokdata-timestamp": "yesterday" }, secret, { now })),
    ).toBe("webhook_invalid_timestamp");
    const raw = "not json";
    expect(
      reason(() =>
        verifyWebhook(raw, { "x-toktokdata-signature": sign(now, raw), "x-toktokdata-timestamp": String(now) }, secret, { now }),
      ),
    ).toBe("webhook_invalid_json");
  });
});
