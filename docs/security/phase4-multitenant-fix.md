# Phase 4: Multi-tenant Store in Async Functions

## Issue
AP-06: forwardToBot, forwardCallback, pushMessageToChat, pushEditToChat, serveFile all used the default org store instead of the org-specific store. Messages from non-default orgs would fail to relay.

## What Was Done
- `forwardToBot(s *store.Store, ...)` — receives store from caller
- `forwardCallback(s *store.Store, ...)` — receives store from caller
- `pushMessageToChat(s *store.Store, ...)` — receives store from handler
- `pushEditToChat(s *store.Store, ...)` — receives store from handler
- `serveFile` — resolves org store from JWT via `storeForJWT()`
- Handler now holds `*auth.JWTService` for JWT parsing in file endpoint

## Files Changed
- `internal/api/client.go` — pass `a.db(r)` to forwardToBot/forwardCallback
- `internal/bot/handler.go` — pass `h.db(r)` to push functions, add storeForJWT
- `cmd/pusk/main.go` — pass jwtSvc to NewHandler

## Test Results
```
Create org testorg:     OK
List bots (testorg):    1 bot (system bot)
Start chat:             Chat ID 1
Send message:           Stored in testorg DB
Messages:               2 (welcome + test)
Default org:            2 bots (DemoBot, MonitorBot) — isolated
```

## Verification
- Messages in testorg are stored in `data/orgs/testorg/pusk.db`
- Messages in default are stored in `data/orgs/default/pusk.db`
- No cross-tenant data leakage
