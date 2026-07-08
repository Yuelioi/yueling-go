# Index - webui-admin

## State

Design approved in conversation. Implementation plan is written in `plan.md` and waiting for the user's execution choice.

## Next

Ask the user to choose an execution mode for `plan.md`: subagent-driven development or inline execution.

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

Current:
- Implementation plan handoff.

Verified:
- No implementation code has been changed.
- `node --version` is `v22.14.0`; `pnpm --version` is `11.1.2`.

## Open questions

- None for the spec. Implementation plan starts after user review.
