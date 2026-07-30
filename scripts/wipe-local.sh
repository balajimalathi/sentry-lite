#!/usr/bin/env bash
# Reset local SQLite + Redpanda together so ingest backlog cannot resurrect deleted projects.
# Stop the API / TUI / load tool first (they hold the consumer group and DB).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

REDPANDA_COMPOSE="${REDPANDA_COMPOSE:-docker-compose.redpanda.yml}"
DATA_DIR="${DATA_DIR:-./data}"
SQLITE_PATH="${SQLITE_PATH:-$DATA_DIR/sentry-lite.db}"

usage() {
  cat <<'EOF'
Usage: scripts/wipe-local.sh [--yes]

Wipes local telemetry state:
  - SQLite DB (+ WAL/SHM) at SQLITE_PATH (default ./data/sentry-lite.db)
  - on-disk event payloads under DATA_DIR/events
  - Redpanda volume via docker compose -f docker-compose.redpanda.yml down -v

Env overrides: REDPANDA_COMPOSE, DATA_DIR, SQLITE_PATH

Stop sentry-lite / TUI / load generators before running.
EOF
}

confirm=0
for arg in "$@"; do
  case "$arg" in
    -y|--yes) confirm=1 ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $arg" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [[ "$confirm" -ne 1 ]]; then
  echo "This deletes $SQLITE_PATH, $DATA_DIR/events, and the Redpanda volume."
  read -r -p "Continue? [y/N] " ans
  case "$ans" in
    y|Y|yes|YES) ;;
    *)
      echo "aborted"
      exit 1
      ;;
  esac
fi

echo "==> stopping Redpanda and removing volume"
docker compose -f "$REDPANDA_COMPOSE" down -v

echo "==> removing SQLite + event payloads"
rm -f "$SQLITE_PATH" "$SQLITE_PATH-wal" "$SQLITE_PATH-shm"
rm -rf "$DATA_DIR/events"
mkdir -p "$DATA_DIR/events"

echo "==> starting Redpanda"
docker compose -f "$REDPANDA_COMPOSE" up -d

echo "==> waiting for Redpanda health"
for _ in $(seq 1 30); do
  if docker compose -f "$REDPANDA_COMPOSE" exec -T redpanda rpk cluster health 2>/dev/null | grep -q 'Healthy'; then
    break
  fi
  sleep 1
done

echo "Done. Restart the API/TUI, then create a project (Issues should be empty)."
echo "Check consumer lag later with:"
echo "  docker compose -f $REDPANDA_COMPOSE exec redpanda rpk group describe sentry-lite-processor"
