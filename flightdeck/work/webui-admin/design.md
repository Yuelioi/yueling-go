# WebUI Admin Design

## Goal

Add a WebUI admin surface that starts with the bot process, is protected by a configured password, and lets an operator manage per-group plugin availability and AI affinity scores.

## Non-Goals

- No QQ identity binding, QR login, or multi-user account system.
- No public-internet security hardening beyond password login and authenticated API routes.
- No online editing of AI affinity algorithm rules, keyword classifiers, or thresholds.
- No standalone Node production server.
- No separate global plugin-disable data model in the first version.

## Configuration

`config.toml` gets a new `[webui]` section:

```toml
[webui]
enabled = true
addr = ":9080"
password = "change-me"
```

Behavior:
- `enabled = false` means the WebUI server does not start.
- `enabled = true` requires a non-empty `password`; startup fails during config validation if it is missing.
- `addr` is the Gin listen address.
- The password is plaintext, matching the current project pattern where secrets live in `config.toml`.

Runtime management data is stored in SQLite, not written back to `config.toml`. This avoids mutating the commonly read-only Docker config mount and makes changes effective immediately.

## Authentication

The first version uses one admin password with full permissions.

The backend exposes:
- `POST /api/webui/auth/login` with `{ "password": "..." }`
- `POST /api/webui/auth/logout`
- `GET /api/webui/auth/me`

Successful login sets an HTTP-only session cookie. Sessions can be in-memory and expire after a fixed window, such as 24 hours. Restarting the bot invalidates sessions. All non-login WebUI API routes require a valid session and return `401` JSON when unauthenticated.

## Backend Architecture

Add `services/webui` using Gin. `cmd/bot/main.go` starts it in a goroutine after config load, DB init, plugin registration, and bot construction when `[webui].enabled` is true.

Main backend responsibilities:
- Serve `webui/dist` for the browser app.
- Provide authenticated JSON APIs under `/api/webui/*`.
- Hold the latest connected `*bot.BotAPI`, similar to `services/httpapi`, so WebUI can ask NapCat for live group data.
- Return `503` when an API requires a live bot connection but NapCat is not connected.

The server should use `services/logx` for all logging.

## Group Source

Group list comes from NapCat through a new `BotAPI.GetGroupList()` wrapper around OneBot `get_group_list`.

The WebUI shows a clear offline state when the bot is not connected or the list cannot be refreshed. The first version does not need a persistent "seen groups" table. Plugin disable records can still be stored by `group_id`; they are surfaced when that group appears in the NapCat list.

## Plugin Catalog

The WebUI uses the same plugin catalog as `帮助` so the UI matches existing bot terminology.

Implementation direction:
- Move or expose the help registry as a read-only catalog that can be consumed outside `plugins/system`.
- Each catalog item has `id`, `name`, `group`, `desc`, and `commands`.
- Dynamic image-related help entries still finalize after `image.Register`, matching the existing help behavior.

The first version manages plugin entries, not individual commands and not internal Go registration functions.

## Plugin Disable Model

Add a SQLite model similar to:

```go
type GroupPluginDisabled struct {
    ID       uint  `gorm:"primarykey;autoIncrement"`
    GroupID  int64 `gorm:"uniqueIndex:idx_group_plugin_disabled"`
    PluginID int   `gorm:"uniqueIndex:idx_group_plugin_disabled"`
}
```

Behavior:
- A row means the plugin is disabled for that group.
- Deleting the row enables the plugin for that group.
- The WebUI can batch apply one plugin to all currently loaded groups by writing or deleting per-group rows.
- There is no separate global-disable table in the first version.

## Bot Dispatch Integration

Add plugin metadata to bot handler registrations, for example `Builder.Plugin(id int)`.

`bot.dispatchGroupMessage` should check the plugin gate after a matcher succeeds and before the handler runs:
- If the registration has no plugin ID, it is not affected by group plugin disables.
- If the plugin is disabled for the current group, dispatch silently skips that handler and continues to later handlers.
- The bot does not reply with "plugin disabled".

This preserves silent behavior for command plugins and passive plugins such as link parsing, repeater, poke, and proactive AI.

## AI Affinity Management

The WebUI manages existing `db.AIAffinity` rows.

Features:
- Filter/list by group.
- Search by QQ number or nickname.
- Display group ID/name, user ID, nickname, score, last reason, and update time.
- Set score directly.
- Increment or decrement score by an operator-entered delta.
- Reset score to `config.C.AI.Affinity.Initial`.

All write operations clamp scores to normalized `[min, max]` from `config.C.AI.Affinity`. Low scores below `block_below` are visually marked in the UI but the WebUI does not change threshold configuration.

## API Shape

The implementation plan can refine names, but the first version should cover these routes:

```text
POST   /api/webui/auth/login
POST   /api/webui/auth/logout
GET    /api/webui/auth/me

GET    /api/webui/groups
GET    /api/webui/plugins
GET    /api/webui/groups/:groupID/plugins
PUT    /api/webui/groups/:groupID/plugins/:pluginID
POST   /api/webui/plugins/:pluginID/apply-all

GET    /api/webui/affinity?group_id=...&q=...
PUT    /api/webui/affinity/:id/score
POST   /api/webui/affinity/:id/adjust
POST   /api/webui/affinity/:id/reset
```

API responses are JSON. Validation errors return `400`, authentication failures return `401`, missing rows return `404`, and live bot dependency failures return `503`.

## Frontend Architecture

Create `webui/` with:
- Vue 3
- TypeScript
- Vite 8
- Nuxt UI
- Tabler icons via Iconify/Nuxt UI icon configuration
- pnpm

Production:
- `pnpm build` writes `webui/dist`.
- Gin serves `webui/dist` and falls back to `index.html` for SPA routes.
- No Node process is required in production.

Development:
- Vite dev server can proxy `/api/webui` to the Go WebUI address.

## Frontend Screens

Screens:
- `/login`: password-only login.
- Main admin shell: compact management layout with navigation between "群插件" and "AI 好感度".
- Group plugins page:
  - left or top group selector populated from NapCat;
  - grouped plugin list matching the help catalog;
  - per-plugin switches;
  - per-plugin "apply to all loaded groups" actions.
- AI affinity page:
  - group selector;
  - search by nickname or QQ;
  - table with group, QQ, nickname, score, last reason, update time;
  - row actions for set, adjust, reset.

Feedback behavior:
- Prefer local, in-place feedback for successful saves: switch loading state, row "saved" state, disabled controls while a request is in flight, and immediate UI updates after success.
- Use toast only for errors, session expiry, or failed batch operations that need attention.
- Avoid a landing/marketing page; the first authenticated screen is the admin tool.

## Testing And Verification

Backend:
- `config.Load` validates `[webui]` correctly.
- DB tests cover group plugin disable create/delete/query/batch behavior.
- DB tests cover affinity list/search/set/adjust/reset with score clamping.
- Bot dispatch tests verify plugin-disabled handlers are skipped silently and enabled handlers still run.
- WebUI handler tests verify login, session enforcement, wrong password rejection, JSON validation, and protected route behavior.
- `BotAPI.GetGroupList` has a JSON parsing unit test using a fake response path.

Frontend:
- `pnpm build` passes.
- Login, group plugins, and affinity pages compile and call the typed API client.
- Add Vitest coverage for small pure state/API helpers if the setup remains lightweight.

Whole project verification:
- `go test ./...`
- `go vet ./...`
- `pnpm --dir webui build`
- Manual startup check with `[webui].enabled=true` and `[webui].enabled=false`.
