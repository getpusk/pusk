# Phase 1: Rate Limiting on Auth Endpoints

## Issue
SEC-08: No rate limiting on authentication endpoints. PIN brute force trivial (10000 variants for 4-digit PIN = 10 seconds at 1000 req/s).

## What Was Done
- Added `internal/api/ratelimit.go` — in-memory token bucket per IP
- Rate limits applied to:
  - `POST /api/auth` — 5 attempts per minute per IP
  - `POST /api/register` — 3 registrations per minute per IP
  - `POST /api/org/register` — 2 org registrations per minute per IP
- Stale buckets cleaned up every 5 minutes
- 429 Too Many Requests response with Retry-After header

## Architecture
Token bucket algorithm. Per-IP tracking using `X-Forwarded-For` header (for Caddy/nginx) with fallback to `RemoteAddr`. In-memory (no Redis/external deps).

## Files Changed
- `internal/api/ratelimit.go` — NEW (RateLimiter + RateLimit middleware)
- `internal/api/client.go` — auth/register routes wrapped with RateLimit
- `cmd/pusk/main.go` — org/register wrapped with RateLimit

## Test Results
```
Attempt 1: HTTP 401 (wrong creds)
Attempt 2: HTTP 401
Attempt 3: HTTP 401
Attempt 4: HTTP 401
Attempt 5: HTTP 401
Attempt 6: HTTP 429 (rate limited)
Attempt 7: HTTP 429
```

## Smoke Test
- Guest login: works (when not rate-limited)
- List bots: OK
- List channels: OK
- Start chat: OK
- Create org: OK
- Landing page: HTTP 200

## Limitations
- In-memory: rate limits reset on server restart
- Per-IP only: shared IPs (corporate NAT) affect all users behind same IP
- No per-username limiting (attacker can rotate IPs)

## Next Steps
- Consider per-username + per-IP combined limiting
- Consider persistent rate limit state (SQLite table)
