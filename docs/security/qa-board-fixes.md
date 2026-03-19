# QA Board: 6 Critical Fixes

## Panel
- Rob Pike (Go idioms), Charity Majors (observability), Tanya Janca (AppSec), Dmitry Vyukov (concurrency)

## Fixes Applied

### Fix 1: deleteMessage ownership check
- **Before:** Any user could delete any message by ID
- **After:** Verify user owns the chat → 403 if not

### Fix 2: Auth required on all data endpoints
- **Before:** listBots, listChannels, listChats, startChat, subscribe, unsubscribe, channelMessages worked with userID=0
- **After:** `requireAuth()` guard returns 401 for unauthenticated requests
- Added `requireAuth` helper used consistently across all endpoints

### Fix 3: WebSocket origin — exact host match
- **Before:** `strings.Contains(origin, host)` — `evil-getpusk.ru` passed
- **After:** Parse origin URL, extract hostname, exact comparison with request host

### Fix 4: Health version from ldflags
- **Before:** Hardcoded `"0.3.1"`
- **After:** `var Version = "dev"`, settable via `-ldflags "-X .../api.Version=0.5.0"`

### Fix 5: Rate limiter logging
- **Before:** Silent 429, no visibility
- **After:** `[ratelimit] blocked 89.47.126.77 on /api/auth` in logs

### Fix 6: SSRF — URL parsing instead of string matching
- **Before:** `strings.Contains(lower, "10.")` blocked `api.tenant10.com`
- **After:** Parse URL → resolve hostname → check `ip.IsLoopback()`, `ip.IsPrivate()`, `ip.IsLinkLocalUnicast()`

## Test Results
```
Unauth listBots:     401 (was 200) ✓
Unauth listChannels: 401 (was 200) ✓
Auth listBots:       200 ✓
File without auth:   401 ✓
Health version:      "dev" (not "0.3.1") ✓
Landing:             200 ✓
Create org:          True ✓
Rate limit log:      "[ratelimit] blocked IP on /api/auth" ✓
```
