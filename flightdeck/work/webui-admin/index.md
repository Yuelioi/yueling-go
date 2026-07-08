# Index - webui-admin

## State

Implementation in progress on worktree branch `webui-admin`.

## Next

Continue with Task 2 in `plan.md`: DB Models And Admin Mutations.

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

Current:
- Task 2: DB Models And Admin Mutations.

Verified:
- No implementation code has been changed.
- `node --version` is `v22.14.0`; `pnpm --version` is `11.1.2`.
- Task 1 spec review passed.
- Task 1 code-quality review passed after fix.
- `go test ./config -run WebUI -v -count=1` passed.
- `go test ./config -v -count=1` passed.

## Open questions

- None for the spec. Implementation plan starts after user review.
