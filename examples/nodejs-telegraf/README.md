# Pusk + Telegraf 4.x (Node.js)

## Migration from Telegram

```diff
  const bot = new Telegraf(TOKEN);
+ bot.telegram.options.apiRoot = 'https://your-pusk:8443';
```

One line. That's it.

## Run

```bash
npm install telegraf
node bot.js
```
