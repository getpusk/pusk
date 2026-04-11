#!/bin/bash
# lint-pusk.sh — domain-specific linter for Pusk
# Catches bugs that standard linters miss.
# Runs on staged files (--staged) or all files (default).
# Exit code: 1 if ERRORs found, 0 if only WARNINGs or clean.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

STAGED=false
[ "${1:-}" = "--staged" ] && STAGED=true

ERRORS=0
WARNINGS=0

err()  { echo "  ERROR: $1"; ERRORS=$((ERRORS + 1)); }
warn() { echo "  WARN:  $1"; WARNINGS=$((WARNINGS + 1)); }

# File list: staged or all tracked
if $STAGED; then
    JS_FILES=$(git diff --cached --name-only --diff-filter=AM | grep 'web/static/.*\.js$' || true)
    GO_FILES=$(git diff --cached --name-only --diff-filter=AM | grep '\.go$' || true)
else
    JS_FILES=$(find web/static -name '*.js' 2>/dev/null || true)
    GO_FILES=$(find internal cmd -name '*.go' ! -name '*_test.go' 2>/dev/null || true)
fi

# ─── JS Checks ───────────────────────────────────────────────

echo "[lint-pusk] JS checks..."

# 1. Raw localStorage outside storage.js
for f in $JS_FILES; do
    [ "$(basename "$f")" = "storage.js" ] && continue
    [ ! -f "$f" ] && continue
    HITS=$(grep -n 'localStorage\.\(getItem\|setItem\|removeItem\)' "$f" | grep -v '// lint:ok' || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            err "$f:$line — use storage.js helpers (get/set/remove), not raw localStorage"
        done <<< "$HITS"
    fi
done

# 2. innerHTML without escaping (skip static HTML assignments)
for f in $JS_FILES; do
    [ ! -f "$f" ] && continue
    # Match innerHTML= with variable interpolation (${...} or concatenation with +)
    # Skip: innerHTML='' (clear), innerHTML='<static>' (no variables)
    HITS=$(grep -n '\.innerHTML\s*=' "$f" \
        | grep -v "innerHTML\s*=\s*''" \
        | grep -v "innerHTML\s*=\s*\"\"" \
        | grep -v 'esc(\|escapeHtml(\|md(' \
        | grep -v '// lint:ok' \
        | grep '\${\|+ ' || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            warn "$f:$line — innerHTML with dynamic content, verify esc()/md() usage"
        done <<< "$HITS"
    fi
done

# 3. Nested backtick templates
for f in $JS_FILES; do
    [ ! -f "$f" ] && continue
    HITS=$(grep -n '`[^`]*\${[^}]*`' "$f" | grep -v '// lint:ok' || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            err "$f:$line — nested backtick template literal (causes JS syntax errors)"
        done <<< "$HITS"
    fi
done

# ─── Go Checks ───────────────────────────────────────────────

echo "[lint-pusk] Go checks..."

# 4. SQL outside store/ package
for f in $GO_FILES; do
    [ ! -f "$f" ] && continue
    case "$f" in
        internal/store/*|internal/org/manager.go|*_test.go) continue ;;
    esac
    # Match db.Query/QueryRow/Exec calls (exclude r.URL.Query which is HTTP, not SQL)
    HITS=$(grep -n '\.\(Query\|QueryRow\|Exec\)\s*(' "$f" \
        | grep -v 'URL\.Query\|// lint:ok' || true)
    # Also catch raw SQL string literals (but not HTTP methods)
    HITS2=$(grep -n '"SELECT \|"INSERT \|"UPDATE \|"DELETE FROM\|"CREATE \|"DROP \|"ALTER ' "$f" \
        | grep -v '// lint:ok' || true)
    HITS=$(printf '%s\n%s' "$HITS" "$HITS2" | grep -v '^$' | sort -t: -k1,1n -u || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            err "$f:$line — SQL query outside store/ package"
        done <<< "$HITS"
    fi
done

# 5. fmt.Print/log.Print instead of slog
for f in $GO_FILES; do
    [ ! -f "$f" ] && continue
    case "$f" in
        *_test.go) continue ;;
    esac
    HITS=$(grep -n 'fmt\.Print\|fmt\.Fprint\|log\.Print\|log\.Fatal\|log\.Panic' "$f" \
        | grep -v 'fmt\.Fprintf(w,\|fmt\.Fprintf(&\|fmt\.Fprintf(buf' \
        | grep -v 'fmt\.Sprintf' \
        | grep -v 'fmt\.Errorf' \
        | grep -v '// lint:ok' || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            warn "$f:$line — use slog instead of fmt/log for logging"
        done <<< "$HITS"
    fi
done

# 6. Non-monotonic update_id patterns
for f in $GO_FILES; do
    [ ! -f "$f" ] && continue
    # Catch: messageID * 1000, id * 1000 (the pattern that caused the 3h debug)
    HITS=$(grep -n 'ID\s*\*\s*1000\|messageID\s*\*\s*1000' "$f" \
        | grep -v 'nextID\|UnixMilli\|atomic\|monotonic' \
        | grep -v '// lint:ok' || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            err "$f:$line — update_id must be monotonically increasing (use nextID()/UnixMilli)"
        done <<< "$HITS"
    fi
    # Warn on direct msg.ID assignment to update_id (ok for webhooks, risky for polling)
    HITS=$(grep -n '"update_id".*msg\.ID\|"update_id".*message\.ID' "$f" \
        | grep -v 'nextID\|UnixMilli\|atomic\|monotonic\|webhook' \
        | grep -v '// lint:ok' || true)
    if [ -n "$HITS" ]; then
        while IFS= read -r line; do
            warn "$f:$line — direct msg.ID as update_id; ok for webhooks, risky for polling"
        done <<< "$HITS"
    fi
done

# ─── Build Tag Guards ────────────────────────────────────────

echo "[lint-pusk] Build tag guards..."

# 7. demo.go must have //go:build demo tag
if [ -f cmd/pusk/demo.go ]; then
    if ! head -1 cmd/pusk/demo.go | grep -q "^//go:build demo"; then
        err "cmd/pusk/demo.go missing //go:build demo tag — demo code must not leak into release binary"
    fi
fi

# 8. demo_stub.go must have //go:build !demo tag
if [ -f cmd/pusk/demo_stub.go ]; then
    if ! head -1 cmd/pusk/demo_stub.go | grep -q "^//go:build !demo"; then
        err "cmd/pusk/demo_stub.go missing //go:build !demo tag"
    fi
fi

# ─── Summary ─────────────────────────────────────────────────

echo ""
if [ "$ERRORS" -gt 0 ]; then
    echo "[lint-pusk] FAIL: $ERRORS error(s), $WARNINGS warning(s)"
    exit 1
elif [ "$WARNINGS" -gt 0 ]; then
    echo "[lint-pusk] PASS with $WARNINGS warning(s)"
    exit 0
else
    echo "[lint-pusk] PASS: all clean"
    exit 0
fi
