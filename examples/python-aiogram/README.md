# Pusk + aiogram 3.x

## Migration from Telegram

```diff
- bot = Bot(token="YOUR_TOKEN")
+ bot = Bot(token="YOUR_TOKEN", base_url="https://your-pusk:8443/bot")
```

That's it. One line.

## Run

```bash
pip install aiogram
python bot.py
```

## How it works

1. aiogram sends requests to `base_url + /token/method`
2. Pusk receives `sendMessage`, `editMessageText`, etc.
3. Pusk delivers to PWA via WebSocket + Push
4. User clicks inline button → Pusk sends `callback_query` to your webhook
