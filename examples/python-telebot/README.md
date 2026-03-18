# Pusk + python-telegram-bot 21.x

## Migration from Telegram

```diff
  app = (
      Application.builder()
      .token(TOKEN)
+     .base_url("https://your-pusk:8443/bot")
+     .base_file_url("https://your-pusk:8443/bot")
      .build()
  )
```

Two lines. Everything else stays the same.

## Run

```bash
pip install python-telegram-bot
python bot.py
```
