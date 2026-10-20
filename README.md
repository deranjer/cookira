# Cookira

A self-hosted household recipe book that AI agents can read and write over
[MCP](https://modelcontextprotocol.io/). Successor to the Hugo `hugo-cookbook`
site; built on the same stack as Loopira.

- **Private by default.** Everything needs a login. A recipe can get a
  revocable, view-only **share link** (`/s/<token>`) that works without one.
- **Structured recipes.** Ingredient groups, numbered steps with sub-steps,
  tags, timings, notes (markdown), a header photo.
- **Agent-friendly.** `/mcp` exposes tools to search, create, patch, and share
  recipes. Agents create **drafts**; a person publishes. Every change (web or
  agent, with the API key name) is recorded and can be restored.
- **One binary + Postgres 18.** The React UI is embedded in the Go binary;
  migrations run on startup.

## Stack

Go (chi + huma REST/OpenAPI, pgx, goose, sqlc, `modelcontextprotocol/go-sdk`),
React + Vite + TypeScript + Mantine + TanStack Query, PostgreSQL 18.

## Run with Docker

```sh
docker compose up --build
```

App at http://localhost:8080 (first visit runs setup to create the admin).
Set `PUBLIC_URL` (e.g. `https://recipes.example.com`) so share links and the
secure-cookie flag are right behind a TLS proxy.

| Variable | Default | |
|---|---|---|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/cookira?sslmode=disable` | |
| `ADDR` | `:8080` | listen address |
| `PUBLIC_URL` | `http://localhost:8080` | used for share links; `https://` enables Secure cookies |
| `IMAGES_DIR` | `./data/images` | recipe photos |

## Development

```sh
docker compose up -d db          # Postgres 18 on localhost:5434
export DATABASE_URL="postgres://postgres:postgres@localhost:5434/cookira?sslmode=disable"
go run ./cmd/server              # API + MCP on :8080
cd web && pnpm install && pnpm dev   # UI on :5190, proxies /api to :8080
```

After editing `internal/db/queries/*.sql` or migrations: `sqlc generate`.

The initial schema is consolidated into `internal/db/migrations/00001_init.sql`,
including image thumbnails and OAuth tables. This baseline is for new databases;
databases created with the earlier three-migration development history need a
separate migration-history reconciliation before adopting it. Do not rerun init
or reset an existing database to apply this change.

Tests (the DB-backed ones are skipped unless `TEST_DATABASE_URL` points at a
throwaway Postgres 18 database):

```sh
go test ./...
TEST_DATABASE_URL="postgres://postgres:postgres@localhost:5434/cookira_test?sslmode=disable" go test ./...
```

## Connecting agents

Create an API key in **Settings → API keys** (read-only or read-write), then:

```sh
# Claude Code
claude mcp add --transport http cookira http://localhost:8080/mcp \
  --header "Authorization: Bearer <key>"
```

```toml
# Codex: ~/.codex/config.toml  (export COOKIRA_API_KEY=<key>)
[mcp_servers.cookira]
url = "http://localhost:8080/mcp"
bearer_token_env_var = "COOKIRA_API_KEY"
```

Claude desktop's config only speaks stdio, so use the `mcp-remote` bridge; the
exact JSON is shown in Settings. Photos are resized on upload (a 1600px full-size and a 640px thumbnail, both JPEG,
EXIF-rotated and metadata-stripped). `set_recipe_image` takes a public image URL
(private/local addresses are refused) or base64 data.

### ChatGPT and other OAuth-only clients

Codex does **not** require OAuth: it supports Bearer API keys as configured
above ([official MCP documentation](https://learn.chatgpt.com/docs/extend/mcp?surface=cli)).
ChatGPT's custom MCP connection supports OAuth, no authentication, or mixed
OAuth/anonymous access, without a custom API-key input
([official connection documentation](https://developers.openai.com/api/docs/guides/custom-mcp-server)).
For private Cookira access in ChatGPT, use OAuth.

ChatGPT's custom MCP servers (and claude.ai connectors) can't send a Bearer key,
so Cookira is also an OAuth 2.1 authorization server. Bearer API keys keep
working side by side.

1. Cookira must be reachable over **public HTTPS** (set `PUBLIC_URL` to that
   address; it becomes the OAuth issuer). ChatGPT can't reach `localhost`.
2. In ChatGPT, add a custom MCP server with URL `https://<your-host>/mcp` and
   authentication **OAuth** (leave client ID/secret empty).
3. ChatGPT registers itself, opens Cookira's sign-in and consent page, and you
   choose **read-only** or **read-write**. The connection (listed in
   **Settings → Connected apps**, revocable there) acts as you; recipe history
   shows `key:oauth:<app name>` as the actor.

What is implemented: discovery (`/.well-known/oauth-protected-resource[/mcp]`,
`/.well-known/oauth-authorization-server`), dynamic client registration
(`POST /oauth/register`, public clients only), authorization code + PKCE (S256
required), `POST /oauth/token` (authorization_code and rotating refresh tokens,
so connections survive `offline_access`-style long use; access tokens last 1
hour, refresh tokens 90 days), and `POST /oauth/revoke`. `/mcp` answers 401 with
a `WWW-Authenticate: Bearer resource_metadata=...` challenge. Tokens, codes and
refresh tokens are stored only as SHA-256 hashes. Client ID Metadata Documents
are not supported (clients fall back to dynamic registration).

To exercise the flow by hand: register a client, sign in, POST the consent
decision (`/oauth/authorize/decision`, what the React page does) with a PKCE
challenge, then exchange the returned `code` at `/oauth/token`.

Tools: `list_recipes`, `get_recipe`,
`list_tags`, `get_recipe_history`, `create_recipe`, `update_recipe`,
`add_recipe_step`, `add_recipe_ingredient`, `set_recipe_image`, `delete_recipe`,
`restore_recipe_revision`, `create_share_link`, `list_share_links`,
`revoke_share_link`.

## Layout

```
cmd/server/            entrypoint
internal/recipes/      domain layer: validation, revisions, shares, images (REST and MCP both use it)
internal/api/          huma/chi routes (REST, images, public share view, /mcp mount)
internal/mcpserver/    MCP tools
internal/auth/         sessions, bcrypt, Bearer API keys and OAuth access tokens
internal/oauth/        OAuth 2.1 authorization server (discovery, registration, token, consent API)
internal/db/           migrations (goose), queries (sqlc), generated code
internal/storage/      image storage interface (local disk)
internal/webapp/       embedded frontend build
web/                   React SPA
```

See [PLAN.md](PLAN.md) for the design notes and roadmap.
