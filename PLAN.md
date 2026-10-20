# Cookira: Hugo cookbook → hosted app with an MCP server

## Status (2026-10-15)

Phases 1-3, 5 and most of 6 are built and smoke-tested: scaffold, schema, REST, MCP server, share links, React UI,
Docker image + compose with Postgres 18. Remaining: deploy to the real host (reverse proxy, backups), Drone/CI pipeline,
and the "later" list below.

Deviations from the original plan, made while building:

- **Ingredients and steps are JSONB** on `recipes` (groups of plain-text lines, steps with sub-steps), and **tags are a `text[]`** column,
  instead of child tables. They are always read, written and snapshotted whole, and it keeps the agent tools simple.
  Add parsed quantities or a normalized ingredient table later when scaling/pantry features need them.
- Numeric fields use 0 for "not specified" (patch semantics stay trivial).
- Ids are `uuidv7()` (Postgres 18).
- No importer/exporter and no markdown mirror: the old Hugo `content/` folder gets fed to an agent, which recreates recipes through MCP.
- Phase 4 became "share links" (done); the Hugo importer was dropped.


## Goal

Replace the static Hugo site (`G:\Programming\cookbook`, drone-built, ~13 recipes in markdown) with a
self-hosted Go app, built on the Loopira stack (`G:\Programming\linear-replacement`), where
**agents create/update/search recipes over MCP** and humans browse/edit/print them in a web UI.

## What exists today

**hugo-cookbook** — content is `content/*.md`: YAML front matter plus a body convention.

- Front matter: `title, author, date, draft, tags[], tagline, servings, prep_time, cook (bool), cook_increment (minutes|hours), cook_time, calories, recipe_image, image_width, image_height`
- Body: `## Ingredients` (with `#### Subheading` groups, bullet lists, `[^1]` footnotes) then `## Directions` (numbered list, nested sub-steps).
- Images: `static/images/recipe-headers/*.{jpg,webp}`. Theme gives tag taxonomy, JSON search index, print view.
- Deploy: push → Drone builds Hugo → `recipes.derajnet.duckdns.org`.

**Loopira** (module `github.com/deranjer/loopira`) — the pieces we can lift as-is:

| Piece | Where | Reuse |
|---|---|---|
| chi + huma REST/OpenAPI | `internal/api` | copy pattern |
| Postgres, pgx, goose migrations, sqlc | `internal/db`, `sqlc.yaml` | copy pattern |
| Auth: sessions, bcrypt, **Bearer API keys (read-only / read-write)** | `internal/auth` | copy nearly verbatim |
| MCP server (`modelcontextprotocol/go-sdk`, Streamable HTTP, stateless, mounted `/mcp` behind `RequireAuth`) | `internal/mcpserver`, `registerMCPRoute` | copy `addTool` helper + route wiring |
| Disk `storage.Store` interface for uploads | `internal/storage` | use for recipe photos |
| Embedded React/Vite SPA served by the binary | `web/`, `internal/webapp` | copy build pipeline |
| Dockerfile / docker-compose (app + Postgres) | root | copy, rename |
| Docusaurus docs | `docs/` | optional |

## Key decisions (recommended defaults; flip any you disagree with)

1. **Source of truth = Postgres**, not git markdown. Agents writing through MCP shouldn't have to commit/push/wait for a Hugo build. No import/export tooling.
2. **Structured recipe model** instead of free-form markdown. Ingredients and steps as rows/JSON make the MCP tools reliable (an agent can "scale to 6 servings", "swap butter for oil", "add a step after 3") and make validation possible. Notes/footnotes stay as markdown text.
3. **New standalone repo/app** in `G:\Programming\cookbooks` (module `github.com/deranjer/cookbooks`), copying Loopira's scaffolding rather than importing it. Loopira is an app, not a library.
4. **Private by default, multi-user household**: login required for everything. A recipe can get a revocable **share link** (`/s/{token}`, unguessable, view-only, optional expiry; a `recipe_shares` table) that renders without login. API keys are read-only or read-write as in Loopira.
5. **Frontend**: React + Vite + Tailwind + TanStack Query as in Loopira. Organize the backend by domain package (`internal/recipes`, later `internal/mealplan`, `internal/pantry`…) and keep auth/db/storage/mcp shared, so new food features add a package and a tool group rather than touching the core.

## Data model (migration `00001_init.sql`)

- `users`, `sessions`, `api_keys` — straight from Loopira.
- `recipes`: `id uuid, slug unique, title, tagline, author, servings, prep_minutes, cook_minutes, calories, is_cooked bool (the Hugo `cook` flag), source_url, notes_md, status ('draft'|'published'), image_key, created_at, updated_at`
- `recipe_ingredient_groups`: `recipe_id, position, name` (the `####` subheadings)
- `recipe_ingredients`: `group_id, position, text` (keep as display text; optional parsed `quantity, unit, item` columns later for scaling — start with text, add parsing as a follow-up)
- `recipe_steps`: `recipe_id, position, parent_step_id nullable, text` (covers the nested sub-steps in ramen)
- `tags` + `recipe_tags`
- `recipe_revisions`: `recipe_id, revision, snapshot jsonb, changed_by, created_at` — an audit trail like Loopira's `issue_history`, so a bad agent edit is one click to undo.
- Full-text search: generated `tsvector` column over title/tagline/ingredients/tags + GIN index (replaces Hugo's JSON index).

## MCP tool surface (`internal/mcpserver`)

Read tools (any key): `list_recipes` (compact summaries; filter by tag/text/status, limit/offset), `get_recipe` (id or slug; full structured), `search_recipes`, `list_tags`, `get_recipe_history`, `create_share_link`, `revoke_share_link`.

Write tools (read-write key only): `create_recipe`, `update_recipe` (patch semantics — only supplied fields change), `set_ingredients` / `set_steps` (replace a list), `add_step`, `add_ingredient`, `set_recipe_tags`, `set_recipe_status` (draft ↔ published), `delete_recipe` (soft delete / archived), `restore_revision`, `import_recipe_from_url` (later: extracts schema.org/Recipe JSON-LD, creates a *draft*).

Conventions carried over from Loopira: tools return compact JSON text only (`addTool` wrapper, no duplicated structured content); descriptions state "Requires a read-write API key"; **agents create as `draft` by default** and a human publishes — this is the main guardrail.

Also expose recipes as MCP **resources** (`recipe://{slug}`) and a **prompt** ("add a recipe from this text") so clients that support them get a nicer flow.

## REST + web UI

- REST under `/api/v1/recipes…` (huma), generated OpenAPI; the MCP tools and REST share the same sqlc queries and DTO shaping, as Loopira does, so they can't drift.
- UI pages: recipe list (tag filter + search), recipe detail (ingredients, steps, metadata, **print view** and servings scaler), editor (structured form + drafts), tag index, API-key management, login. Photo upload via `storage.Store`, resized to webp on upload.
- Keep the current look: Bulma-ish layout, header image, stats table (prep/cook/total/calories/servings).

## Migration of existing content

None. No importer and no exporter. The old `content/*.md` folder gets handed to an agent, which recreates the recipes through the MCP tools (which doubles as the first real test of the tool surface). The Hugo repo is archived afterward.

## Hosting / deployment

- Same shape as Loopira: one app container + Postgres 18 + mounted volume for images; migrations auto-run at boot; `/api/v1/health`.
- Reverse proxy (whatever fronts `recipes.derajnet.duckdns.org` today) terminates TLS; MCP at `https://recipes…/mcp` with `Authorization: Bearer <key>`.
- Replace Drone's Hugo build with a Drone (or GitHub Actions) pipeline: `go test` → docker build → push → deploy. Nightly `pg_dump` + image dir backup, (Loopira's in-process scheduler can run it).
- MCP security to settle before exposing publicly: HTTPS only, per-key rate limit, Origin validation on `/mcp`, key scoping (read-only vs read-write), revocation UI, and request logging that records the key name on every revision.
- If you later want claude.ai / ChatGPT connectors (remote MCP), they expect OAuth rather than static Bearer keys; the MCP SDK has auth helpers. Out of scope for v1; Bearer works for Claude Code, Cursor, and your own agents.

## Phases

1. **Scaffold** — new module, copy auth/db/storage/api/webapp scaffolding from Loopira, health route, docker-compose, CI. *Done when `docker compose up` serves an empty app with login.*
2. **Data + REST** — migration, sqlc queries, recipe CRUD, tags, revisions, search. Unit tests for DTO shaping and revision snapshots.
3. **MCP server** — `/mcp` route, read tools, then write tools (drafts by default), tests modeled on `internal/mcpserver/tools_test.go`. *Done when an agent can create a recipe from pasted text and edit one step via Claude Code.*
4. **Share links** — `recipe_shares`, public view route, create/revoke tools and UI.
5. **Web UI** — list, detail (+print, scaler), editor, photo upload, API keys page.
6. **Deploy + cutover** — host on the server, DNS switch for `recipes.derajnet.duckdns.org`, backups, retire the Hugo pipeline (keep repo archived).
7. **Later / nice-to-have** — parsed quantities for real scaling, URL import polish, meal-plan/shopping-list tools, OAuth for remote connectors.

Phases 3 and 4 can run in parallel once phase 2 lands; 5 can start after phase 2 too.

## Risks

- **Free-text ingredients limit scaling/substitution tools** — mitigated by adding parsed columns later without changing the tool API.
- **Agent writes bad data** — drafts-by-default, revision history with restore, read-only keys for most agents.
- **Complexity vs. the Hugo site's "just markdown" simplicity** — if you'd rather skip a custom UI, a thin MCP + REST backend with a minimal server-rendered UI is a viable smaller v1.

## Open questions for you

Decided: Postgres 18; private by default with revocable **share links** (view-only, per recipe); React SPA, structured so the app can grow into other food features (meal plans, pantry, shopping lists); no import/export; clients are Claude desktop and Codex desktop.

Still open:
1. **Client auth**: Codex takes a streamable-HTTP MCP URL with a bearer token. Claude desktop's remote connectors expect OAuth, while a Bearer key works through a local bridge (`mcp-remote` with a header). Pick: bridge for v1, or build a small OAuth layer (single-user, so it can be minimal)?
2. **Name / module path** (see name options).
