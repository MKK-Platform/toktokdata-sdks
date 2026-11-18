import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import type { CallToolResult } from "@modelcontextprotocol/sdk/types.js";
import { DEFAULT_BASE_URL, PendingTimeoutError, TokTokData, TokTokDataError } from "@toktokdata.live/sdk";
import { TOOLS, describeTool, type ToolDef } from "./tools.js";
import { VERSION } from "./version.js";

export const DEFAULT_MAX_WAIT_SECONDS = 40;

export interface ServerOptions {
  /** A ready client, or a factory called on the first tool call (default: `new TokTokData()` from env). */
  client?: TokTokData | (() => TokTokData);
  /** How long 202-aware tools wait for fresh data, in seconds (default 40). */
  maxWaitSeconds?: number;
}

const INSTRUCTIONS =
  "Read-only access to the toktokdata.live API: official TikTok LIVE rankings, creator profiles and follow graph, " +
  "recorded LIVE sessions, gifts, gifters, agencies, trends and the account's credit usage. Every answer carries " +
  "`provenance`: `sourceKind` says whether a number is TikTok's own (`official`) or computed by toktokdata.live " +
  "(`derived`), `coverageStatus` how complete it is. Treat a missing number as unknown, not 0. Most tools cost " +
  "credits; `account_entitlements` and `account_usage` are free and show the remaining balance.";

const text = (value: string, isError = false): CallToolResult => ({
  content: [{ type: "text", text: value }],
  ...(isError ? { isError: true } : {}),
});

function seconds(value: number | undefined): string {
  return value === undefined ? "a little while" : `${Math.max(1, Math.ceil(value))} s`;
}

/** Turns any failure into a short, actionable tool result for the model (and the person reading it). */
export function errorResult(error: unknown, tool: Pick<ToolDef, "name" | "scope">, baseUrl?: string): CallToolResult {
  if (error instanceof PendingTimeoutError) {
    return text(
      `toktokdata.live is still collecting this data (status ${error.pendingStatus ?? "resolving"}` +
        `${error.jobId ? `, job ${error.jobId}` : ""}). Call ${tool.name} again in ${seconds(error.retryAfterSeconds)}.`,
    );
  }
  if (!(error instanceof TokTokDataError)) {
    const message = error instanceof Error ? error.message : String(error);
    return text(`${tool.name} failed: ${message}`, true);
  }
  const ref = error.requestId ? ` (request ${error.requestId})` : "";
  const detail = error.message ? ` ${error.message.trim().replace(/\.?$/, ".")}` : "";
  switch (true) {
    case error.code === "missing_api_key":
      return text(
        "TOKTOKDATA_API_KEY is not set for the toktokdata MCP server. Create a key in the toktokdata.live dashboard " +
          "(Settings → API keys) and add it to the server's env in your MCP client config, then restart the server.",
        true,
      );
    case error.status === 401:
      return text(
        `The toktokdata.live API key was rejected (401 ${error.code}).${detail} Check TOKTOKDATA_API_KEY: it must be ` +
          `an active key (ttk_live_...).${ref}`,
        true,
      );
    case error.status === 402: {
      const resetAt = typeof error.details?.resetAt === "string" ? ` Credits reset at ${error.details.resetAt}.` : "";
      return text(
        `The workspace is out of credits (402 ${error.code}).${resetAt} Top up credits or upgrade the plan in the ` +
          `toktokdata.live dashboard; retrying will not help until then.${ref}`,
        true,
      );
    }
    case error.status === 403 && error.code === "module_not_entitled":
      return text(
        `The workspace's plan does not include this feature (403 module_not_entitled).${detail} Upgrade the plan to ` +
          `use ${tool.name}.${ref}`,
        true,
      );
    case error.status === 403:
      return text(
        `The API key lacks the \`${tool.scope}\` scope that ${tool.name} needs (403 ${error.code}).${detail} Create ` +
          `or edit a key with the \`${tool.scope}\` scope in the toktokdata.live dashboard.${ref}`,
        true,
      );
    case error.status === 404:
      return text(`Not found (404 ${error.code}).${detail}${ref}`, true);
    case error.status === 429 && error.code === "daily_quota_exceeded":
      return text(
        `The daily request quota of this workspace is used up (429 daily_quota_exceeded).${detail}` +
          `${error.retryAfter !== undefined ? ` Retry in ${seconds(error.retryAfter)}.` : " Retry tomorrow."}${ref}`,
        true,
      );
    case error.status === 429:
      return text(
        `Rate limited by toktokdata.live (429 ${error.code}). Retry in ${seconds(error.retryAfter ?? 1)}.${ref}`,
        true,
      );
    case error.status === 400:
      return text(`Invalid request (400 ${error.code}).${detail}${ref}`, true);
    case error.status === 0:
      return text(
        `Could not reach ${baseUrl ?? "the toktokdata.live API"} (${error.code}).${detail} Check the network or ` +
          "TOKTOKDATA_BASE_URL.",
        true,
      );
    default:
      return text(
        `toktokdata.live answered ${error.status} ${error.code}.${detail}` +
          `${error.retryable ? ` This is temporary: retry in ${seconds(error.retryAfter ?? 5)}.` : ""}${ref}`,
        true,
      );
  }
}

/** Builds the MCP server with every read-only tool registered. Connect it to a transport to serve. */
export function createServer(options: ServerOptions = {}): McpServer {
  const maxWaitSeconds = options.maxWaitSeconds ?? DEFAULT_MAX_WAIT_SECONDS;
  let client: TokTokData | undefined = options.client instanceof TokTokData ? options.client : undefined;
  const factory = typeof options.client === "function" ? options.client : () => new TokTokData();
  const getClient = () => (client ??= factory());

  const server = new McpServer({ name: "toktokdata", version: VERSION }, { instructions: INSTRUCTIONS });

  for (const tool of TOOLS) {
    server.registerTool(
      tool.name,
      {
        title: tool.title,
        description: describeTool(tool, maxWaitSeconds),
        inputSchema: tool.input,
        annotations: { title: tool.title, readOnlyHint: true, openWorldHint: true },
      },
      async (args: Record<string, unknown>) => {
        let baseUrl: string | undefined;
        try {
          const c = getClient();
          baseUrl = c.baseUrl;
          const result = await tool.run(c, args, { maxWaitMs: maxWaitSeconds * 1000 });
          return text(result === undefined ? "null" : JSON.stringify(result));
        } catch (error) {
          return errorResult(error, tool, baseUrl ?? DEFAULT_BASE_URL);
        }
      },
    );
  }
  return server;
}
