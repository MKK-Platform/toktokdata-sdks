import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { DEFAULT_BASE_URL } from "@toktokdata/sdk";
import { DEFAULT_MAX_WAIT_SECONDS, createServer } from "./server.js";
import { TOOLS } from "./tools.js";
import { VERSION } from "./version.js";

// stdout carries the MCP protocol: everything human-readable goes to stderr.

function env(name: string): string | undefined {
  const value = process.env[name]?.trim();
  return value ? value : undefined;
}

async function main(): Promise<void> {
  const args = process.argv.slice(2);
  if (args.includes("--version") || args.includes("-v")) {
    process.stderr.write(`${VERSION}\n`);
    return;
  }
  const waitRaw = Number(env("TOKTOKDATA_MCP_MAX_WAIT_SECONDS"));
  const maxWaitSeconds = Number.isFinite(waitRaw) && waitRaw > 0 ? waitRaw : DEFAULT_MAX_WAIT_SECONDS;
  const baseUrl = (env("TOKTOKDATA_BASE_URL") ?? DEFAULT_BASE_URL).replace(/\/+$/, "");

  const server = createServer({ maxWaitSeconds });
  await server.connect(new StdioServerTransport());

  const keyNote = env("TOKTOKDATA_API_KEY") ? "" : " (TOKTOKDATA_API_KEY is not set: tools will answer with an error)";
  process.stderr.write(`toktokdata MCP ready: ${TOOLS.length} tools, base ${baseUrl}${keyNote}\n`);

  const shutdown = () => {
    void server.close().finally(() => process.exit(0));
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

main().catch((error: unknown) => {
  process.stderr.write(`toktokdata MCP failed to start: ${error instanceof Error ? error.message : String(error)}\n`);
  process.exit(1);
});
