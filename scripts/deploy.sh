#!/bin/bash
# Pusk deploy script — build, upload, restart with health check + auto-rollback
# Configure via env or scripts/deploy.env
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$SCRIPT_DIR/deploy.env" ] && source "$SCRIPT_DIR/deploy.env"

REMOTE_HOST="${PUSK_DEPLOY_HOST:?Set PUSK_DEPLOY_HOST}"
REMOTE_PORT="${PUSK_DEPLOY_PORT:-22}"
SSH_KEY="${PUSK_DEPLOY_KEY:?Set PUSK_DEPLOY_KEY}"
REMOTE_DIR="${PUSK_DEPLOY_DIR:-/opt/pusk}"
HEALTH_URL="http://localhost:${PUSK_PORT:-8443}/api/health"
ROLLBACK_WAIT="${PUSK_ROLLBACK_WAIT:-5}"

SSH="ssh -p $REMOTE_PORT -i $SSH_KEY $REMOTE_HOST"
SCP="scp -P $REMOTE_PORT -i $SSH_KEY"

cd "$SCRIPT_DIR/.."
export PATH=$PATH:/usr/local/go/bin

# --- Version ---
VER=$(git describe --tags --always 2>/dev/null || echo "dev")
echo "=== Deploying Pusk $VER ==="

# --- Build ---
echo "[1/6] Building..."
CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/pusk-platform/pusk/internal/api.Version=$VER" \
  -o /tmp/pusk-deploy ./cmd/pusk
echo "  Binary: $(du -h /tmp/pusk-deploy | cut -f1)"

# --- Upload ---
echo "[2/6] Uploading binary + static..."
$SCP /tmp/pusk-deploy "$REMOTE_HOST:/tmp/pusk-deploy"
for f in web/static/sw.js web/static/css/pusk.css web/static/js/*.js; do
  $SCP "$f" "$REMOTE_HOST:$REMOTE_DIR/$f"
done

# --- Backup ---
echo "[3/6] Backing up current binary..."
$SSH "cp $REMOTE_DIR/pusk $REMOTE_DIR/pusk.rollback 2>/dev/null || true"

# --- Stop + Replace ---
echo "[4/6] Stopping and replacing..."
$SSH "kill \$(pgrep -f 'pusk.*${PUSK_PORT:-8443}' | head -1) 2>/dev/null || true; sleep 2; cp /tmp/pusk-deploy $REMOTE_DIR/pusk; chmod +x $REMOTE_DIR/pusk"

# --- Wait for restart ---
echo "[5/6] Waiting ${ROLLBACK_WAIT}s for restart..."
sleep "$ROLLBACK_WAIT"

# --- Health check ---
echo "[6/6] Health check..."
HEALTH=$($SSH "curl -sf $HEALTH_URL" 2>/dev/null || echo "FAIL")

if echo "$HEALTH" | grep -q '"status":"ok"'; then
  VERSION=$(echo "$HEALTH" | grep -o '"version":"[^"]*"' | cut -d'"' -f4)
  ONLINE=$(echo "$HEALTH" | grep -o '"online":[0-9]*' | cut -d: -f2)
  echo ""
  echo "=== DEPLOY SUCCESS ==="
  echo "  Version: $VERSION"
  echo "  Online:  $ONLINE users"

  ERRORS=$($SSH "tail -10 $REMOTE_DIR/pusk.log 2>/dev/null | grep -c 'ERROR\|panic'" 2>/dev/null || echo "0")
  [ "$ERRORS" -gt 0 ] && echo "  WARNING: $ERRORS errors in recent log"
else
  echo ""
  echo "=== HEALTH FAILED — ROLLING BACK ==="
  $SSH "kill \$(pgrep -f 'pusk.*${PUSK_PORT:-8443}' | head -1) 2>/dev/null || true; sleep 2; cp $REMOTE_DIR/pusk.rollback $REMOTE_DIR/pusk 2>/dev/null; sleep 3"
  HEALTH2=$($SSH "curl -sf $HEALTH_URL" 2>/dev/null || echo "FAIL")
  if echo "$HEALTH2" | grep -q '"status":"ok"'; then
    echo "  Rollback OK. Previous version restored."
  else
    echo "  CRITICAL: Rollback failed! Manual intervention needed."
  fi
  exit 1
fi
