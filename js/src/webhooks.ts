import { Buffer } from "node:buffer";
import { createHmac, timingSafeEqual } from "node:crypto";
import { TokTokDataError } from "./errors.js";

export const SIGNATURE_HEADER = "x-toktokdata-signature";
export const TIMESTAMP_HEADER = "x-toktokdata-timestamp";

/** A Fetch `Headers` object, or Node's `IncomingHttpHeaders`-style record. */
export type WebhookHeaders =
  | { get(name: string): string | null }
  | Record<string, string | string[] | undefined>;

export interface VerifyWebhookOptions {
  /** Reject timestamps further than this from now, in seconds (default 300). */
  toleranceSeconds?: number;
  /** Current time in unix seconds (for tests). */
  now?: number;
}

export type WebhookVerificationCode =
  | "webhook_missing_header"
  | "webhook_invalid_timestamp"
  | "webhook_timestamp_expired"
  | "webhook_invalid_signature"
  | "webhook_invalid_json";

/** A webhook that failed verification. `status` is 0 (no HTTP call was made), `code` says why. */
export class WebhookVerificationError extends TokTokDataError {
  declare readonly code: WebhookVerificationCode;

  constructor(code: WebhookVerificationCode, message: string) {
    super({ status: 0, code, message, retryable: false });
    this.name = "WebhookVerificationError";
  }
}

/** The alert webhook body: `{schemaVersion, alertId, rule, event, occurredAt, deliveredAt}`. */
export interface WebhookPayload {
  schemaVersion?: number | string;
  alertId?: string;
  rule?: Record<string, unknown>;
  event?: Record<string, unknown>;
  occurredAt?: string;
  deliveredAt?: string;
  [key: string]: unknown;
}

function header(headers: WebhookHeaders, name: string): string | undefined {
  if (typeof (headers as { get?: unknown }).get === "function") {
    return (headers as { get(name: string): string | null }).get(name) ?? undefined;
  }
  for (const [key, value] of Object.entries(headers as Record<string, string | string[] | undefined>)) {
    if (key.toLowerCase() !== name) continue;
    return Array.isArray(value) ? value[0] : value;
  }
  return undefined;
}

/** `sha256=<hex HMAC-SHA256(secret, "<timestamp>.<raw body>")>`, as the API signs it. */
export function signWebhook(secret: string, timestamp: number | string, rawBody: string | Uint8Array): string {
  const hmac = createHmac("sha256", secret);
  hmac.update(`${timestamp}.`);
  hmac.update(rawBody);
  return `sha256=${hmac.digest("hex")}`;
}

/**
 * Verifies an alert webhook and returns its parsed payload.
 *
 * Pass the **raw** request body (before any JSON parsing), the request headers and the rule
 * channel's signing secret. Throws {@link WebhookVerificationError} when the signature is missing or
 * wrong, or the timestamp is more than `toleranceSeconds` (300) away from now.
 */
export function verifyWebhook<T = WebhookPayload>(
  rawBody: string | Uint8Array,
  headers: WebhookHeaders,
  secret: string,
  options: VerifyWebhookOptions = {},
): T {
  const signature = header(headers, SIGNATURE_HEADER)?.trim();
  if (!signature) throw new WebhookVerificationError("webhook_missing_header", `Missing ${SIGNATURE_HEADER} header.`);
  const timestampRaw = header(headers, TIMESTAMP_HEADER)?.trim();
  if (!timestampRaw) throw new WebhookVerificationError("webhook_missing_header", `Missing ${TIMESTAMP_HEADER} header.`);
  if (!/^\d+$/.test(timestampRaw)) {
    throw new WebhookVerificationError("webhook_invalid_timestamp", `${TIMESTAMP_HEADER} is not unix seconds.`);
  }
  const timestamp = Number(timestampRaw);
  const tolerance = options.toleranceSeconds ?? 300;
  const now = options.now ?? Math.floor(Date.now() / 1000);
  if (Math.abs(now - timestamp) > tolerance) {
    throw new WebhookVerificationError(
      "webhook_timestamp_expired",
      `Webhook timestamp is ${now - timestamp} s away from now (tolerance ${tolerance} s).`,
    );
  }

  const expected = Buffer.from(signWebhook(secret, timestampRaw, rawBody), "utf8");
  const candidates = signature.split(",").map((part) => Buffer.from(part.trim(), "utf8"));
  const matches = candidates.some((candidate) => candidate.length === expected.length && timingSafeEqual(candidate, expected));
  if (!matches) throw new WebhookVerificationError("webhook_invalid_signature", "Webhook signature does not match.");

  const text = typeof rawBody === "string" ? rawBody : new TextDecoder().decode(rawBody);
  try {
    return JSON.parse(text) as T;
  } catch {
    throw new WebhookVerificationError("webhook_invalid_json", "Webhook body is not JSON.");
  }
}
