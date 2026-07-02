# Index — ai-affinity

## State

Design approved by user: delete PK, make all affinity an internal AI-chat-only score, and expose no user query command. Future web/admin UI may inspect the hidden DB state.

No business code has been changed yet.

## Next

Review `design.md` and `plan.md`. If approved, implement the plan with TDD:

1. Add hidden AI affinity persistence and scoring tests.
2. Wire AI dispatch to update score and silently skip replies below threshold.
3. Remove public affinity commands/tools and PK.
4. Update help/README/config docs and run verification.

## Read now

- design.md
- plan.md

## Read if

- ../../knowledge/logging/logx.md — before adding or changing log statements.
- ../../knowledge/command/slow-command-progress-react.md — if adding a slow external/AI command; this plan does not.

## Progress

Done:
- Project context inspected: AI dispatch, DB models, PK command, public affinity command, AI affinity ranking tool, help registry, README references.
- User confirmed hidden affinity cannot be queried by ordinary users.
- Design and implementation plan written.

Current:
- Waiting for user review / execution choice.

Verified:
- No code implementation run yet.

## Open questions

- Exact threshold values can be adjusted during implementation, but the plan proposes defaults with config overrides.
