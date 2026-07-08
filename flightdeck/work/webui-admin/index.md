# Index - webui-admin

## State

Implementation in progress on worktree branch `webui-admin`.

## Next

Continue with Task 9 in `plan.md`: Frontend Scaffold.

## Read now

- design.md
- plan.md

## Read if

- flightdeck/knowledge/logging/logx.md - before adding or changing Go logging in the implementation plan.

## Progress

Done:
- Explored current project context: startup in `cmd/bot/main.go`, config in `config/config.go`, DB in `db/db.go`, bot dispatch in `bot/bot.go`, plugin help registry in `plugins/system/help.go`, AI affinity in `ai/affinity.go`, existing external HTTP API in `services/httpapi`.
- Settled scope through grilling: SQLite-backed runtime management, single password auth, silent plugin disable behavior, help-system plugin IDs, affinity score management only, Vite build served by Go/Gin, NapCat group list source, per-group disable records, plaintext WebUI password.
- Chose architecture route 1: thin admin UI with a unified bot dispatch plugin gate.
- Wrote `design.md`.
- Self-reviewed `design.md` for placeholders, contradictions, scope creep, and ambiguous feedback behavior.
- Wrote `plan.md` with task-by-task implementation steps.
- Created isolated worktree `E:\projects\apps\yueling-go\.worktrees\webui-admin` on branch `webui-admin`; baseline `go test ./...` passed.
- Completed Task 1: WebUI config. Implementer commit `93ad45f` added config/docs/tests; follow-up commit `d8112d4` fixed repeated-load leakage by unmarshalling into local config before assigning global `C`.
- Completed Task 2: DB Models And Admin Mutations. Commit `539f716` added DB model/helpers/tests, `28995b2` fixed numeric nickname search, and `b28e9d1` made affinity adjustments atomic with regression coverage.
- Completed Task 3: Export The Plugin Catalog. Commit `d7a777f` added catalog export/constants/tests; follow-up commit `45b1cf9` made `Catalog()` side-effect-free so it does not consume help registry finalization before `image.Register`.
- Completed Task 4: Bot Plugin Gate And Group List API. Commit `2de1a7` added plugin metadata/gate and `GetGroupList`; follow-up commit `2f44662` made the gate concurrency-safe and fixed timeout-driven tests.
- Completed Task 5: Tag Existing Plugin Registrations. Commit `ac6204d` tagged managed handlers with catalog plugin IDs; follow-up commit `fd3f379` made notice/request handlers obey the plugin gate for non-zero group IDs.
- Completed Task 6: WebUI Server Auth And Static Shell. Commit `3addc2b` added the Gin server, password login, session auth, static SPA fallback, and tests; follow-up commit `31d4c83` hardened cookie options and password comparison.
- Completed Task 7: WebUI JSON APIs. Commit `51c25ef` added protected admin routes for groups, plugins, and AI affinity; follow-up commit `bc4aa8b` required explicit `disabled` fields and added a `groupLister` test seam for `/groups` success/error coverage.
- Completed Task 8: Wire WebUI Into Bot Startup. Commit `c3a193c` connected the DB-backed plugin gate and starts the WebUI server when `[webui].enabled` is true.

Current:
- Task 9: Frontend Scaffold.

Verified:
- No implementation code has been changed.
- `node --version` is `v22.14.0`; `pnpm --version` is `11.1.2`.
- Task 1 spec review passed.
- Task 1 code-quality review passed after fix.
- `go test ./config -run WebUI -v -count=1` passed.
- `go test ./config -v -count=1` passed.
- Task 2 spec review passed after numeric nickname search fix.
- Task 2 code-quality review passed after atomic adjustment fix.
- `go test ./db -run "GroupPluginDisabled|AIAffinityAdmin" -v -count=1` passed.
- `go test ./db -v -count=1` passed.
- Task 3 spec review passed.
- Task 3 code-quality review passed after catalog finalization-order fix.
- `go test ./plugins/system -run Catalog -v -count=1` passed.
- `go test ./plugins/system -v -count=1` passed.
- Task 4 spec review passed.
- Task 4 code-quality review passed after plugin gate concurrency/test fix.
- `go test ./bot -run "PluginGate|ParseGroupList" -v -count=1` passed.
- `go test ./bot -v -count=1` passed.
- `go test ./bot -race -run "PluginGate" -count=1` passed.
- Task 5 spec review passed after notice/request gate fix.
- Task 5 code-quality review passed after notice/request gate fix.
- `rg -n "b\\.On(Command|GroupMessage|Keyword|Regex|FullMatch|Notice|Request)" plugins cmd\bot\main.go` completed as the Task 5 sanity check.
- `go test ./bot -run TestPluginGate` passed.
- `go test ./bot ./plugins/... ./cmd/bot` passed.
- `go test ./plugins/... ./cmd/bot` passed.
- `go test ./...` passed.
- `git diff --check` passed.
- Task 6 spec review passed.
- Task 6 code-quality review initially required cookie/password hardening; follow-up review passed after `31d4c83`.
- `go test ./services/webui -run "Login|Protected|Logout|Static" -v -count=1` passed.
- `go test ./services/webui -run "Login|Protected|Logout|Static|Cookie" -v -count=1` passed.
- `go test ./services/webui -v -count=1` passed.
- `go test -race ./services/webui -count=1` passed.
- `go test ./...` passed after adding Gin.
- `go list -m github.com/gin-gonic/gin` resolved `github.com/gin-gonic/gin v1.12.0`.
- Task 7 spec review passed.
- Task 7 code-quality review initially required explicit `disabled` validation and `/groups` success/error coverage; follow-up review passed after `bc4aa8b`.
- `go test ./services/webui -run "Groups|Plugins|Affinity" -v -count=1` passed.
- `go test ./services/webui -v -count=1` passed.
- `go test -race ./services/webui -count=1` passed.
- `go test ./...` passed after Task 7.
- Task 8 spec review passed.
- Task 8 code-quality review passed.
- `go test ./cmd/bot ./bot ./services/webui -v` passed.
- `go test ./bot ./services/httpapi ./services/webui ./config` passed.
- `go test ./cmd/bot` passed.
- `go test ./db` passed.
- `go test ./...` passed after Task 8.

## Open questions

- None for the spec. Implementation plan starts after user review.
