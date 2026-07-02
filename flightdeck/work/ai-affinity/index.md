# Index — ai-affinity

## State

Implemented on branch `ai-affinity`. PK and public affinity query/ranking surfaces are removed; affinity is now hidden AI-chat state backed by a dedicated DB table. Future web/admin UI may inspect the hidden DB state.

## Next

Choose branch integration:

1. Merge `ai-affinity` back to `main` locally.
2. Push branch / create PR.
3. Keep branch as-is for later.

## Read now

- design.md
- plan.md
- ../../knowledge/ai/proactive-direct-trigger-skip.md

## Read if

- ../../knowledge/logging/logx.md — before adding or changing log statements.

## Progress

Done:
- Project context inspected: AI dispatch, DB models, PK command, public affinity command, AI affinity ranking tool, help registry, README references.
- User confirmed hidden affinity cannot be queried by ordinary users.
- Design and implementation plan written.
- Added `config.AffinityConfig` and `[ai.affinity]` defaults.
- Added deterministic hidden affinity scoring and behavior-level prompts without exact score leakage.
- Added `db.AIAffinity` persistence with atomic SQLite upsert for concurrent deltas.
- Wired direct AI dispatch so hidden affinity updates before LLM; low score silently returns `""`; guard-blocked messages update affinity before denial; disabled affinity preserves old rate-limit-before-guard behavior.
- Wired proactive AI to respect hidden affinity, inject behavior prompt when enabled, and skip direct AI triggers to avoid double-counting.
- Removed public `查看好感度`/`查询好感度`, AI affinity ranking tool, PK command/registration/help/docs, and PK battle stats from `积分`.
- Updated README and `config.example.toml` for hidden `[ai.affinity]`; aligned AI base URL examples to `/v1`.

Current:
- Implementation complete on `ai-affinity`; waiting for integration choice.

Verified:
- `go test ./... -count=1` passed after final fixes.
- Runtime/docs grep for public PK/affinity query terms returns only retained `WinCount`/`LoseCount` DB model fields.
- `gofmt -l` on changed existing Go files returned no output.
- `git diff --check main..HEAD` returned no output.
- Final subagent code review approved the full branch diff through `053a956`.

## Open questions

- Integration choice: merge locally, push/PR, or keep branch as-is.
