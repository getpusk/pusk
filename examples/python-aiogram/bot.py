"""
Pusk bot example using aiogram 3.x
Migration from Telegram: change 1 line (base_url)
"""
import asyncio
from aiogram import Bot, Dispatcher, types
from aiogram.filters import Command

# === CHANGE THIS LINE TO MIGRATE FROM TELEGRAM ===
# Before (Telegram): bot = Bot(token="YOUR_TOKEN")
# After  (Pusk):
bot = Bot(token="YOUR_BOT_TOKEN", base_url="https://your-pusk-server:8443/bot")

dp = Dispatcher()


@dp.message(Command("start"))
async def start(message: types.Message):
    kb = types.InlineKeyboardMarkup(inline_keyboard=[
        [
            types.InlineKeyboardButton(text="Status", callback_data="status"),
            types.InlineKeyboardButton(text="Help", callback_data="help"),
        ]
    ])
    await message.answer("Hello from Pusk bot!", reply_markup=kb)


@dp.callback_query()
async def callback(query: types.CallbackQuery):
    await query.message.edit_text(f"You pressed: {query.data}")
    await query.answer()


async def main():
    # Set webhook so Pusk sends updates to this bot
    await bot.set_webhook("http://localhost:3001/webhook")
    # Or use polling (not supported yet, use webhook)
    print("Bot started. Webhook set.")


if __name__ == "__main__":
    asyncio.run(main())
