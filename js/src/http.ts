import { PendingTimeoutError, TokTokDataError, type ErrorEnvelope } from "./errors.js";
import type { RateLimitInfo } from "./types.js";
import { VERSION } from "./version.js";

export const DEFAULT_BASE_URL = "https://api.toktokdata.live";
export const DEFAULT_TIMEOUT_MS = 30_000;
export const DEFAULT_MAX_RETRIES = 2;
export const DEFAULT_PENDING_TIMEOUT_MS = 180_000;
const MAX_RETRY_AFTER_SECONDS = 60;

/** The subset of a fetch `Response` the client uses. */
export interface FetchResponseLike {
  status: number;
  headers: { get(name: string): string | null };
  text(): Promise<string>;
  arrayBuffer(): Promise<ArrayBuffer>;
}

/** A `fetch`-compatible function (the global `fetch` fits). */
export type FetchLike = (
  url: string,
  init: { method: string; headers: Record<string, string>; body?: string; signal?: AbortSignal },
) => Promise<FetchResponseLike>;

export type HttpMethod = "GET" | "POST" | "PATCH" | "DELETE";

export type QueryValue = string | number | boolean | null | undefined | ReadonlyArray<string | number>;
export type Query = Record<string, QueryValue>;

export interface TransportOptions {
  apiKey: string;
  baseUrl: string;
  timeoutMs: number;
  maxRetries: number;
  pendingTimeoutMs: number;
  fetch: FetchLike;
}

export interface RawResult {
  status: number;
  headers: FetchResponseLike["headers"];
  body: unknown;
}

/** Per-call options of 202-aware methods. */
export interface WaitOptions<W extends boolean = boolean> {
  /** Wait for the data while the API answers 202 (default `true`). `false` returns the 202 body. */
  wait?: W;
  /** Wait budget in ms (default: the client's `pendingTimeoutMs`, 180 000). */
  maxWaitMs?: number;
}

/**
 * Builds a query string: `undefined`/`null` are omitted, booleans become `true`/`false`, arrays are
 * comma-joined.
 */
export function buildQuery(query: Query = {}): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null) continue;
    if (Array.isArray(value)) {
      if (value.length) params.set(key, value.join(","));
      continue;
    }
    params.set(key, String(value));
  }
  const encoded = params.toString();
  return encoded ? `?${encoded}` : "";
}

/** Encodes one path segment; a leading `@` on a creator handle is dropped. */
export function pathSegment(value: string | number, { stripAt = false } = {}): string {
  let text = String(value).trim();
  if (stripAt && text.startsWith("@")) text = text.slice(1);
  if (!text) throw new TypeError("A path parameter is empty.");
  return encodeURIComponent(text);
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Parses `Retry-After` (seconds or an HTTP date) into seconds. */
export function parseRetryAfter(value: string | null | undefined, now = Date.now()): number | undefined {
  if (value == null || value.trim() === "") return undefined;
  const seconds = Number(value);
  if (Number.isFinite(seconds)) return Math.max(0, seconds);
  const date = Date.parse(value);
  if (Number.isNaN(date)) return undefined;
  return Math.max(0, Math.ceil((date - now) / 1000));
}

function headerNumber(headers: FetchResponseLike["headers"], name: string): number | undefined {
  const raw = headers.get(name);
  if (raw == null || raw === "") return undefined;
  const value = Number(raw);
  return Number.isFinite(value) ? value : undefined;
}

export function parseRateLimit(headers: FetchResponseLike["headers"]): RateLimitInfo {
  const info: RateLimitInfo = {};
  const pairs: Array<[keyof RateLimitInfo, string]> = [
    ["limit", "x-ratelimit-limit"],
    ["remaining", "x-ratelimit-remaining"],
    ["minuteLimit", "x-ratelimit-minute-limit"],
    ["minuteRemaining", "x-ratelimit-minute-remaining"],
    ["dailyLimit", "x-ratelimit-daily-limit"],
    ["dailyRemaining", "x-ratelimit-daily-remaining"],
  ];
  for (const [key, header] of pairs) {
    const value = headerNumber(headers, header);
    if (value !== undefined) info[key] = value;
  }
  return info;
}

function parseJson(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

function isEnvelope(value: unknown): value is ErrorEnvelope {
  return typeof value === "object" && value !== null && !Array.isArray(value) && "code" in value;
}

function backoffMs(attempt: number): number {
  const base = 500 * 2 ** attempt;
  return Math.round(base + Math.random() * base * 0.25);
}

/** Request plumbing: auth, timeout, retries, error mapping, rate-limit capture. */
export class Transport {
  readonly baseUrl: string;
  readonly maxRetries: number;
  readonly timeoutMs: number;
  readonly pendingTimeoutMs: number;
  lastRateLimit: RateLimitInfo | undefined;
  readonly #apiKey: string;
  readonly #fetch: FetchLike;

  constructor(options: TransportOptions) {
    this.#apiKey = options.apiKey;
    this.baseUrl = options.baseUrl.replace(/\/+$/, "");
    this.timeoutMs = options.timeoutMs;
    this.maxRetries = Math.max(0, Math.floor(options.maxRetries));
    this.pendingTimeoutMs = options.pendingTimeoutMs;
    this.#fetch = options.fetch;
  }

  /** Sends a request and returns the parsed JSON body (`undefined` on 204). */
  async request<T = unknown>(
    method: HttpMethod,
    path: string,
    init: { query?: Query; body?: unknown } = {},
  ): Promise<T> {
    const result = await this.send(method, path, init);
    return result.body as T;
  }

  /** GET that waits while the API answers 202 ("still collecting"). */
  async getWaiting<T = unknown>(path: string, query: Query | undefined, options: WaitOptions = {}): Promise<T> {
    const started = Date.now();
    const budget = options.maxWaitMs ?? this.pendingTimeoutMs;
    for (;;) {
      const result = await this.send("GET", path, query ? { query } : {});
      if (result.status !== 202 || options.wait === false) return result.body as T;
      const body = (result.body ?? {}) as Record<string, unknown>;
      const hinted =
        typeof body.retryAfterSeconds === "number"
          ? body.retryAfterSeconds
          : parseRetryAfter(result.headers.get("retry-after"));
      const delaySeconds = Math.min(120, Math.max(1, hinted ?? 1));
      const waited = Date.now() - started;
      if (waited + delaySeconds * 1000 > budget) {
        throw new PendingTimeoutError({ path, body: result.body, retryAfterSeconds: delaySeconds, waitedMs: waited });
      }
      await sleep(delaySeconds * 1000);
    }
  }

  /** GET of a file: the body as text or bytes. */
  async download(path: string, as: "text" | "bytes"): Promise<string | Uint8Array> {
    const result = await this.send("GET", path, { raw: as });
    return result.body as string | Uint8Array;
  }

  async send(
    method: HttpMethod,
    path: string,
    init: { query?: Query; body?: unknown; raw?: "text" | "bytes" } = {},
  ): Promise<RawResult> {
    const url = `${this.baseUrl}${path}${buildQuery(init.query)}`;
    const headers: Record<string, string> = {
      Authorization: `Bearer ${this.#apiKey}`,
      Accept: "application/json",
      "User-Agent": `toktokdata-js/${VERSION}`,
    };
    let body: string | undefined;
    if (init.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(init.body);
    }

    for (let attempt = 0; ; attempt++) {
      const attemptsLeft = attempt < this.maxRetries;
      // Network errors and 5xx are retried for GET only. A 429 comes from the limiter before the
      // handler runs, so it is retried for every method.
      const canRetry = method === "GET" && attemptsLeft;
      const controller = new AbortController();
      const timer = setTimeout(() => controller.abort(), this.timeoutMs);
      let status: number;
      let responseHeaders: FetchResponseLike["headers"];
      let text = "";
      let bytes: Uint8Array | undefined;
      try {
        const response = await this.#fetch(url, {
          method,
          headers,
          signal: controller.signal,
          ...(body !== undefined ? { body } : {}),
        });
        status = response.status;
        responseHeaders = response.headers;
        if (init.raw === "bytes" && status >= 200 && status < 300) {
          bytes = new Uint8Array(await response.arrayBuffer());
        } else {
          text = await response.text();
        }
      } catch (cause) {
        const timedOut = controller.signal.aborted;
        const error = new TokTokDataError({
          status: 0,
          code: timedOut ? "timeout" : "network_error",
          message: timedOut
            ? `${method} ${path} timed out after ${this.timeoutMs} ms.`
            : `${method} ${path} failed: ${cause instanceof Error ? cause.message : String(cause)}`,
          retryable: true,
          cause,
        });
        if (canRetry) {
          await sleep(backoffMs(attempt));
          continue;
        }
        throw error;
      } finally {
        clearTimeout(timer);
      }

      this.lastRateLimit = parseRateLimit(responseHeaders);

      if (status >= 200 && status < 300) {
        if (bytes) return { status, headers: responseHeaders, body: bytes };
        if (init.raw === "text") return { status, headers: responseHeaders, body: text };
        return { status, headers: responseHeaders, body: text ? parseJson(text) ?? text : undefined };
      }

      const error = this.toError(status, responseHeaders, text);
      if (attemptsLeft && this.shouldRetry(error, method)) {
        const wait =
          error.retryAfter !== undefined
            ? Math.min(error.retryAfter, MAX_RETRY_AFTER_SECONDS) * 1000
            : backoffMs(attempt);
        await sleep(wait);
        continue;
      }
      throw error;
    }
  }

  private shouldRetry(error: TokTokDataError, method: HttpMethod): boolean {
    if (error.status === 429) return error.code !== "daily_quota_exceeded";
    if (error.status >= 500 && method === "GET") {
      return error.details === undefined || error.details.retryable === true;
    }
    return false;
  }

  private toError(status: number, headers: FetchResponseLike["headers"], text: string): TokTokDataError {
    const parsed = text ? parseJson(text) : undefined;
    const envelope = isEnvelope(parsed) ? parsed : undefined;
    const retryAfter = parseRetryAfter(headers.get("retry-after"));
    const requestId = envelope?.requestId ?? headers.get("x-request-id") ?? undefined;
    const fallbackRetryable = status === 429 || status >= 500;
    return new TokTokDataError({
      status,
      code: typeof envelope?.code === "string" ? envelope.code : "http_error",
      message:
        typeof envelope?.message === "string"
          ? envelope.message
          : `HTTP ${status}${text ? `: ${text.slice(0, 200)}` : ""}`,
      requestId,
      retryable: typeof envelope?.retryable === "boolean" ? envelope.retryable : fallbackRetryable,
      retryAfter,
      details: envelope,
    });
  }
}
