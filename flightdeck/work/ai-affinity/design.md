# AI Chat Affinity Design

## Goal

Remove the old public affinity feature and PK battle command. Rebuild affinity as hidden AI-chat state: AI chat updates a per-user, per-group score based on the user's message, uses that score to shape tone, and silently refuses to answer when the score is too low.

## Decisions

- Affinity is no longer backed by game `Score`.
- Affinity is not queryable by users.
- PK is deleted because its only durable effect was changing the shared game score.
- Existing check-in / score / ranking can remain as game points, but they no longer mean AI affinity.
- Future web/admin UI can read the hidden affinity table directly.

## Data Model

Create a dedicated `AIAffinity` table:

- `user_id`
- `group_id`
- `score`
- `level`
- `last_reason`
- `updated_at`

The table is keyed by `(user_id, group_id)`. New records start from a neutral score.

## Scoring

Use a local deterministic scorer first, not another LLM call.

Negative signals:
- sexual harassment / explicit sexual teasing toward the bot
- insults, abusive language, hostile spam
- jailbreak / prompt injection attempts
- repeated low-quality provocation

Positive signals:
- normal sincere questions
- polite thanks
- useful follow-up conversation

The scorer returns a delta and a reason. Obvious harmful messages get larger negative deltas; ordinary messages recover slowly. This makes tests stable and avoids spending extra tokens on moderation.

## AI Chat Flow

`ai.Dispatch` should:

1. Apply rate limit and existing security guard.
2. Score the user's current input and persist the hidden affinity delta.
3. If the score is below the block threshold, return an empty reply so `plugins/ai_dispatch` sends nothing.
4. Build the system prompt with an affinity level, not the exact score.
5. Run the existing ReAct loop.

The prompt should describe only behavior, for example "当前关系：疏远；回复应更冷淡简短". It must not expose hidden scores.

## Public Surface Removed

Remove these public affordances:

- `查看好感度` / `查询好感度`
- AI tool `affinity_ranking`
- help entry "好感度"
- README "好感度" user section

Remove PK:

- `plugins/game/pk.go`
- `game.RegisterPK(b)`
- help entry "PK对战"
- README `pk @某人`
- `db.UpdatePKResult`

## Configuration

Add optional `[ai.affinity]` settings with defaults:

- `enabled = true`
- `initial = 50`
- `block_below = 10`
- `min = 0`
- `max = 100`

The default should work without config changes.

## Testing

Use TDD.

Core tests:
- DB creates and updates hidden affinity records.
- Score clamps to configured min/max.
- Harmful sexual/provocation input lowers score.
- Normal or polite input raises score slowly.
- Low score causes dispatch to return empty before calling LLM.
- System prompt includes level wording but not exact score.
- Removed commands/tools no longer register or compile references.

## Out Of Scope

- Web/admin UI.
- LLM-based moderation.
- User-visible affinity query, ranking, or notifications.
