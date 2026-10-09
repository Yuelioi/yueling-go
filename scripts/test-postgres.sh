#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
test_image="${YUELING_TEST_POSTGRES_IMAGE:-ghcr.io/mnixry/postgres-zhparser:16}"
test_container=""
cleanup() {
  if [[ -n "$test_container" ]]; then
    docker rm -f "$test_container" >/dev/null
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

test_container=$(docker run --detach --rm --memory=512m --cpus=1 \
  --tmpfs /var/lib/postgresql/data \
  -e POSTGRES_USER=fixture -e POSTGRES_PASSWORD=fixture -e POSTGRES_DB=yueling_test \
  -p 127.0.0.1::5432 "$test_image")
ready=false
for ((attempt=0; attempt<60; attempt++)); do
  if docker exec "$test_container" pg_isready -h 127.0.0.1 -U fixture -d yueling_test >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [[ "$ready" != true ]]; then
  docker logs "$test_container" >&2
  exit 1
fi
binding=$(docker port "$test_container" 5432/tcp)
export YUELING_TEST_DATABASE_DSN="postgres://fixture:fixture@127.0.0.1:${binding##*:}/yueling_test?sslmode=disable"
if [[ $# -eq 0 ]]; then
  set -- ./...
fi
go test -race "$@"
