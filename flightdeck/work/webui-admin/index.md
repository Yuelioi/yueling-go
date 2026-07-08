# Index - webui-admin

## State

Design approved in conversation. Spec is written in `design.md` and waiting for user review before implementation planning.

## Next

Ask the user to review `design.md`. After approval, use `superpowers:writing-plans` to write the implementation plan in this topic package.

## Read now

- design.md

## Read if

- flightdeck/knowledge/logging/logx.md - before adding or changing Go logging in the implementation plan.

## Progress

Done:
- Explored current project context: startup in `cmd/bot/main.go`, config in `config/config.go`, DB in `db/db.go`, bot dispatch in `bot/bot.go`, plugin help registry in `plugins/system/help.go`, AI affinity in `ai/affinity.go`, existing external HTTP API in `services/httpapi`.
- Settled scope through grilling: SQLite-backed runtime management, single password auth, silent plugin disable behavior, help-system plugin IDs, affinity score management only, Vite build served by Go/Gin, NapCat group list source, per-group disable records, plaintext WebUI password.
- Chose architecture route 1: thin admin UI with a unified bot dispatch plugin gate.
- Wrote `design.md`.
- Self-reviewed `design.md` for placeholders, contradictions, scope creep, and ambiguous feedback behavior.

Current:
- Spec review gate.

Verified:
- No implementation code has been changed.

## Open questions

- None for the spec. Implementation plan starts after user review.
