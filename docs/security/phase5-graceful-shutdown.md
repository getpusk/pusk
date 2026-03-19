# Phase 5: Graceful Shutdown

## Issue
ARCH-04: No signal handling. SIGTERM killed process immediately — WebSocket connections hard-dropped, SQLite databases not properly flushed, deferred `orgs.Close()` never executed.

## What Was Done
- Replaced `http.ListenAndServe` with `http.Server` + `srv.Shutdown(ctx)`
- SIGTERM/SIGINT handler: 5 second grace period for in-flight requests
- `orgs.Close()` now executes on shutdown (verified in logs)

## Files Changed
- `cmd/pusk/main.go` — graceful shutdown with signal handling

## Test Results
```
systemctl stop pusk →
  "Received terminated, shutting down..."
  "Pusk stopped gracefully"
  "[org] closed: default"       ← SQLite properly closed
  "Deactivated successfully"

systemctl start pusk →
  health: {"status":"ok"}       ← no DB corruption
```

## Smoke Test
- Server starts normally after graceful stop
- All data intact (no SQLite WAL corruption)
- Health endpoint responds OK
