# @toktokdata/mcp

A local [Model Context Protocol](https://modelcontextprotocol.io) server for the
[toktokdata.live](https://toktokdata.live) API. It gives Claude, Cursor, VS Code, Windsurf and other
MCP clients 30 read-only tools: official TikTok LIVE rankings, creator profiles and follow graph,
recorded LIVE sessions, gifts, gifters, agencies, trends and your credit usage.

It runs on your machine over stdio (`npx -y @toktokdata/mcp`), talks to `https://api.toktokdata.live`
with your API key, and never writes anything: no alerts, exports, refreshes or video playback.

## Setup

1. Create an API key in the toktokdata.live dashboard (**Settings → API keys**) with the scopes of the
   tools you want (see [Tools](#tools)). Keys look like `ttk_live_<id>_<secret>`.
2. Add the server to your client with the key in `TOKTOKDATA_API_KEY` (snippets below).
3. Restart the client. The server prints `toktokdata MCP ready: 30 tools, base https://api.toktokdata.live`
   to stderr (the client's MCP log).

Requires Node.js 20 or newer.

| Variable | |
|---|---|
| `TOKTOKDATA_API_KEY` | Required. Without it the server still starts, and every tool explains how to set it. |
| `TOKTOKDATA_BASE_URL` | Optional, default `https://api.toktokdata.live`. |
| `TOKTOKDATA_MCP_MAX_WAIT_SECONDS` | Optional, default `40`: how long a tool waits while fresh data is collected. |

### Claude Desktop

`claude_desktop_config.json` (Settings → Developer → Edit Config):

```json
{
  "mcpServers": {
    "toktokdata": {
      "command": "npx",
      "args": ["-y", "@toktokdata/mcp"],
      "env": { "TOKTOKDATA_API_KEY": "ttk_live_..." }
    }
  }
}
```

### Claude Code

```bash
claude mcp add toktokdata -e TOKTOKDATA_API_KEY=ttk_live_... -- npx -y @toktokdata/mcp
```

Add `--scope user` to use it in every project.

### Cursor

`.cursor/mcp.json` in the project (or `~/.cursor/mcp.json` for all projects):

```json
{
  "mcpServers": {
    "toktokdata": {
      "command": "npx",
      "args": ["-y", "@toktokdata/mcp"],
      "env": { "TOKTOKDATA_API_KEY": "ttk_live_..." }
    }
  }
}
```

### VS Code (GitHub Copilot agent mode)

`.vscode/mcp.json` (prompts for the key once and stores it securely):

```json
{
  "inputs": [
    { "type": "promptString", "id": "toktokdata-key", "description": "toktokdata.live API key", "password": true }
  ],
  "servers": {
    "toktokdata": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@toktokdata/mcp"],
      "env": { "TOKTOKDATA_API_KEY": "${input:toktokdata-key}" }
    }
  }
}
```

### Windsurf

`~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "toktokdata": {
      "command": "npx",
      "args": ["-y", "@toktokdata/mcp"],
      "env": { "TOKTOKDATA_API_KEY": "ttk_live_..." }
    }
  }
}
```

On Windows, if a client cannot start `npx` directly, use `"command": "cmd", "args": ["/c", "npx", "-y", "@toktokdata/mcp"]`.

Do not commit a config file that contains your key; use the client's secret input or a user-level
config instead.

## Tools

All tools are read-only (`readOnlyHint: true`) and return the API's JSON answer, including
`provenance` (how fresh and complete the data is, and whether a number is TikTok's own or derived).
"Credits" means the call is billed to your workspace's credit wallet as a cached read; "free" costs
nothing.

| Tool | API route | Scope | Cost |
|---|---|---|---|
| `rankings_official` | `GET /v1/rankings/official` | `rank:read` | credits |
| `rankings_regions` | `GET /v1/rankings/regions` | `rank:read` | credits |
| `rankings_history` | `GET /v1/rankings/history` | `rank:read` | credits |
| `rankings_day_bars` | `GET /v1/rankings/day-bars` | `rank:read` | credits |
| `creators_search` | `GET /v1/creators` | `creator:read` | credits |
| `creators_get` | `GET /v1/creators/{uid}` (waits on 202) | `creator:read` | credits |
| `creators_analysis` | `GET /v1/creators/{uid}/analysis` | `creator:read` | credits |
| `creators_changes` | `GET /v1/creators/{uid}/changes` | `creator:read` | credits |
| `creators_followers` | `GET /v1/creators/{uid}/followers` (waits on 202) | `creator:read` | credits |
| `creators_following` | `GET /v1/creators/{uid}/following` (waits on 202) | `creator:read` | credits |
| `creators_gifters` | `GET /v1/creators/{uid}/gifters` (Pro plan) | `gifter:read` | credits |
| `creators_insights` | `GET /v1/creators/{uid}/insights` | `creator:read` | credits |
| `creators_videos` | `GET /v1/creators/{uid}/videos` | `content:read` | credits |
| `agencies_list` | `GET /v1/agencies` | `creator:read` | credits |
| `agencies_get` | `GET /v1/agencies/{agency_id}` | `creator:read` | credits |
| `insights_best_time` | `GET /v1/insights/best-time` | `creator:read` | credits |
| `insights_presence` | `GET /v1/insights/presence` | `creator:read` | credits |
| `videos_get` | `GET /v1/videos/{video_id}` | `content:read` | credits |
| `videos_comments` | `GET /v1/videos/{video_id}/comments` (waits on 202) | `content:read` | credits |
| `trends_list` | `GET /v1/trends` | `trend:read` | credits |
| `gifts_list` | `GET /v1/gifts` | `live:read` | credits |
| `gifters_list` | `GET /v1/gifters` (Pro plan) | `gifter:read` | credits |
| `live_sessions` | `GET /v1/live/sessions` | `live:read` | credits |
| `live_session` | `GET /v1/live/sessions/{session_id}` | `live:read` | credits |
| `live_session_timeline` | `GET /v1/live/sessions/{session_id}/timeline` | `live:read` | credits |
| `live_sessions_compare` | `GET /v1/live/sessions/compare` | `live:read` | credits |
| `live_performance` | `GET /v1/live/creators/{uid}/performance` | `live:read` | credits |
| `live_comments` | `GET /v1/live/creators/{uid}/comments` | `live:read` | free |
| `account_usage` | `GET /v1/usage` | `keys:manage` | free |
| `account_entitlements` | `GET /v1/entitlements` | `keys:manage` | free |

A key for everything needs `rank:read`, `creator:read`, `content:read`, `gifter:read`, `trend:read`,
`live:read` and `keys:manage`. Shopping boards (`shopping_daily`, `shopping_weekly`) also need
`shop:read`.

Parameter names, enums and limits follow the [OpenAPI spec](../openapi/openapi.json). Paged tools
return one page; pass `data.nextCursor` back as `cursor` for the next one.

## When something goes wrong

Tools answer with a short explanation (`isError: true`) instead of failing:

- **No key**: how to set `TOKTOKDATA_API_KEY`.
- **401**: the key was rejected.
- **402**: the workspace is out of credits, with the reset time.
- **403**: the key lacks the tool's scope (named), or the plan lacks the feature.
- **429**: rate limited, with the retry time.
- **Still collecting (202)**: creator lookups and comment or follower collection can take longer than
  the wait budget; the tool says how many seconds to wait before calling it again (not an error).

## Development

From the repository root: `npm install`, `npm run build`, `npm test`, `npm run typecheck`. Try it with
the MCP Inspector: `npx @modelcontextprotocol/inspector node mcp/dist/bin.js`.

## License

MIT
