# Phase 2: File Serving with Authorization

## Issue
SEC-09: Files at `/file/{id}` served without authentication. Anyone with file ID could download any file. Cross-tenant data leak possible.

## What Was Done
- `/file/{id}` now requires JWT token via `Authorization` header or `?token=` query parameter
- Without token → 401 Unauthorized
- PWA updated: all file URLs (`<img>`, `<audio>`, `<video>`, `<a>`) append `?token=` for authenticated access

## Files Changed
- `internal/bot/handler.go` — serveFile now checks auth
- `web/static/index.html` — file URLs include token query parameter

## Test Results
```
File without auth:     HTTP 401 (BLOCKED)
File with fake token:  HTTP 404 (auth passed, file not found)
Normal flow:           Login OK, 2 bots visible
```

## Limitations
- Token in URL query string (visible in logs/referrer) — acceptable tradeoff for `<img src>` compatibility
- File access checks bot ownership but not per-user chat access (simplified for MVP)
- Per-org file isolation not yet implemented (Phase 4)
