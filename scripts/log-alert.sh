#!/bin/bash
# Pusk log alert — check for errors, send to Pusk alerts channel
# Configure via scripts/deploy.env, run via cron every 5 min
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$SCRIPT_DIR/deploy.env" ] && source "$SCRIPT_DIR/deploy.env"

REMOTE_HOST="${PUSK_DEPLOY_HOST:?}"
REMOTE_PORT="${PUSK_DEPLOY_PORT:-22}"
SSH_KEY="${PUSK_DEPLOY_KEY:?}"
REMOTE_DIR="${PUSK_DEPLOY_DIR:-/opt/pusk}"
ALERT_BOT="${PUSK_ALERT_BOT_TOKEN:-}"
ALERT_CHANNEL="${PUSK_ALERT_CHANNEL:-alerts}"
STATE="/tmp/pusk-alert-last"

SSH="ssh -p $REMOTE_PORT -i $SSH_KEY $REMOTE_HOST"
LOG="$REMOTE_DIR/pusk.log"

LINES=$($SSH "wc -l < $LOG" 2>/dev/null || echo "0")
LAST=$(cat "$STATE" 2>/dev/null || echo "0")
[ "$LINES" -le "$LAST" ] && { echo "$LINES" > "$STATE"; exit 0; }

ERRORS=$($SSH "sed -n '$((LAST+1)),${LINES}p' $LOG | grep -E 'ERROR|panic|CRITICAL'" 2>/dev/null || true)

if [ -n "$ERRORS" ] && [ -n "$ALERT_BOT" ]; then
  COUNT=$(echo "$ERRORS" | wc -l)
  SAMPLE=$(echo "$ERRORS" | tail -3 | sed 's/"/\\"/g')
  $SSH "curl -sf -X POST http://localhost:${PUSK_PORT:-8443}/bot/$ALERT_BOT/sendChannel \
    -H 'Content-Type: application/json' \
    -d '{\"channel\":\"$ALERT_CHANNEL\",\"text\":\"Self-monitor: $COUNT errors\\n\\n$SAMPLE\"}'" >/dev/null 2>&1 || true
fi

echo "$LINES" > "$STATE"
