# Contract registry gate

A machine-checkable registry of PUSK's backend/security invariants
(`contracts.json`) plus a deterministic gate (`pusk_contract_gate.py`) that
ratifies it against the working tree. Where `scripts/lint-pusk.sh` and
`scripts/coherence-check.sh` guard frontend integrity, version drift and build
tags, this guards **authz / multi-tenant isolation / SSRF / fail-closed /
write-atomicity / SQL-injection** invariants that standard linters miss.

The human authors the registry; the gate ratifies. `enforced_by=NONE` is a
**visible** gap, never silent.

## Teeth (each → RED / exit 1)
- **DRIFT** — a contract's `code_anchor` (`relpath:Symbol`) no longer exists
  (identifier-aware: a rename that keeps the old name as a prefix is caught).
- **REGRESSION** — a contract marked OK/PARTIAL fails its live mechanical check
  (a guard removed, or the guarded function renamed away).
- **NEW BLIND SPOT** — a new `enforced_by=NONE` contract not in `gap-baseline`
  and not `accepted`.

Known gaps (`status=GAP`) are listed loudly but tolerated while baselined.
Closing a gap in code flips its check to HOLDS and the gate prints
"declared GAP but now HOLDS — upgrade to OK".

## Run
```bash
make contracts                                   # or:
python3 scripts/contracts/pusk_contract_gate.py  # exit 1 = RED
python3 scripts/contracts/pusk_contract_gate.py --update-baseline  # accept open-gap set
```
Wired into `make check` and the CI build job. Set `PUSK_CODE_ROOT` to ratify a
tree other than the repo root.

## Live checks
Most contracts carry a data-driven `live_check`
(`{file, fn_sig, must_contain[], must_not_contain[]}`); a few use a named
`live_check_fn` (custom scan) in the gate. To add a contract: append an entry to
`contracts.json` with a real `code_anchor` and a `live_check` that fails when the
invariant regresses, then run the gate.
