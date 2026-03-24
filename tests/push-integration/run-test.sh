#!/bin/bash
# Full integration test for Push notifications
# Uses a fake push receiver instead of real FCM/Mozilla
set -e

PUSK_URL="${1:-http://localhost:8443}"
RECEIVER_PORT=9876
RECEIVER_URL="http://localhost:$RECEIVER_PORT/push-receive"

export PATH=$PATH:/usr/local/go/bin
cd /srv/projects/pusk

echo "============================================"
echo "PUSH NOTIFICATION INTEGRATION TEST"
echo "============================================"
echo "Pusk: $PUSK_URL"
echo "Receiver: $RECEIVER_URL"
echo ""

# 1. Start fake push receiver
echo "[1/7] Starting fake push receiver..."
cd tests/push-integration
go run receiver.go $RECEIVER_PORT &
RECV_PID=$!
sleep 2
curl -sf http://localhost:$RECEIVER_PORT/push-results > /dev/null || { echo "FAIL: receiver not started"; kill $RECV_PID 2>/dev/null; exit 1; }
echo "  Receiver PID: $RECV_PID"

# 2. Create test org with clean state
echo "[2/7] Creating test org..."
cd /srv/projects/pusk
SLUG="push-test-$(date +%s)"
REG=$(curl -sf -X POST "$PUSK_URL/api/org/register" -H 'Content-Type: application/json' \
  -d "{\"slug\":\"$SLUG\",\"name\":\"PushTest\",\"username\":\"admin\",\"pin\":\"pushtest1\"}")
TOKEN=$(echo "$REG" | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
echo "  Org: $SLUG, Token: ${TOKEN:0:20}..."

# 3. Get bot token for webhooks
echo "[3/7] Getting bot token..."
BOTS=$(curl -sf "$PUSK_URL/api/bots" -H "Authorization: $TOKEN")
BOT_TOKEN=$(echo "$BOTS" | python3 -c "import sys,json;bs=json.load(sys.stdin);print(bs[0]['token'] if bs else '')")
echo "  Bot token: ${BOT_TOKEN:0:15}..."

# 4. Insert fake push subscription pointing to our receiver
echo "[4/7] Injecting fake push subscription..."
# We need to add a subscription via the API with our receiver URL as endpoint
# The API expects: endpoint, keys.p256dh, keys.auth
# We use dummy keys — the server will try to encrypt with them,
# and our receiver will get encrypted data (can't decrypt, but proves delivery)
curl -sf -X POST "$PUSK_URL/api/push/subscribe" \
  -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"endpoint\":\"$RECEIVER_URL\",\"keys\":{\"p256dh\":\"BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8p8eMaPXk\",\"auth\":\"tBHItJI5svbpC7F8pK_esA\"}}"
echo "  Subscription injected"

# 5. Reset receiver counter
curl -sf http://localhost:$RECEIVER_PORT/push-reset > /dev/null

# 6. Subscribe to alerts channel and send test webhook
echo "[5/7] Creating #alerts channel and subscribing..."
curl -sf -X POST "$PUSK_URL/admin/channel" \
  -H "Authorization: $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"push-test-alerts","description":"Push test"}' > /dev/null
CHANNELS=$(curl -sf "$PUSK_URL/api/channels" -H "Authorization: $TOKEN")
CH_ID=$(echo "$CHANNELS" | python3 -c "import sys,json;chs=json.load(sys.stdin);print(next((c['id'] for c in chs if 'push-test' in c['name']),0))")
curl -sf -X POST "$PUSK_URL/api/channels/$CH_ID/subscribe" -H "Authorization: $TOKEN" > /dev/null
echo "  Channel ID: $CH_ID, subscribed"

echo "[6/7] Sending webhook alert..."
curl -sf -X POST "$PUSK_URL/hook/$BOT_TOKEN?format=alertmanager&channel=push-test-alerts" \
  -H 'Content-Type: application/json' \
  -d '{"status":"firing","alerts":[{"status":"firing","labels":{"alertname":"PushIntegrationTest","severity":"critical"},"annotations":{"summary":"Integration test push"}}]}' > /dev/null
echo "  Webhook sent"

# Wait for async push delivery
sleep 3

# 7. Check results
echo "[7/7] Checking push delivery..."
RESULTS=$(curl -sf http://localhost:$RECEIVER_PORT/push-results)
TOTAL=$(echo "$RESULTS" | python3 -c "import sys,json;print(json.load(sys.stdin)['total'])")
echo ""
echo "============================================"
echo "RESULTS"
echo "============================================"
echo "Push notifications received: $TOTAL"
echo ""

if [ "$TOTAL" -ge 1 ]; then
    echo "DETAILS:"
    echo "$RESULTS" | python3 -c "
import sys,json
data = json.load(sys.stdin)
for i, r in enumerate(data['records']):
    print(f'  Push #{i+1}:')
    print(f'    Method: {r[\"method\"]}')
    print(f'    Body size: {r[\"body_len\"]} bytes')
    print(f'    Content-Encoding: {r[\"headers\"].get(\"Content-Encoding\", \"none\")}')
    print(f'    TTL: {r[\"headers\"].get(\"Ttl\", \"none\")}')
    print(f'    Authorization: {r[\"headers\"].get(\"Authorization\", \"none\")[:40]}...')
    print()
"
    echo "✅ PUSH INTEGRATION TEST PASSED"
    echo "  - Webhook received by Pusk"
    echo "  - Message saved to channel"
    echo "  - Push notification sent to subscriber"
    echo "  - Fake receiver got the encrypted payload"
else
    echo "❌ PUSH INTEGRATION TEST FAILED"
    echo "  - No push received by fake receiver"
    echo "  - Check: is push subscription stored?"
    echo "  - Check: are VAPID keys configured?"

    # Debug
    echo ""
    echo "DEBUG: Test push via API..."
    TEST=$(curl -sf -X POST "$PUSK_URL/api/push/test" -H "Authorization: $TOKEN")
    echo "  Test push response: $TEST"
fi

# Cleanup
kill $RECV_PID 2>/dev/null
echo ""
echo "Receiver stopped. Test complete."
