# Phase 3: WebSocket Origin Check

## Issue
SEC-01: WebSocket upgraders accepted connections from any Origin, enabling Cross-Site WebSocket Hijacking (CSWSH).

## What Was Done
- Client WebSocket (`/api/ws`) now validates Origin header:
  - Same-host origin → allowed
  - localhost/127.0.0.1 → allowed (dev)
  - No Origin header → allowed (non-browser clients)
  - Other origins → rejected (403 Forbidden)
- Bot relay WebSocket (`/bot/{token}/relay`) keeps `CheckOrigin: true` — bots are non-browser clients authenticated by token

## Files Changed
- `internal/api/client.go` — `checkWSOrigin` function replaces `return true`

## Test Results
- curl cannot complete WebSocket handshake (expected 400)
- Origin validation logic verified by code review
- Full E2E test with Playwright planned in Phase 6

## Smoke Test
- Guest login: OK
- All API endpoints: functional
- PWA WebSocket connection: functional (same-origin)
