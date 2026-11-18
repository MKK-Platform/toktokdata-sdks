# Contributing

Thanks for helping. This repository holds every public client of the toktokdata.live API.

## Layout

| Folder | What | Check before a pull request |
|---|---|---|
| `js/` | `@toktokdata/sdk` (TypeScript) | `npm ci && npm run build --workspaces && npm run typecheck --workspaces && npm test --workspaces` (repo root) |
| `mcp/` | `@toktokdata/mcp` (MCP server, uses `js/`) | same as above |
| `python/` | `toktokdata` (Python) | `pip install -e ".[dev]" && ruff check . && mypy --strict src && pytest` (in `python/`) |
| `go/` | `github.com/MKK-Platform/toktokdata-sdks/go` | `gofmt -l . && go vet ./... && go test ./...` (in `go/`) |
| `examples/` | Samples per language | run the sample you changed |
| `openapi/` | Public OpenAPI document | refreshed by maintainers with `scripts/sync-openapi.mjs` |

All SDKs follow [`DESIGN.md`](DESIGN.md). A change to shared behaviour (retries, errors, `202`
waiting, pagination, webhooks) should land in every SDK, or say in the pull request which ones
still need it.

## Rules

- Tests never call the real API and never contain a real key, handle list or customer data.
- Keep the SDKs free of runtime dependencies (MCP excepted).
- One change per pull request; describe what you changed and how you tested it.

## Releases

Maintainers tag `js-vX.Y.Z`, `mcp-vX.Y.Z`, `python-vX.Y.Z` or `go/vX.Y.Z`; the Release workflow
publishes npm and PyPI packages, and the Go proxy serves the tag.
