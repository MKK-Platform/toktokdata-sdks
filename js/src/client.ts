import { TokTokDataError } from "./errors.js";
import {
  DEFAULT_BASE_URL,
  DEFAULT_MAX_RETRIES,
  DEFAULT_PENDING_TIMEOUT_MS,
  DEFAULT_TIMEOUT_MS,
  Transport,
  type FetchLike,
  type HttpMethod,
  type Query,
} from "./http.js";
import { paginate, type PaginateOptions } from "./pagination.js";
import {
  Account,
  Agencies,
  Alerts,
  Creators,
  Exports,
  Gifters,
  Gifts,
  Insights,
  Live,
  Rankings,
  Trends,
  Videos,
} from "./resources.js";
import type { RateLimitInfo } from "./types.js";

export interface TokTokDataOptions {
  /** API key (`ttk_live_<id>_<secret>`). Defaults to the `TOKTOKDATA_API_KEY` environment variable. */
  apiKey?: string;
  /** Defaults to `TOKTOKDATA_BASE_URL`, else `https://api.toktokdata.live`. */
  baseUrl?: string;
  /** Per-request timeout in ms (default 30 000). */
  timeoutMs?: number;
  /** Retries of idempotent requests on network errors, 429 and retryable 5xx (default 2). */
  maxRetries?: number;
  /** How long 202-aware methods wait for data by default, in ms (default 180 000). */
  pendingTimeoutMs?: number;
  /** A `fetch` implementation (default: the global `fetch`). */
  fetch?: FetchLike;
}

function readEnv(name: string): string | undefined {
  try {
    const env = (globalThis as { process?: { env?: Record<string, string | undefined> } }).process?.env;
    const value = env?.[name];
    return value && value.trim() ? value.trim() : undefined;
  } catch {
    return undefined;
  }
}

/**
 * Client of the toktokdata.live API.
 *
 * ```ts
 * const client = new TokTokData(); // reads TOKTOKDATA_API_KEY
 * const board = await client.rankings.official("hourly", { region: "VN" });
 * ```
 *
 * Keep the key on a server: never ship it to a browser.
 */
export class TokTokData {
  readonly rankings: Rankings;
  readonly creators: Creators;
  readonly agencies: Agencies;
  readonly insights: Insights;
  readonly videos: Videos;
  readonly trends: Trends;
  readonly gifts: Gifts;
  readonly gifters: Gifters;
  readonly live: Live;
  readonly alerts: Alerts;
  readonly exports: Exports;
  readonly account: Account;
  readonly #transport: Transport;

  constructor(options: TokTokDataOptions = {}) {
    const apiKey = (options.apiKey ?? readEnv("TOKTOKDATA_API_KEY"))?.trim();
    if (!apiKey) {
      throw new TokTokDataError({
        status: 0,
        code: "missing_api_key",
        message:
          "No toktokdata.live API key: pass { apiKey } or set the TOKTOKDATA_API_KEY environment variable " +
          "(create a key in the dashboard under Settings → API keys).",
        retryable: false,
      });
    }
    const fetchImpl =
      options.fetch ??
      (typeof globalThis.fetch === "function" ? (globalThis.fetch.bind(globalThis) as unknown as FetchLike) : undefined);
    if (!fetchImpl) {
      throw new TypeError("No global fetch is available (Node >= 18 has one); pass { fetch } to TokTokData.");
    }
    this.#transport = new Transport({
      apiKey,
      baseUrl: options.baseUrl ?? readEnv("TOKTOKDATA_BASE_URL") ?? DEFAULT_BASE_URL,
      timeoutMs: options.timeoutMs ?? DEFAULT_TIMEOUT_MS,
      maxRetries: options.maxRetries ?? DEFAULT_MAX_RETRIES,
      pendingTimeoutMs: options.pendingTimeoutMs ?? DEFAULT_PENDING_TIMEOUT_MS,
      fetch: fetchImpl,
    });
    const t = this.#transport;
    this.rankings = new Rankings(t);
    this.creators = new Creators(t);
    this.agencies = new Agencies(t);
    this.insights = new Insights(t);
    this.videos = new Videos(t);
    this.trends = new Trends(t);
    this.gifts = new Gifts(t);
    this.gifters = new Gifters(t);
    this.live = new Live(t);
    this.alerts = new Alerts(t);
    this.exports = new Exports(t);
    this.account = new Account(t);
  }

  /** The API base URL in use. */
  get baseUrl(): string {
    return this.#transport.baseUrl;
  }

  /** `X-RateLimit-*` headers of the last response (`undefined` before the first one). */
  get lastRateLimit(): RateLimitInfo | undefined {
    return this.#transport.lastRateLimit;
  }

  /**
   * Iterates over the pages of any cursor list.
   *
   * ```ts
   * for await (const page of client.paginate((p) => client.live.sessions(p), { limit: 100 })) { ... }
   * ```
   */
  paginate<P extends object, R>(
    fetchPage: (params: P) => Promise<R>,
    params?: P,
    options?: PaginateOptions,
  ): AsyncGenerator<R, void, undefined> {
    return paginate(fetchPage, params, options);
  }

  /** Escape hatch for a route this version does not wrap yet. Same auth, retries and errors. */
  request<T = unknown>(method: HttpMethod, path: string, init: { query?: Query; body?: unknown } = {}): Promise<T> {
    return this.#transport.request<T>(method, path, init);
  }

  /** Never print the key. */
  toJSON(): Record<string, unknown> {
    return { baseUrl: this.baseUrl };
  }
}
