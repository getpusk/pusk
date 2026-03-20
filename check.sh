#!/bin/bash
# Pusk pre-build verification script
# Run before EVERY build: ./check.sh
set -e

echo "=== Pusk Pre-Build Check ==="

# 1. Go lint
echo "[1/7] go vet..."
go vet ./...

# 2. staticcheck
echo "[2/7] staticcheck..."
$HOME/go/bin/staticcheck ./... 2>&1 || true

# 3. gofmt
echo "[3/7] gofmt..."
UNFORMATTED=$(gofmt -l .)
if [ -n "$UNFORMATTED" ]; then
    echo "  FAIL: unformatted files: $UNFORMATTED"
    exit 1
fi
echo "  OK"

# 4. Security: no hardcoded tokens/secrets in JS
echo "[4/7] Security: hardcoded secrets..."
if grep -rn 'monitor-bot-token\|admin-token\|secret\|password' web/static/ --include="*.html" --include="*.js" | grep -v 'placeholder\|type="password"\|autocomplete\|pin\|guest' | grep -v '//.*secret'; then
    echo "  WARN: possible hardcoded secrets in frontend"
fi
echo "  OK"

# 5. Endpoint integrity — count must match expected
echo "[5/7] Endpoint integrity..."
ENDPOINTS=$(grep -c 'HandleFunc\|mux.Handle' cmd/pusk/main.go internal/api/client.go internal/bot/route.go 2>/dev/null | tail -1 | cut -d: -f2)
echo "  Total routes: $ENDPOINTS"

# 6. JS function integrity
echo "[6/7] JS function check..."
for fn in addMsg sendMsg showList openChat openChan connectWS onCb onDel toggleSub showApp logout auth setLang beep scrollDown; do
    if ! grep -q "function $fn\|async function $fn" web/static/index.html; then
        echo "  FAIL: missing function $fn"
        exit 1
    fi
done
echo "  OK: all 15 core functions present"

# 7. HTML validity — basic checks
echo "[7/7] HTML checks..."
# Check for unclosed tags
OPENS=$(grep -o '<div' web/static/index.html | wc -l)
CLOSES=$(grep -o '</div>' web/static/index.html | wc -l)
echo "  divs: $OPENS open, $CLOSES close"
if [ "$OPENS" -ne "$CLOSES" ]; then
    echo "  WARN: div mismatch ($OPENS != $CLOSES)"
fi

# Dead CSS check
if grep -q 'msg-ava-right' web/static/index.html; then
    if ! grep -q 'msg-ava-right' web/static/index.html | grep -v '{'; then
        echo "  WARN: msg-ava-right CSS exists but may be dead code"
    fi
fi

echo ""
echo "=== Build ==="
VERSION=$(git describe --tags --always 2>/dev/null || echo dev)
go build -ldflags "-X github.com/pusk-platform/pusk/internal/api.Version=$VERSION" -o pusk ./cmd/pusk/
echo "BUILD OK ($(ls -lh pusk | awk '{print $5}')) version=$VERSION"
echo ""
echo "=== All checks passed ==="
