#!/usr/bin/env python3
"""PUSK contract gate — ratifies the hand-authored contract registry against live code.

A machine-checkable registry of backend/security invariants (contracts.json) plus a
deterministic gate that ratifies it against the working tree. Blind spots are visible;
regressions are blocked. Complements the frontend/coherence linters in scripts/.

Teeth (each -> exit 1 / RED):
  - DRIFT:        a contract's code_anchor (relpath:Symbol) no longer exists.
  - REGRESSION:   a contract marked OK/PARTIAL fails its live mechanical check.
  - NEW BLIND SPOT: an enforced_by=NONE id not in gap-baseline and not accepted.
Known gaps (status GAP, enforced_by=NONE) are LISTED loudly but tolerated when accepted
or already in gap-baseline (visible, not silent).

Live checks are DATA-DRIVEN: most contracts carry a `live_check` object
  { "file": "<relpath>", "fn_sig": "<func sig prefix or omit>",
    "must_contain": [..literal substrings that MUST be present..],
    "must_not_contain": [..literal substrings whose presence = violation..] }
A few complex invariants use a NAMED custom check via "live_check_fn".

Usage:
  python3 scripts/contracts/pusk_contract_gate.py                 # check (exit 1 = RED)
  python3 scripts/contracts/pusk_contract_gate.py --update-baseline  # accept open-gap set
Set PUSK_CODE_ROOT to ratify a tree other than the repo root.
"""
import json
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.join(HERE, "contracts.json")
BASELINE = os.path.join(HERE, "gap-baseline")
# repo root = <root>/scripts/contracts/this_file
REPO_ROOT = os.path.dirname(os.path.dirname(HERE))

R = "\033[31m"; G = "\033[32m"; Y = "\033[33m"; B = "\033[1m"; X = "\033[0m"
if not sys.stdout.isatty() or os.environ.get("NO_COLOR"):
    R = G = Y = B = X = ""


def load():
    with open(REG) as f:
        return json.load(f)


def read(root, rel):
    p = rel if os.path.isabs(rel) else os.path.join(root, rel)
    try:
        with open(p, encoding="utf-8", errors="replace") as f:
            return f.read()
    except OSError:
        return None


def fn_body(text, sig):
    """Slice from a Go func signature to the next top-level `func` (heuristic).
    Returns None if the signature is absent (caller treats that as a violation when
    the file itself is readable — the guard function vanished)."""
    i = text.find(sig)
    if i < 0:
        return None
    rest = text[i + len(sig):]
    m = re.search(r"\nfunc ", rest)
    return rest[: m.start()] if m else rest[:4000]


# ── generic data-driven evaluator ───────────────────────────────────────────
def eval_spec(root, spec):
    """True=holds, False=violated, None=cannot read the file at all."""
    t = read(root, spec["file"])
    if t is None:
        return None
    scope = t
    if spec.get("fn_sig"):
        body = fn_body(t, spec["fn_sig"])
        if body is None:
            # File readable but the guarded signature is gone -> renamed/removed
            # -> the enforcement point vanished -> VIOLATION (not 'cannot read').
            return False
        scope = body
    for s in spec.get("must_contain", []):
        if s not in scope:
            return False
    for s in spec.get("must_not_contain", []):
        if s in scope:
            return False
    return True


# ── custom checks (invariants the generic evaluator can't express) ───────────
def every_protected_route_authrequired(root):
    """authz: every data route in client.go is wrapped in a.AuthRequired, except an
    explicit public allow-list. Any other /api route lacking AuthRequired -> False."""
    t = read(root, "internal/api/client.go")
    if t is None:
        return None
    public = (
        "POST /api/auth", "POST /api/register", "GET /api/health",
        "GET /api/push/vapid", "POST /api/invite/accept", "GET /api/invite/check-user",
    )
    for ln in t.splitlines():
        if 'mux.HandleFunc("' not in ln or "/api/" not in ln:
            continue
        if any(p in ln for p in public):
            continue
        if "AuthRequired" not in ln:
            return False
    return True


def store_sql_parameterized(root):
    """injection: no SQL in internal/store/ is built via fmt.Sprintf or string
    concatenation passed to a query method. Teeth: `.Query(fmt.Sprintf(...))` -> False."""
    d = os.path.join(root, "internal/store")
    if not os.path.isdir(d):
        return None
    danger = re.compile(
        r"\.(Query|QueryRow|Exec|QueryContext|QueryRowContext|ExecContext)"
        r"\(\s*(fmt\.Sprintf|`[^`]*`\s*\+|\"[^\"]*\"\s*\+)"
    )
    for fn in os.listdir(d):
        if not fn.endswith(".go") or fn.endswith("_test.go"):
            continue
        t = read(root, os.path.join("internal/store", fn))
        if t is None:
            continue
        if danger.search(t):
            return False
    return True


CUSTOM_CHECKS = {
    "every_protected_route_authrequired": every_protected_route_authrequired,
    "store_sql_parameterized": store_sql_parameterized,
}


def run_live_check(root, c):
    fn = c.get("live_check_fn")
    if fn:
        f = CUSTOM_CHECKS.get(fn)
        return f(root) if f else None
    spec = c.get("live_check")
    if spec:
        return eval_spec(root, spec)
    return None


def anchor_exists(root, anchor):
    if not anchor:
        return True
    path, _, sym = anchor.partition(":")
    full = path if os.path.isabs(path) else os.path.join(root, path)
    if not os.path.exists(full):
        return False
    if sym:
        t = read(root, path)
        if t is None:
            return False
        # Identifier-aware: a rename keeping the old name as a prefix
        # (Validate -> ValidateX) must NOT satisfy the anchor.
        return re.search(r"(?<!\w)" + re.escape(sym) + r"(?!\w)", t) is not None
    return True


def main():
    reg = load()
    root = os.environ.get("PUSK_CODE_ROOT") or REPO_ROOT
    if not os.path.isdir(root):
        print(f"{R}{B}PUSK CONTRACT GATE: RED (1){X}")
        print(f"  {R}- code root missing: {root}{X}")
        return 1
    contracts = reg["contracts"]
    red = []
    notices = []

    # 1. DRIFT — anchors must still exist
    for c in contracts:
        if not anchor_exists(root, c.get("code_anchor", "")):
            red.append(f"DRIFT: {c['id']} -> code_anchor '{c['code_anchor']}' not found")

    # 2. LIVE CHECKS — compare reality vs declared status
    print(f"{B}== live contract checks =={X}")
    for c in contracts:
        if not (c.get("live_check") or c.get("live_check_fn")):
            continue
        holds = run_live_check(root, c)
        expected_ok = c["status"] in ("OK", "PARTIAL")
        if holds is None:
            notices.append(f"SKIP {c['id']}: live check could not read code")
            print(f"  {Y}? {c['id']}: check unavailable{X}")
        elif expected_ok and not holds:
            red.append(f"REGRESSION: {c['id']} marked {c['status']} but live check FAILED")
            print(f"  {R}REGRESSION {c['id']}: declared {c['status']}, but contract does NOT hold{X}")
        elif not expected_ok and holds:
            notices.append(f"STALE: {c['id']} marked {c['status']} but contract now HOLDS — upgrade to OK")
            print(f"  {Y}^ {c['id']}: declared {c['status']} but now HOLDS — registry stale, upgrade{X}")
        elif not expected_ok and not holds:
            print(f"  {Y}GAP {c['id']}: known blind spot, confirmed (status={c['status']}){X}")
        else:
            print(f"  {G}OK {c['id']}: holds{X}")

    # 3. GAP ratchet — no NEW *open* blind spot may appear silently.
    gaps = {c["id"] for c in contracts if c.get("enforced_by") == "NONE"}
    accepted = {c["id"] for c in contracts if c.get("accepted")}
    open_gaps = gaps - accepted
    if "--update-baseline" in sys.argv:
        with open(BASELINE, "w") as f:
            f.write("\n".join(sorted(open_gaps)) + "\n")
        print(f"\n{G}baseline updated: {len(open_gaps)} open gaps ({len(gaps & accepted)} accepted){X}")
        return 0
    if os.path.exists(BASELINE):
        with open(BASELINE) as f:
            base = {l.strip() for l in f if l.strip()}
        for g in sorted(open_gaps - base):
            red.append(f"NEW OPEN BLIND SPOT: {g} (enforced_by=NONE, not accepted, not in baseline)")
    else:
        with open(BASELINE, "w") as f:
            f.write("\n".join(sorted(open_gaps)) + "\n")
        print(f"{Y}(no baseline — wrote initial with {len(open_gaps)} open gaps){X}")

    # 4. summary
    by_status = {}
    for c in contracts:
        by_status[c["status"]] = by_status.get(c["status"], 0) + 1
    print(f"\n{B}== summary =={X}")
    print(f"  contracts: {len(contracts)}  |  " + "  ".join(f"{k}={v}" for k, v in sorted(by_status.items())))
    print(f"  OPEN gaps (unhandled): {len(open_gaps)} -> {', '.join(sorted(open_gaps)) or 'none'}")
    for n in notices:
        print(f"  {Y}note: {n}{X}")

    if red:
        print(f"\n{R}{B}PUSK CONTRACT GATE: RED ({len(red)}){X}")
        for r in red:
            print(f"  {R}- {r}{X}")
        return 1
    print(f"\n{G}{B}PUSK CONTRACT GATE: GREEN — registry coherent with code, no new blind spots{X}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
