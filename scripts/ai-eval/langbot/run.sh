#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 3 ]]; then
  echo 'Usage: run.sh LANGBOT_SOURCE VENV_PYTHON OUTPUT_DIRECTORY' >&2
  exit 2
fi
script_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export LANGBOT_SOURCE
LANGBOT_SOURCE=$(cd "$1" && pwd)
python="$2"
mkdir -p "$3"
export AI_EVAL_OUTPUT
AI_EVAL_OUTPUT=$(cd "$3" && pwd)
expected=6b83089535c1b71bae45f42bdb522d67f20be3eb
actual=$(git -C "$LANGBOT_SOURCE" rev-parse HEAD)
if [[ "$actual" != "$expected" ]]; then
  echo "Source revision mismatch: expected $expected; got $actual" >&2
  exit 2
fi
git -C "$LANGBOT_SOURCE" diff --quiet -- src/langbot
export PYTHONPATH="$LANGBOT_SOURCE/src"
"$python" "$script_dir/http_probe.py" > "$AI_EVAL_OUTPUT/http-probe.log"
"$python" "$script_dir/runner_probe.py" > "$AI_EVAL_OUTPUT/runner-probe.log"
"$python" "$script_dir/upstream_probe.py" > "$AI_EVAL_OUTPUT/upstream-probe.log"
printf 'LangBot component probes completed. Results: %s\n' "$AI_EVAL_OUTPUT"
