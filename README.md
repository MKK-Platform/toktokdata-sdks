# toktokdata.live SDKs, MCP server and samples

Official client libraries for the [toktokdata.live](https://toktokdata.live) API: TikTok LIVE
rankings, creators, gifters, agencies, LIVE sessions, trends, alerts and exports.

Use an API key instead of hand-written HTTP calls: the SDKs add auth, retries, waiting for data
that is still being collected, pagination and typed errors.

| Package | Install | Folder |
|---|---|---|
| TypeScript / JavaScript | `npm install @toktokdata/sdk` | [`js/`](js) |
| Python | `pip install toktokdata` | [`python/`](python) |
| Go | `go get github.com/MKK-Platform/toktokdata-sdks/go` | [`go/`](go) |
| MCP server (Claude, Cursor, VS Code ...) | `npx -y @toktokdata/mcp` | [`mcp/`](mcp) |
| Samples: cURL, TypeScript, Python, Go, Java, C#, PHP, Ruby | — | [`examples/`](examples) |
| OpenAPI 3 document | — | [`openapi/openapi.json`](openapi/openapi.json) |

Docs: <https://toktokdata.live/docs> · API reference: <https://toktokdata.live/docs/reference>

## Get an API key

Sign in at <https://toktokdata.live>, open **Settings → API keys** and create a key with the
scopes you need. The key (`ttk_live_...`) is shown once. Keep it on the server side and pass it
through the environment:

```bash
export TOKTOKDATA_API_KEY=ttk_live_...
```

## Quickstart

**TypeScript**

```ts
import { TokTokData } from '@toktokdata/sdk'

const client = new TokTokData() // reads TOKTOKDATA_API_KEY
const board = await client.rankings.official('hourly', { region: 'VN' })
console.log(board.data)
```

**Python**

```python
from toktokdata import TokTokData

client = TokTokData()  # reads TOKTOKDATA_API_KEY
board = client.rankings.official("hourly", region="VN")
print(board["data"])
```

**Go**

```go
c, err := toktokdata.NewClient() // reads TOKTOKDATA_API_KEY
if err != nil {
	log.Fatal(err)
}
board, err := c.Rankings.Official(ctx, "hourly", &toktokdata.RankingsOfficialParams{Region: "VN"})
```

**MCP** (Claude Desktop, `claude_desktop_config.json`)

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

Claude Code: `claude mcp add toktokdata -e TOKTOKDATA_API_KEY=ttk_live_... -- npx -y @toktokdata/mcp`.
More clients in [`mcp/README.md`](mcp/README.md).

## What every SDK does for you

- `Authorization: Bearer` from `TOKTOKDATA_API_KEY` (or an explicit option).
- Retries on `429` and retryable `5xx`, honouring `Retry-After`; never on `402` (out of credits).
- Waits on `202` ("still collecting") for creator lookups, follow lists, video comments and LIVE
  playback, up to three minutes.
- Cursor pagination helpers.
- One error type with `status`, `code`, `message`, `requestId` and `retryable`.
- The last response's `X-RateLimit-*` figures.
- Webhook signature verification for alert rules (`x-toktokdata-signature`).

The shared contract is in [`DESIGN.md`](DESIGN.md).

## Credits and limits

Every call spends credits from your workspace wallet and counts against your plan's request
rate. See [credits](https://toktokdata.live/docs/concepts/credits) and
[rate limits](https://toktokdata.live/docs/concepts/rate-limits).

## Contributing and support

Bug reports and pull requests are welcome, see [CONTRIBUTING.md](CONTRIBUTING.md). Security
issues: [SECURITY.md](SECURITY.md). Questions about your account or data:
support@toktokdata.live.

## License

[MIT](LICENSE)
