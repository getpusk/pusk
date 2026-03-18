"""
Pusk bot example using python-telegram-bot 21.x
Migration from Telegram: change 1 line (base_url)
"""
from telegram import InlineKeyboardButton, InlineKeyboardMarkup, Update
from telegram.ext import Application, CommandHandler, CallbackQueryHandler, ContextTypes

# === CHANGE THIS LINE TO MIGRATE FROM TELEGRAM ===
TOKEN = "YOUR_BOT_TOKEN"
PUSK_URL = "https://your-pusk-server:8443/bot"

# Before (Telegram): app = Application.builder().token(TOKEN).build()
# After  (Pusk):
app = (
    Application.builder()
    .token(TOKEN)
    .base_url(PUSK_URL)
    .base_file_url(PUSK_URL)
    .build()
)


async def start(update: Update, context: ContextTypes.DEFAULT_TYPE):
    keyboard = [
        [
            InlineKeyboardButton("Status", callback_data="status"),
            InlineKeyboardButton("Help", callback_data="help"),
        ]
    ]
    await update.message.reply_text(
        "Hello from Pusk bot!",
        reply_markup=InlineKeyboardMarkup(keyboard),
    )


async def button(update: Update, context: ContextTypes.DEFAULT_TYPE):
    query = update.callback_query
    await query.answer()
    await query.edit_message_text(f"You pressed: {query.data}")


app.add_handler(CommandHandler("start", start))
app.add_handler(CallbackQueryHandler(button))

if __name__ == "__main__":
    print("Bot started")
    app.run_webhook(listen="0.0.0.0", port=3001, webhook_url="http://localhost:3001")
