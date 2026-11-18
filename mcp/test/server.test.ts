import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { Client } from "@modelcontextprotocol/sdk/client/index.js";
import { InMemoryTransport } from "@modelcontextprotocol/sdk/inMemory.js";
import { TokTokData, type FetchLike } from "@toktokdata.live/sdk";
import { afterEach, describe, expect, it, vi } from "vitest";
import { z } from "zod";
import { TOOLS, createServer, type ServerOptions } from "../src/index.js";

const EXPECTED = [
  "rankings_official",
  "rankings_regions",
  "rankings_history",
  "rankings_day_bars",
  "creators_search",
  "creators_get",
  "creators_analysis",
  "creators_changes",
  "creators_followers",
  "creators_following",
  "creators_gifters",
  "creators_insights",
  "creators_videos",
  "agencies_list",
  "agencies_get",
  "insights_best_time",
  "insights_presence",
  "videos_get",
  "videos_comments",
  "trends_list",
  "gifts_list",
  "gifters_list",
  "live_sessions",
  "live_session",
  "live_session_timeline",
  "live_sessions_compare",
  "live_performance",
  "live_comments",
  "account_usage",
  "account_entitlements",
];

interface Call {
  url: URL;
  headers: Record<string, string>;
}

function json(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json", ...headers } });
}

function mockClient(...replies: Response[]) {
  const calls: Call[] = [];
  const fetch = vi.fn(async (url: string, init: { headers: Record<string, string> }) => {
    calls.push({ url: new URL(url), headers: init.headers });
    const reply = replies.length > 1 ? replies.shift()! : replies[0]!;
    return reply.clone();
  });
  const client = new TokTokData({ apiKey: "ttk_live_test", baseUrl: "https://api.test", fetch: fetch as unknown as FetchLike, maxRetries: 0 });
  return { client, calls };
}

async function connect(options: ServerOptions) {
  const server = createServer(options);
  const [clientTransport, serverTransport] = InMemoryTransport.createLinkedPair();
  const client = new Client({ name: "test", version: "0.0.0" });
  await Promise.all([server.connect(serverTransport), client.connect(clientTransport)]);
  return client;
}

async function call(options: ServerOptions, name: string, args: Record<string, unknown> = {}) {
  const client = await connect(options);
  const result = await client.callTool({ name, arguments: args });
  const content = result.content as Array<{ type: string; text: string }>;
  return { isError: result.isError === true, text: content[0]!.text };
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("tool registration", () => {
  it("lists exactly the 30 read-only tools", async () => {
    const client = await connect({ client: mockClient(json(200, {})).client });
    const { tools } = await client.listTools();
    expect(tools).toHaveLength(30);
    expect(tools.map((t) => t.name).sort()).toEqual([...EXPECTED].sort());
    for (const tool of tools) {
      expect(tool.annotations?.readOnlyHint).toBe(true);
      expect(tool.annotations?.openWorldHint).toBe(true);
      expect(tool.description).toMatch(/Needs API key scope `[a-z:]+`/);
      expect(tool.description).toMatch(/Free \(no credits\)|Costs credits/);
      expect(tool.inputSchema.type).toBe("object");
    }
  });

  it("exposes no write, export, refresh or playback tool", () => {
    for (const tool of TOOLS) {
      expect(tool.name).not.toMatch(/alert|export|refresh|playback|masked/);
      expect(tool.operation.startsWith("GET ")).toBe(true);
    }
  });

  it("matches openapi.json: route, scope, credits, param names and enums", () => {
    const specPath = fileURLToPath(new URL("../../openapi/openapi.json", import.meta.url));
    const spec = JSON.parse(readFileSync(specPath, "utf8")) as {
      paths: Record<string, Record<string, { parameters?: Array<{ name: string; required?: boolean; schema?: { enum?: string[] } }>; "x-scope": string; "x-credits": string; "x-public"?: boolean }>>;
    };
    for (const tool of TOOLS) {
      const [method, path] = tool.operation.split(" ") as [string, string];
      const op = spec.paths[path]?.[method.toLowerCase()];
      expect(op, tool.operation).toBeDefined();
      expect(op!["x-public"]).toBe(true);
      expect(op!["x-scope"], tool.name).toBe(tool.scope);
      expect(op!["x-credits"], tool.name).toBe(tool.credits);
      const params = op!.parameters ?? [];
      expect(Object.keys(tool.input).sort(), tool.name).toEqual(params.map((p) => p.name).sort());
      for (const param of params) {
        const field = tool.input[param.name] as z.ZodType;
        // Required in the spec ⇔ required in the tool.
        expect(field.safeParse(undefined).success, `${tool.name}.${param.name} optional`).toBe(!param.required);
        // Every spec enum value is accepted (liveOnly is a string enum "true"/"false" in the spec, a boolean here).
        if (param.schema?.enum && param.name !== "liveOnly") {
          for (const value of param.schema.enum) expect(field.safeParse(value).success, `${tool.name}.${param.name}=${value}`).toBe(true);
          expect(field.safeParse("not-an-enum-value").success).toBe(false);
        }
      }
    }
  });

  it("descriptions mention waiting on 202-aware tools only", () => {
    const waiting = TOOLS.filter((t) => t.waits).map((t) => t.name).sort();
    expect(waiting).toEqual(["creators_followers", "creators_following", "creators_get", "videos_comments"]);
  });
});

describe("tool calls", () => {
  it("calls the API and returns its JSON as text", async () => {
    const body = { data: { board: { entries: [] } }, provenance: { sourceKind: "official" } };
    const { client, calls } = mockClient(json(200, body));
    const result = await call({ client }, "rankings_official", { board: "hourly", region: "VN", liveOnly: true, limit: 5 });
    expect(result.isError).toBe(false);
    expect(JSON.parse(result.text)).toEqual(body);
    expect(calls[0]!.url.pathname).toBe("/v1/rankings/official");
    expect(Object.fromEntries(calls[0]!.url.searchParams)).toEqual({ board: "hourly", region: "VN", liveOnly: "true", limit: "5" });
    expect(calls[0]!.headers.Authorization).toBe("Bearer ttk_live_test");
  });

  it("maps path params and arrays", async () => {
    const { client, calls } = mockClient(json(200, { data: {} }));
    await call({ client }, "live_sessions_compare", { ids: ["a", "b"] });
    await call({ client }, "creators_followers", { uid: "linh.live", limit: 10 });
    expect(calls[0]!.url.searchParams.get("ids")).toBe("a,b");
    expect(calls[1]!.url.pathname).toBe("/v1/creators/linh.live/followers");
    expect(calls[1]!.url.searchParams.get("limit")).toBe("10");
  });

  it("rejects invalid arguments before calling the API", async () => {
    const { client, calls } = mockClient(json(200, {}));
    const result = await call({ client }, "gifters_list", { segment: "billionaire" });
    expect(result.isError).toBe(true);
    expect(calls).toHaveLength(0);
  });
});

describe("error mapping", () => {
  it("explains a missing key", async () => {
    vi.stubEnv("TOKTOKDATA_API_KEY", "");
    const result = await call({ client: () => new TokTokData() }, "rankings_regions");
    expect(result.isError).toBe(true);
    expect(result.text).toContain("TOKTOKDATA_API_KEY is not set");
  });

  it("explains 401", async () => {
    const { client } = mockClient(json(401, { code: "invalid_api_key", message: "Unknown key.", requestId: "r-1" }));
    const result = await call({ client }, "rankings_regions");
    expect(result.isError).toBe(true);
    expect(result.text).toContain("API key was rejected");
    expect(result.text).toContain("r-1");
  });

  it("explains 402 with the reset time", async () => {
    const { client } = mockClient(
      json(402, { code: "credit_cap_exceeded", message: "Out of credits.", resetAt: "2026-12-01T00:00:00.000Z" }),
    );
    const result = await call({ client }, "creators_search", { q: "linh" });
    expect(result.isError).toBe(true);
    expect(result.text).toContain("out of credits");
    expect(result.text).toContain("2026-12-01T00:00:00.000Z");
  });

  it("names the missing scope on 403 insufficient_scope", async () => {
    const { client } = mockClient(json(403, { code: "insufficient_scope", message: "Missing scope." }));
    const result = await call({ client }, "gifters_list");
    expect(result.isError).toBe(true);
    expect(result.text).toContain("`gifter:read`");
  });

  it("explains a plan without the module on 403 module_not_entitled", async () => {
    const { client } = mockClient(json(403, { code: "module_not_entitled", message: "Needs Pro." }));
    const result = await call({ client }, "creators_gifters", { uid: "linh.live" });
    expect(result.isError).toBe(true);
    expect(result.text).toContain("plan does not include");
  });

  it("gives the retry time on 429", async () => {
    const { client } = mockClient(json(429, { code: "rate_limited", message: "Slow down." }, { "Retry-After": "12" }));
    const result = await call({ client }, "trends_list");
    expect(result.isError).toBe(true);
    expect(result.text).toContain("Retry in 12 s");
  });

  it("says to retry later when the data is still being collected", async () => {
    const { client, calls } = mockClient(json(202, { status: "resolving", jobId: "job-9", retryAfterSeconds: 30 }));
    const result = await call({ client, maxWaitSeconds: 5 }, "creators_get", { uid: "new.creator" });
    expect(calls).toHaveLength(1);
    expect(result.isError).toBe(false);
    expect(result.text).toContain("still collecting");
    expect(result.text).toContain("again in 30 s");
  });
});
