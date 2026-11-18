import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PendingTimeoutError, TokTokData, TokTokDataError, VERSION, isPending } from "../src/index.js";
import { KEY, client, json, mockFetch, ok } from "./helpers.js";

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

describe("construction", () => {
  it("fails clearly without a key", () => {
    vi.stubEnv("TOKTOKDATA_API_KEY", "");
    expect(() => new TokTokData({ fetch: mockFetch(ok()).fetch })).toThrowError(/TOKTOKDATA_API_KEY/);
    try {
      new TokTokData({ fetch: mockFetch(ok()).fetch });
    } catch (error) {
      expect(error).toBeInstanceOf(TokTokDataError);
      expect((error as TokTokDataError).code).toBe("missing_api_key");
    }
  });

  it("reads the key and base URL from the environment", async () => {
    vi.stubEnv("TOKTOKDATA_API_KEY", "ttk_live_env_key");
    vi.stubEnv("TOKTOKDATA_BASE_URL", "https://staging.example/");
    const { fetch, calls } = mockFetch(ok());
    const c = new TokTokData({ fetch });
    expect(c.baseUrl).toBe("https://staging.example");
    await c.rankings.regions();
    expect(calls[0]!.url.href).toBe("https://staging.example/v1/rankings/regions");
    expect(calls[0]!.headers.Authorization).toBe("Bearer ttk_live_env_key");
  });

  it("never serialises the key", () => {
    const c = client(mockFetch(ok()).fetch);
    expect(JSON.stringify(c)).not.toContain(KEY);
  });
});

describe("requests", () => {
  it("sends auth, accept and user-agent headers", async () => {
    const { fetch, calls } = mockFetch(ok());
    await client(fetch).rankings.regions();
    const call = calls[0]!;
    expect(call.method).toBe("GET");
    expect(call.headers.Authorization).toBe(`Bearer ${KEY}`);
    expect(call.headers.Accept).toBe("application/json");
    expect(call.headers["User-Agent"]).toBe(`toktokdata-js/${VERSION}`);
    expect(call.headers["Content-Type"]).toBeUndefined();
    expect(call.body).toBeUndefined();
  });

  it("encodes the query: omits undefined/null, booleans as true/false", async () => {
    const { fetch, calls } = mockFetch(ok());
    await client(fetch).rankings.official("hourly", {
      region: "VN",
      liveOnly: false,
      leagueTier: undefined,
      limit: 10,
    });
    const url = calls[0]!.url;
    expect(url.pathname).toBe("/v1/rankings/official");
    expect(url.search).toBe("?board=hourly&region=VN&liveOnly=false&limit=10");
  });

  it("encodes true booleans, arrays and special characters", async () => {
    const { fetch, calls } = mockFetch(ok());
    const c = client(fetch);
    await c.agencies.list({ q: "a&b c", publicOnly: true });
    await c.live.compare(["s1", "s2", "s3"]);
    expect(calls[0]!.url.searchParams.get("q")).toBe("a&b c");
    expect(calls[0]!.url.searchParams.get("publicOnly")).toBe("true");
    expect(calls[1]!.url.searchParams.get("ids")).toBe("s1,s2,s3");
  });

  it("encodes path params and drops a leading @ on handles", async () => {
    const { fetch, calls } = mockFetch(ok());
    const c = client(fetch);
    await c.creators.analysis("@linh.live");
    await c.videos.get("74/00?x");
    expect(calls[0]!.url.pathname).toBe("/v1/creators/linh.live/analysis");
    expect(calls[1]!.url.pathname).toBe("/v1/videos/74%2F00%3Fx");
  });

  it("sends JSON bodies with a content type", async () => {
    const { fetch, calls } = mockFetch(json(201, { rule: { id: "r1" }, webhookSecrets: [] }));
    const result = await client(fetch).alerts.createRule({ name: "Envelopes", eventTypes: ["envelope"] });
    expect(result.rule.id).toBe("r1");
    expect(calls[0]!.method).toBe("POST");
    expect(calls[0]!.headers["Content-Type"]).toBe("application/json");
    expect(JSON.parse(calls[0]!.body!)).toEqual({ name: "Envelopes", eventTypes: ["envelope"] });
  });

  it("returns undefined on 204", async () => {
    const { fetch, calls } = mockFetch(json(204, null));
    await expect(client(fetch).alerts.deleteRule("r1")).resolves.toBeUndefined();
    expect(calls[0]!.method).toBe("DELETE");
  });

  it("downloads exports as text or bytes", async () => {
    const csv = "id,score\n1,2\n";
    const { fetch } = mockFetch(new Response(csv, { status: 200, headers: { "content-type": "text/csv" } }));
    const c = client(fetch);
    expect(await c.exports.download("job1")).toBe(csv);
    const bytes = await c.exports.download("job1", { as: "bytes" });
    expect(bytes).toBeInstanceOf(Uint8Array);
    expect(new TextDecoder().decode(bytes)).toBe(csv);
  });

  it("exposes the last rate-limit headers", async () => {
    const { fetch } = mockFetch(
      json(200, { data: {} }, {
        "X-RateLimit-Limit": "150",
        "X-RateLimit-Remaining": "149",
        "X-RateLimit-Minute-Limit": "9000",
        "X-RateLimit-Minute-Remaining": "8999",
        "X-RateLimit-Daily-Limit": "1200000",
        "X-RateLimit-Daily-Remaining": "1199999",
      }),
    );
    const c = client(fetch);
    expect(c.lastRateLimit).toBeUndefined();
    await c.account.entitlements();
    expect(c.lastRateLimit).toEqual({
      limit: 150,
      remaining: 149,
      minuteLimit: 9000,
      minuteRemaining: 8999,
      dailyLimit: 1200000,
      dailyRemaining: 1199999,
    });
  });
});

describe("errors", () => {
  it("maps the envelope onto TokTokDataError", async () => {
    const { fetch } = mockFetch(
      json(403, { code: "insufficient_scope", message: "Needs gifter:read.", requestId: "req-1", retryable: false }),
    );
    const error = await client(fetch).gifters.list().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(TokTokDataError);
    const e = error as TokTokDataError;
    expect(e.status).toBe(403);
    expect(e.code).toBe("insufficient_scope");
    expect(e.message).toBe("Needs gifter:read.");
    expect(e.requestId).toBe("req-1");
    expect(e.retryable).toBe(false);
    expect(e.isForbidden).toBe(true);
    expect(e.isRateLimited).toBe(false);
    expect(e.details?.code).toBe("insufficient_scope");
  });

  it("maps 401, 404 helpers", async () => {
    const { fetch } = mockFetch(json(401, { code: "unauthorized", message: "bad key" }), json(404, { code: "not_found", message: "nope" }));
    const c = client(fetch);
    const e401 = (await c.rankings.regions().catch((e: unknown) => e)) as TokTokDataError;
    const e404 = (await c.rankings.regions().catch((e: unknown) => e)) as TokTokDataError;
    expect(e401.isUnauthorized).toBe(true);
    expect(e404.isNotFound).toBe(true);
  });

  it("does not retry 402 and keeps resetAt", async () => {
    const { fetch, calls } = mockFetch(
      json(402, { code: "credit_cap_exceeded", message: "Out of credits.", resetAt: "2026-12-01T00:00:00Z", retryable: false }),
    );
    const e = (await client(fetch).creators.search({ q: "linh" }).catch((x: unknown) => x)) as TokTokDataError;
    expect(calls).toHaveLength(1);
    expect(e.isPaymentRequired).toBe(true);
    expect(e.details?.resetAt).toBe("2026-12-01T00:00:00Z");
  });

  it("turns a non-JSON body into code http_error", async () => {
    const { fetch } = mockFetch(new Response("<html>Bad gateway</html>", { status: 502 }));
    const e = (await client(fetch, { maxRetries: 0 }).rankings.regions().catch((x: unknown) => x)) as TokTokDataError;
    expect(e.status).toBe(502);
    expect(e.code).toBe("http_error");
    expect(e.retryable).toBe(true);
    expect(e.message).toContain("502");
  });
});

describe("retries", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it("retries 429 after Retry-After seconds", async () => {
    const { fetch, calls } = mockFetch(
      json(429, { code: "rate_limited", message: "slow down", retryable: true }, { "Retry-After": "3" }),
      ok({ regions: [] }),
    );
    const promise = client(fetch).rankings.regions();
    await vi.advanceTimersByTimeAsync(2_900);
    expect(calls).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(200);
    expect(calls).toHaveLength(2);
    await expect(promise).resolves.toMatchObject({ data: { regions: [] } });
  });

  it("caps Retry-After at 60 s", async () => {
    const { fetch, calls } = mockFetch(json(429, { code: "rate_limited", message: "x" }, { "Retry-After": "600" }), ok());
    const promise = client(fetch).rankings.regions();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(calls).toHaveLength(2);
    await promise;
  });

  it("does not retry daily_quota_exceeded", async () => {
    const { fetch, calls } = mockFetch(json(429, { code: "daily_quota_exceeded", message: "tomorrow" }, { "Retry-After": "1" }));
    const e = (await client(fetch).rankings.regions().catch((x: unknown) => x)) as TokTokDataError;
    expect(calls).toHaveLength(1);
    expect(e.isRateLimited).toBe(true);
    expect(e.retryAfter).toBe(1);
  });

  it("gives up after maxRetries", async () => {
    const { fetch, calls } = mockFetch(json(429, { code: "rate_limited", message: "x" }, { "Retry-After": "1" }));
    const promise = client(fetch, { maxRetries: 2 }).rankings.regions();
    const assertion = expect(promise).rejects.toMatchObject({ status: 429 });
    await vi.advanceTimersByTimeAsync(5_000);
    await assertion;
    expect(calls).toHaveLength(3);
  });

  it("retries 5xx without envelope or with retryable true, not retryable false", async () => {
    const a = mockFetch(new Response("oops", { status: 503 }), ok());
    const pa = client(a.fetch).rankings.regions();
    await vi.advanceTimersByTimeAsync(1_000);
    await pa;
    expect(a.calls).toHaveLength(2);

    const b = mockFetch(json(500, { code: "internal", message: "x", retryable: true }), ok());
    const pb = client(b.fetch).rankings.regions();
    await vi.advanceTimersByTimeAsync(1_000);
    await pb;
    expect(b.calls).toHaveLength(2);

    const c = mockFetch(json(503, { code: "live_playback_not_configured", message: "off", retryable: false }));
    await expect(client(c.fetch).rankings.regions()).rejects.toMatchObject({ code: "live_playback_not_configured" });
    expect(c.calls).toHaveLength(1);
  });

  it("does not retry other 4xx", async () => {
    const { fetch, calls } = mockFetch(json(400, { code: "invalid_request", message: "limit" }));
    await expect(client(fetch).rankings.regions()).rejects.toMatchObject({ status: 400 });
    expect(calls).toHaveLength(1);
  });

  it("retries network errors on GET only", async () => {
    const get = mockFetch(new TypeError("fetch failed"), ok());
    const promise = client(get.fetch).rankings.regions();
    await vi.advanceTimersByTimeAsync(1_000);
    await promise;
    expect(get.calls).toHaveLength(2);

    const post = mockFetch(new TypeError("fetch failed"), ok());
    await expect(client(post.fetch).live.refresh("linh.live")).rejects.toMatchObject({ code: "network_error", status: 0 });
    expect(post.calls).toHaveLength(1);
  });

  it("retries 429 on POST too (the limiter answers before the handler)", async () => {
    const { fetch, calls } = mockFetch(
      json(429, { code: "rate_limited", message: "x" }, { "Retry-After": "1" }),
      json(202, { queued: true, jobId: "j1" }),
    );
    const promise = client(fetch).live.refresh("linh.live");
    await vi.advanceTimersByTimeAsync(1_000);
    await expect(promise).resolves.toEqual({ queued: true, jobId: "j1" });
    expect(calls).toHaveLength(2);
    expect(calls.every((c) => c.method === "POST")).toBe(true);
  });

  it("never retries 5xx on POST", async () => {
    const { fetch, calls } = mockFetch(json(503, { code: "unavailable", message: "x", retryable: true }));
    await expect(client(fetch).alerts.createRule({ name: "x" })).rejects.toMatchObject({ status: 503 });
    expect(calls).toHaveLength(1);
  });

  it("never retries 402 on any method", async () => {
    const { fetch, calls } = mockFetch(json(402, { code: "credit_cap_exceeded", message: "x" }, { "Retry-After": "1" }));
    await expect(client(fetch).live.refresh("linh.live")).rejects.toMatchObject({ isPaymentRequired: true });
    expect(calls).toHaveLength(1);
  });

  it("validates 2–4 ids in live.compare before calling", async () => {
    const { fetch, calls } = mockFetch(ok());
    const c = client(fetch);
    await expect(c.live.compare(["only-one"])).rejects.toBeInstanceOf(RangeError);
    await expect(c.live.compare(["a", "b", "c", "d", "e"])).rejects.toBeInstanceOf(RangeError);
    expect(calls).toHaveLength(0);
  });

  it("times out a hanging request", async () => {
    const fetch = vi.fn(
      (_url: string, init: { signal?: AbortSignal }) =>
        new Promise<Response>((_resolve, reject) => {
          init.signal?.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
        }),
    );
    const promise = client(fetch as never, { timeoutMs: 1_000, maxRetries: 0 }).rankings.regions();
    const assertion = expect(promise).rejects.toMatchObject({ code: "timeout", status: 0 });
    await vi.advanceTimersByTimeAsync(1_000);
    await assertion;
  });
});

describe("202 waiting", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  const pending = (retryAfterSeconds?: number, headers: Record<string, string> = {}) =>
    json(202, { status: "resolving", jobId: "job-1", ...(retryAfterSeconds !== undefined ? { retryAfterSeconds } : {}) }, headers);

  it("waits retryAfterSeconds and asks again until 200", async () => {
    const { fetch, calls } = mockFetch(pending(5), pending(5), ok({ creator: { uniqueId: "linh.live" } }));
    const promise = client(fetch).creators.get("linh.live");
    await vi.advanceTimersByTimeAsync(4_999);
    expect(calls).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(calls).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(5_000);
    const result = await promise;
    expect(result.data.creator.uniqueId).toBe("linh.live");
    expect(calls).toHaveLength(3);
  });

  it("uses Retry-After when the body has no hint and clamps to 1–120 s", async () => {
    const { fetch, calls } = mockFetch(pending(undefined, { "Retry-After": "0" }), pending(500), ok({ entries: [], nextCursor: null }));
    const promise = client(fetch).creators.followers("linh.live", { limit: 10 });
    await vi.advanceTimersByTimeAsync(1_000);
    expect(calls).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(119_000);
    expect(calls).toHaveLength(2);
    await vi.advanceTimersByTimeAsync(1_000);
    await promise;
    expect(calls).toHaveLength(3);
    expect(calls[2]!.url.searchParams.get("limit")).toBe("10");
    expect(calls[2]!.url.searchParams.has("wait")).toBe(false);
  });

  it("returns the 202 body with wait: false", async () => {
    const { fetch, calls } = mockFetch(pending(7));
    const result = await client(fetch).videos.comments("v1", { wait: false });
    expect(calls).toHaveLength(1);
    expect(isPending(result)).toBe(true);
    if (isPending(result)) expect(result.retryAfterSeconds).toBe(7);
  });

  it("throws PendingTimeoutError when the budget runs out", async () => {
    const { fetch, calls } = mockFetch(pending(60));
    const promise = client(fetch).live.playback("linh.live", { maxWaitMs: 100_000 });
    const assertion = expect(promise).rejects.toBeInstanceOf(PendingTimeoutError);
    await vi.advanceTimersByTimeAsync(60_000);
    await assertion;
    expect(calls).toHaveLength(2);
    const error = (await promise.catch((e: unknown) => e)) as PendingTimeoutError;
    expect(error.jobId).toBe("job-1");
    expect(error.pendingStatus).toBe("resolving");
    expect(error.retryAfterSeconds).toBe(60);
    expect(error).toBeInstanceOf(TokTokDataError);
  });

  it("gives up after 180 s by default", async () => {
    const { fetch, calls } = mockFetch(pending(30));
    const promise = client(fetch).creators.get("linh.live");
    const assertion = expect(promise).rejects.toBeInstanceOf(PendingTimeoutError);
    await vi.advanceTimersByTimeAsync(180_000);
    await assertion;
    expect(calls).toHaveLength(7);
  });

  it("waits for an export to be ready", async () => {
    const job = (status: string) => json(200, { id: "e1", status, rowCount: 5000 });
    const { fetch, calls } = mockFetch(job("pending"), job("pending"), job("ready"));
    const promise = client(fetch).exports.waitUntilReady("e1");
    await vi.advanceTimersByTimeAsync(4_000);
    await expect(promise).resolves.toMatchObject({ status: "ready" });
    expect(calls).toHaveLength(3);
  });

  it("returns a failed export instead of throwing", async () => {
    const { fetch } = mockFetch(json(200, { id: "e1", status: "failed", error: "too many rows" }));
    await expect(client(fetch).exports.waitUntilReady("e1")).resolves.toMatchObject({ status: "failed" });
  });

  it("throws PendingTimeoutError when an export stays pending past the timeout", async () => {
    const { fetch, calls } = mockFetch(json(200, { id: "e1", status: "pending" }));
    const promise = client(fetch).exports.waitUntilReady("e1", { timeoutMs: 5_000 });
    const assertion = expect(promise).rejects.toBeInstanceOf(PendingTimeoutError);
    await vi.advanceTimersByTimeAsync(5_000);
    await assertion;
    expect(calls).toHaveLength(3);
  });
});

describe("pagination", () => {
  const page = (cursor: string | null, n: number) => ok({ items: [{ id: `s${n}` }], nextCursor: cursor, historyWindowDays: 365 });

  it("iterates pages with live.iterSessions", async () => {
    const { fetch, calls } = mockFetch(page("c1", 1), page("c2", 2), page(null, 3));
    const ids: string[] = [];
    for await (const p of client(fetch).live.iterSessions({ limit: 1, creatorUniqueId: "linh.live" })) {
      ids.push(String(p.data.items[0]!.id));
    }
    expect(ids).toEqual(["s1", "s2", "s3"]);
    expect(calls.map((c) => c.url.searchParams.get("cursor"))).toEqual([null, "c1", "c2"]);
    expect(calls.every((c) => c.url.searchParams.get("creatorUniqueId") === "linh.live")).toBe(true);
  });

  it("works through client.paginate for any cursor list and honours maxPages", async () => {
    const { fetch, calls } = mockFetch(page("c1", 1), page("c2", 2), page("c3", 3));
    const c = client(fetch);
    const pages = [];
    for await (const p of c.paginate((params) => c.creators.followers("linh.live", params), { limit: 50 }, { maxPages: 2 })) {
      pages.push(p);
    }
    expect(pages).toHaveLength(2);
    expect(calls).toHaveLength(2);
    expect(calls[1]!.url.pathname).toBe("/v1/creators/linh.live/followers");
  });

  it("stops when the API repeats a cursor", async () => {
    const { fetch, calls } = mockFetch(page("same", 1));
    const c = client(fetch);
    let count = 0;
    for await (const _ of c.rankings.iterHistory("daily")) count++;
    expect(count).toBe(2);
    expect(calls).toHaveLength(2);
  });
});
