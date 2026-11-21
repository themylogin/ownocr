#!/usr/bin/env bash
# Runs the Playwright suite against the development stack.
# Pass extra arguments straight through, for example:
#   ./run-e2e.sh grid.spec.ts
set -euo pipefail

cd "$(dirname "$0")"

COMPOSE=(docker compose -f docker-compose.dev.yml -f docker-compose.dev-e2e.yml)
if ! docker info >/dev/null 2>&1; then
  COMPOSE=(sudo "${COMPOSE[@]}")
fi

cleanup() {
  if [ "${KEEP_STACK:-0}" != "1" ]; then
    "${COMPOSE[@]}" down --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

echo "==> building and starting the stack"
"${COMPOSE[@]}" up -d --build postgres backend frontend sidecar

echo "==> waiting for the backend"
for _ in $(seq 1 60); do
  if "${COMPOSE[@]}" exec -T frontend wget -qO- http://backend:8080/api/health 2>/dev/null | grep -q '"status":"ok"'; then
    break
  fi
  sleep 2
done

echo "==> waiting for the frontend"
for _ in $(seq 1 60); do
  if "${COMPOSE[@]}" exec -T frontend wget -qO- http://localhost:5173/ >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

echo "==> installing test dependencies"
"${COMPOSE[@]}" run --rm --entrypoint sh e2e -c 'npm install --no-audit --no-fund --silent'

echo "==> running the suite"
"${COMPOSE[@]}" run --rm e2e npx playwright test "$@"
