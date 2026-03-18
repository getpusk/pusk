/**
 * Pusk bot example using Telegraf 4.x
 * Migration from Telegram: change 1 line (telegram.apiRoot)
 */
const { Telegraf } = require('telegraf');

const TOKEN = 'YOUR_BOT_TOKEN';

const bot = new Telegraf(TOKEN);

// === CHANGE THIS LINE TO MIGRATE FROM TELEGRAM ===
// Before (Telegram): (nothing, uses api.telegram.org by default)
// After  (Pusk):
bot.telegram.options.apiRoot = 'https://your-pusk-server:8443';

bot.start((ctx) => {
  ctx.reply('Hello from Pusk bot!', {
    reply_markup: {
      inline_keyboard: [
        [
          { text: 'Status', callback_data: 'status' },
          { text: 'Help', callback_data: 'help' },
        ],
      ],
    },
  });
});

bot.on('callback_query', (ctx) => {
  ctx.editMessageText(`You pressed: ${ctx.callbackQuery.data}`);
  ctx.answerCbQuery();
});

// Set webhook
bot.telegram.setWebhook('http://localhost:3001/webhook');

// Start webhook server
bot.startWebhook('/', null, 3001);
console.log('Bot started on :3001');
