#!/usr/bin/env bash
# Pusk coherence check — cross-layer consistency validation
# Catches: version drift, missing JS functions, SW cache mismatch, demo tag leaks
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ERRORS=0
WARNINGS=0

fail() { echo "  FAIL: $1"; ERRORS=$((ERRORS + 1)); }
warn() { echo "  WARN: $1"; WARNINGS=$((WARNINGS + 1)); }
ok()   { echo "  OK:   $1"; }

echo "=== Pusk Coherence Check ==="
echo ""

# ── 1. Version injection ──────────────────────────────────────
echo "--- Version ---"
if grep -q 'var Version' "$REPO_ROOT/internal/api/client.go"; then
    ok "Version variable exists in internal/api/client.go"
else
    fail "Missing Version variable in internal/api/"
fi

if grep -q 'Version=' "$REPO_ROOT/.github/workflows/ci.yml"; then
    ok "CI injects version via ldflags"
else
    fail "CI missing version ldflags"
fi

# ── 2. JS function integrity ─────────────────────────────────
echo ""
echo "--- JS Functions ---"
CRITICAL_FNS="addMsg sendMsg showList openChat openChan connectWS onCb onDel toggleSub showApp logout auth setLang beep scrollDown"
for fn in $CRITICAL_FNS; do
    if grep -rq "function $fn\|async function $fn" "$REPO_ROOT/web/static/js/"; then
        ok "$fn"
    else
        fail "Missing JS function: $fn"
    fi
done

# ── 3. SW cache version ──────────────────────────────────────
echo ""
echo "--- Service Worker ---"
SW_VER=$(grep -oP "pusk-v\K[0-9]+" "$REPO_ROOT/web/static/sw.js" || echo "0")
if [ "$SW_VER" -gt 1 ]; then
    ok "SW cache version: v$SW_VER"
else
    fail "SW cache version too low: v$SW_VER"
fi

# Push notification properties
for prop in vibrate requireInteraction; do
    if grep -q "$prop" "$REPO_ROOT/web/static/sw.js"; then
        ok "SW has $prop"
    else
        fail "SW missing $prop"
    fi
done

# ── 4. Demo build tag guards ─────────────────────────────────
echo ""
echo "--- Demo Tags ---"
if [ -f "$REPO_ROOT/cmd/pusk/demo.go" ]; then
    if head -1 "$REPO_ROOT/cmd/pusk/demo.go" | grep -q "^//go:build demo"; then
        ok "demo.go has //go:build demo"
    else
        fail "demo.go MISSING //go:build demo — demo code leaks into release!"
    fi
fi

if [ -f "$REPO_ROOT/cmd/pusk/demo_stub.go" ]; then
    if head -1 "$REPO_ROOT/cmd/pusk/demo_stub.go" | grep -q "^//go:build !demo"; then
        ok "demo_stub.go has //go:build !demo"
    else
        fail "demo_stub.go MISSING //go:build !demo"
    fi
fi

# ── 5. No hardcoded secrets in frontend ──────────────────────
echo ""
echo "--- Secrets ---"
if grep -rn 'monitor-bot-token' "$REPO_ROOT/web/static/" --include='*.html' --include='*.js' 2>/dev/null; then
    fail "Hardcoded bot token in frontend"
else
    ok "No hardcoded tokens in frontend"
fi

if grep -rlE '(192\.168\.[0-9]+\.[0-9]+|10\.0\.[0-9]+\.[0-9]+)' "$REPO_ROOT/web/static/" --include='*.js' --include='*.html' 2>/dev/null; then
    fail "Private IP in frontend files"
else
    ok "No private IPs in frontend"
fi

# ── 6. E2E test consistency ──────────────────────────────────
echo ""
echo "--- E2E Tests ---"
# All spec files should use BASE from env, not hardcoded
for spec in "$REPO_ROOT"/tests/e2e/*.spec.js; do
    [ -f "$spec" ] || continue
    base=$(basename "$spec")
    # Skip screencast utilities (not real tests, excluded via testIgnore)
    case "$base" in screencast*) continue ;; esac
    if grep -q "const BASE = 'https://" "$spec"; then
        fail "$base has hardcoded BASE URL"
    else
        ok "$base uses env-based BASE"
    fi
done

# screencast files must be excluded
if grep -q 'screencast' "$REPO_ROOT/tests/e2e/playwright.config.js"; then
    ok "playwright.config excludes screencast files"
else
    warn "playwright.config should exclude screencast files"
fi

# ── Summary ───────────────────────────────────────────────────
echo ""
echo "=== Summary ==="
echo "Errors:   $ERRORS"
echo "Warnings: $WARNINGS"

if [ "$ERRORS" -gt 0 ]; then
    echo "Coherence check FAILED."
    exit 1
fi
echo "Coherence check PASSED."
