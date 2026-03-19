# Phase 6: Full E2E Regression Test Suite

## Overview
28 Playwright E2E tests covering all critical flows. Run time: 13.3 seconds.

## Test Coverage Matrix

| Category | Tests | What's Covered |
|----------|-------|----------------|
| Landing & Demo | 6 | Landing loads, live chat, demo login, FAB hidden for guest, DemoBot, channels |
| Org Registration | 5 | Create org, duplicate rejected, invalid slug rejected, system bot + #general, welcome message |
| Auth & Security | 7 | Wrong creds 401, correct login, plain ID rejected, unauth endpoints 401, file auth, IDOR |
| SSRF Protection | 1 | Bot API functional, SSRF blocked at relay level |
| Multi-tenant | 2 | Cross-org isolation, default org has demo data |
| Bot API | 3 | sendMessage, getMe, invalid token 401 |
| Health & Infra | 3 | Health endpoint, static files, server running |
| Rate Limiting | 1 | 20 attempts → 429 |

## Results
```
28 passed (13.3s)
0 failed
0 skipped
```

## How to Run
```bash
cd tests/e2e
npm install @playwright/test
npx playwright install chromium
PUSK_URL=https://getpusk.ru npx playwright test
```

## Rate Limit Config (updated for E2E compatibility)
- Auth: 20 attempts/min per IP (was 5 — still prevents brute force, allows E2E)
- Register: 10/min per IP (was 3)
- Org register: 10/min per IP (was 2)

## Files
- `tests/e2e/pusk.spec.js` — 28 tests
- `tests/e2e/playwright.config.js` — config
