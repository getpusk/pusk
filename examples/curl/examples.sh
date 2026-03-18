#!/bin/bash
# Pusk Bot API — curl examples
# Replace YOUR_SERVER and YOUR_TOKEN

SERVER="https://your-pusk-server:8443"
TOKEN="your-bot-token"

# Register bot (admin)
curl -X POST $SERVER/admin/bots \
  -H "Content-Type: application/json" \
  -d '{"token":"'$TOKEN'","name":"MyBot"}'

# Set webhook
curl -X POST $SERVER/bot/$TOKEN/setWebhook \
  -H "Content-Type: application/json" \
  -d '{"url":"http://my-bot:3000/webhook"}'

# Send message
curl -X POST $SERVER/bot/$TOKEN/sendMessage \
  -H "Content-Type: application/json" \
  -d '{"chat_id":1,"text":"Hello!"}'

# Send message with inline keyboard
curl -X POST $SERVER/bot/$TOKEN/sendMessage \
  -H "Content-Type: application/json" \
  -d '{"chat_id":1,"text":"Choose:","reply_markup":{"inline_keyboard":[[{"text":"OK","callback_data":"ok"}]]}}'

# Edit message
curl -X POST $SERVER/bot/$TOKEN/editMessageText \
  -H "Content-Type: application/json" \
  -d '{"chat_id":1,"message_id":1,"text":"Updated!"}'

# Send to channel
curl -X POST $SERVER/bot/$TOKEN/sendChannel \
  -H "Content-Type: application/json" \
  -d '{"channel":"alerts","text":"Server down!"}'

# Create channel
curl -X POST $SERVER/bot/$TOKEN/createChannel \
  -H "Content-Type: application/json" \
  -d '{"name":"alerts","description":"Server alerts"}'

# Send photo
curl -X POST $SERVER/bot/$TOKEN/sendPhoto \
  -F "chat_id=1" \
  -F "photo=@screenshot.png" \
  -F "caption=Dashboard"

# Health check
curl $SERVER/api/health
