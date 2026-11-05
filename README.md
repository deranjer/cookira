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
  agent, with the connected app's name) is recorded and can be restored.
- **One binary + Postgres 18.** The React UI is embedded in the Go binary;
  migrations run on startup.

## Stack

Go (chi + huma REST/OpenAPI, pgx, goose, sqlc, `modelcontextprotocol/go-sdk`),
React + Vite + TypeScript + Mantine + TanStack Query, PostgreSQL 18.

## Run with Docker

Images are published to `ghcr.io/deranjer/cookira` for Linux amd64 and arm64:

- `latest` and `edge` track successful builds of `main`.
- Pushing a stable Git tag such as `v1.2.3` publishes `1.2.3`, `1.2`, `1`,
  and updates `latest`. Releases before 1.0 omit the `0` alias.
- Prerelease tags such as `v1.3.0-beta.1` publish only their full version
  (plus the commit tag), without updating `latest` or stable version aliases.
- Every published build also has a `sha-<commit>` tag. Pull requests build
  the image for validation without publishing it.

Create releases by pushing a `vMAJOR.MINOR.PATCH` Git tag; version numbers
are chosen explicitly rather than inferred from commit messages. Pin a full
version for deployments, or use `ghcr.io/deranjer/cookira:latest` to follow
the most recently published main build or stable release.

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
| `JWT_SECRET` | random per run | signs login tokens; **set to 32+ characters in production** (`openssl rand -hex 32`). Without it, tokens are invalidated on every restart (clients silently refresh, so nothing breaks, but set it anyway) |

### Accounts and login

The first visit creates the admin. After that, admins add people from
**Settings → Household**: **Invite** makes a single-use link (7 days, optionally
locked to an email address, shown once) where the invitee picks their own name
and password; **Add with password** is the manual alternative. Admins can also
change roles, reset passwords and remove members.

Login uses short-lived (15 min) JWT access tokens plus rotating, single-use
refresh tokens (90 days, stored hashed; replaying a used one revokes that
user's tokens). The web app keeps both in httpOnly cookies and refreshes
silently. Other clients (a mobile app) pass `"tokens": true` to
`POST /api/v1/auth/login` (or `/setup`, `/invites/accept/{token}`) to get
`{accessToken, refreshToken, ...}` in the response body instead, send
`Authorization: Bearer <accessToken>`, and renew with
`POST /api/v1/auth/refresh {"refreshToken": "..."}`. Changing your password
signs out your other devices; repeated failed logins for an email are throttled.

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

Agents connect to `/mcp` with **OAuth only**; there are no API keys. The flow is
the standard MCP one (the same shape as Vigara's): the client discovers the
authorization server, registers itself, sends you to Cookira's sign-in and
consent page, and you choose **read-only** or **read-write**. The connection
(listed in **Settings → Connected apps**, revocable there) acts as you, and
recipe history shows `oauth:<app name>` as the actor.

```sh
# Claude Code, then run /mcp inside it and choose cookira to sign in
claude mcp add --transport http cookira http://localhost:8080/mcp
```

```toml
# Codex: ~/.codex/config.toml, then: codex mcp login cookira
[mcp_servers.cookira]
url = "http://localhost:8080/mcp"
```

For claude.ai, Claude desktop and ChatGPT, add a custom connector / MCP server
with URL `https://<your-host>/mcp`, choose **OAuth** and leave client ID and
secret empty (the client registers itself). Hosted apps can't reach
`localhost`, so Cookira must be on **public HTTPS** with `PUBLIC_URL` set to
that address (it becomes the OAuth issuer). Local clients such as Claude Code
and Codex also work against `http://localhost`.

Photos are resized on upload (a 1600px full-size and a 640px thumbnail, both
JPEG, EXIF-rotated and metadata-stripped). `set_recipe_image` takes a public
image URL (private/local addresses are refused) or base64 data. For a local or
large file, `create_image_upload_url` returns a one-time link (10 minutes, one
recipe) that the agent uses with `curl -X PUT --data-binary @photo.jpg <url>`,
so the bytes never pass through the model.

### How the OAuth server works

Credentials don't cross surfaces: `/mcp` accepts **only** OAuth access tokens
(a browser session or login token is rejected there), and the REST API accepts
only people's logins (an OAuth token is rejected there, so a connected app can't
manage users or approve more apps).

Implemented: discovery (`/.well-known/oauth-protected-resource[/mcp]`,
`/.well-known/oauth-authorization-server`), dynamic client registration
(`POST /oauth/register`, public clients only, https or loopback redirects),
authorization code + PKCE (S256 required), `POST /oauth/token`
(authorization_code and rotating refresh tokens; access tokens last 1 hour,
refresh tokens 90 days) and `POST /oauth/revoke`. `/mcp` answers 401 with a
`WWW-Authenticate: Bearer resource_metadata=...` challenge. Tokens, codes and
refresh tokens are stored only as SHA-256 hashes. Register, token and revoke
are rate limited per connection IP (forwarding headers are ignored, so clients
behind a reverse proxy share its budget). Client ID Metadata Documents are not
supported (clients fall back to dynamic registration).

To exercise the flow by hand: register a client, sign in, POST the consent
decision (`/oauth/authorize/decision`, what the React page does) with a PKCE
challenge, then exchange the returned `code` at `/oauth/token`.

Tools: `list_recipes`, `get_recipe`,
`list_tags`, `get_recipe_history`, `create_recipe`, `update_recipe`,
`add_recipe_step`, `add_recipe_ingredient`, `set_recipe_image`,
`create_image_upload_url`, `delete_recipe`,
`restore_recipe_revision`, `create_share_link`, `list_share_links`,
`revoke_share_link`.

## Layout

```
cmd/server/            entrypoint
internal/recipes/      domain layer: validation, revisions, shares, images (REST and MCP both use it)
internal/api/          huma/chi routes (REST, images, public share view, /mcp mount)
internal/mcpserver/    MCP tools
internal/auth/         login JWTs + refresh tokens, bcrypt, OAuth access tokens
internal/oauth/        OAuth 2.1 authorization server (discovery, registration, token, consent API)
internal/db/           migrations (goose), queries (sqlc), generated code
internal/storage/      image storage interface (local disk)
internal/webapp/       embedded frontend build
web/                   React SPA
```

See [PLAN.md](PLAN.md) for the design notes and roadmap.
