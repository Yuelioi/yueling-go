#!/usr/bin/env bash
# notify-task-done — 给月灵 Bot 发一条「✅ … 已完成」通知。
# 配置全部来自环境变量：YUELING_API_URL / YUELING_API_KEY / YUELING_OWNER_ID
set -euo pipefail

msg="${1:-任务}"
group="${2:-}"   # 可选：第二个参数 = 群号，留空则私聊 owner

: "${YUELING_API_URL:?请设置 YUELING_API_URL}"
: "${YUELING_API_KEY:?请设置 YUELING_API_KEY}"

if ! command -v python3 >/dev/null 2>&1; then
    echo "需要 python3 来安全构造 JSON" >&2
    exit 1
fi

text="✅ ${msg} 已完成"

if [ -n "$group" ]; then
    body=$(python3 - "$text" "$group" <<'PY'
import json, sys
text, group = sys.argv[1], int(sys.argv[2])
print(json.dumps({
    "message_type": "group",
    "group_id": group,
    "message": [{"type": "text", "data": {"text": text}}],
}, ensure_ascii=False))
PY
)
else
    : "${YUELING_OWNER_ID:?私聊模式需要设置 YUELING_OWNER_ID（或传入群号作为第二个参数）}"
    body=$(python3 - "$text" "$YUELING_OWNER_ID" <<'PY'
import json, sys
text, owner = sys.argv[1], int(sys.argv[2])
print(json.dumps({
    "message_type": "private",
    "user_id": owner,
    "message": [{"type": "text", "data": {"text": text}}],
}, ensure_ascii=False))
PY
)
fi

curl -s -w '\n[http_status:%{http_code}]\n' -X POST "$YUELING_API_URL" \
    -H "Authorization: Bearer $YUELING_API_KEY" \
    -H "Content-Type: application/json" \
    -d "$body"
