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
if [ -f go.mod ]; then
  VER=$(git describe --tags --always 2>/dev/null || echo "dev")
else
  VER=$(ssh -i "$PUSK_BUILD_KEY" "$PUSK_BUILD_HOST" "cd $PUSK_BUILD_DIR && git describe --tags --always" 2>/dev/null || echo "dev")
fi
echo "=== Deploying Pusk $VER ==="

# --- Lint + Test gate ---
echo "[0/6] Lint & test..."
if [ -f go.mod ]; then
  echo "  go vet..."
  go vet ./...
  echo "  gofmt..."
  BAD=$(gofmt -l . | grep -v vendor || true)
  [ -n "$BAD" ] && { echo "FAIL: gofmt: $BAD"; exit 1; }
  echo "  tests..."
  go test ./... -count=1 -timeout 60s
  echo "  All checks passed."
else
  BSSH="ssh -i $BUILD_KEY $BUILD_HOST"
  $BSSH "cd $PUSK_BUILD_DIR && export PATH=\$PATH:/usr/local/go/bin && go vet ./... && test -z \"\$(gofmt -l .)\" && go test ./... -count=1 -timeout 60s"
  echo "  All checks passed (remote)."
fi

# --- Build ---
echo "[1/6] Building..."
if [ -f go.mod ]; then
  # Local build (running on build host)
  CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/pusk-platform/pusk/internal/api.Version=$VER"     -o /tmp/pusk-deploy ./cmd/pusk
else
  # Remote build via BUILD_HOST
  BUILD_HOST="${PUSK_BUILD_HOST:?Set PUSK_BUILD_HOST or run from source dir}"
  BUILD_KEY="${PUSK_BUILD_KEY:?Set PUSK_BUILD_KEY}"
  BUILD_DIR="${PUSK_BUILD_DIR:-/srv/projects/pusk}"
  BSSH="ssh -i $BUILD_KEY $BUILD_HOST"
  $BSSH "cd $BUILD_DIR && export PATH=\\$PATH:/usr/local/go/bin && CGO_ENABLED=0 go build -ldflags \"-s -w -X github.com/pusk-platform/pusk/internal/api.Version=$VER\" -o /tmp/pusk-deploy ./cmd/pusk"
  scp -i "$BUILD_KEY" "$BUILD_HOST:/tmp/pusk-deploy" /tmp/pusk-deploy
fi
echo "  Binary: $(du -h /tmp/pusk-deploy | cut -f1)"

# --- Upload ---
echo "[2/6] Uploading binary + static..."
$SCP /tmp/pusk-deploy "$REMOTE_HOST:/tmp/pusk-deploy"
if [ -f go.mod ]; then
  for f in web/static/index.html web/static/manifest.json web/static/sw.js web/static/css/pusk.css web/static/js/*.js; do
    $SCP "$f" "$REMOTE_HOST:$REMOTE_DIR/$f"
  done
else
  BSSH="ssh -i $BUILD_KEY $BUILD_HOST"
  for f in index.html manifest.json sw.js css/pusk.css js/state.js js/storage.js js/util.js js/views.js js/ws.js js/actions.js js/settings.js js/landing.js js/onboard.js js/app.js; do
    scp -i "$BUILD_KEY" "$BUILD_HOST:$BUILD_DIR/web/static/$f" "/tmp/pusk-static-$(basename $f)"
    $SCP "/tmp/pusk-static-$(basename $f)" "$REMOTE_HOST:$REMOTE_DIR/web/static/$f"
  done
fi

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

  ERRORS=$($SSH "tail -10 $REMOTE_DIR/pusk.log 2>/dev/null | grep -c ERROR || true" 2>/dev/null | tr -d '[:space:]')
  [ "${ERRORS:-0}" -gt 0 ] 2>/dev/null && echo "  WARNING: $ERRORS errors in recent log" || true
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
