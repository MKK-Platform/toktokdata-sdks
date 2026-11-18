import { vi } from "vitest";
import { TokTokData, type FetchLike, type TokTokDataOptions } from "../src/index.js";

export interface Call {
  url: URL;
  method: string;
  headers: Record<string, string>;
  body: string | undefined;
}

type Reply = Response | Error | (() => Response | Promise<Response>);

export function json(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

/** A fetch mock that answers from a queue (the last reply repeats) and records every call. */
export function mockFetch(...replies: Reply[]) {
  const calls: Call[] = [];
  const queue = [...replies];
  const fn = vi.fn(async (url: string, init: Parameters<FetchLike>[1]) => {
    calls.push({ url: new URL(url), method: init.method, headers: init.headers, body: init.body });
    const reply = queue.length > 1 ? queue.shift()! : queue[0];
    if (reply === undefined) throw new Error("mockFetch: no reply queued");
    if (reply instanceof Error) throw reply;
    const response = typeof reply === "function" ? await reply() : reply;
    return response.clone();
  });
  return { fetch: fn as unknown as FetchLike, calls };
}

export const KEY = "ttk_live_test_secret";

export function client(fetch: FetchLike, options: TokTokDataOptions = {}) {
  return new TokTokData({ apiKey: KEY, baseUrl: "https://api.test", fetch, ...options });
}

export const ok = (data: unknown = {}) =>
  json(200, { data, provenance: { observedAt: null, freshness: "historical", source: "t", sourceKind: "derived", coverageStatus: "observed", methodologyVersion: "v1", window: null } });
