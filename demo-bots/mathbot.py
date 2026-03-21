#!/usr/bin/env python3
"""MathBot — simple math quiz bot for Pusk demo"""
import random
from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import Application, CommandHandler, CallbackQueryHandler, MessageHandler, filters, ContextTypes

PUSK_URL = "https://getpusk.ru/bot"
TOKEN = "math-bot-token"

def gen_problem(level):
    if level == "easy":
        a, b = random.randint(1, 20), random.randint(1, 20)
        op = random.choice(["+", "-"])
    elif level == "medium":
        a, b = random.randint(10, 100), random.randint(2, 12)
        op = random.choice(["+", "-", "*"])
    else:
        a, b = random.randint(10, 200), random.randint(2, 20)
        op = random.choice(["+", "-", "*"])
    answer = eval(f"{a}{op}{b}")
    return f"{a} {op} {b} = ?", int(answer)

problems = {}

async def start(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    kb = InlineKeyboardMarkup([
        [InlineKeyboardButton("Easy", callback_data="level_easy"),
         InlineKeyboardButton("Medium", callback_data="level_medium"),
         InlineKeyboardButton("Hard", callback_data="level_hard")],
    ])
    await update.message.reply_text("**MathBot**\n\nChoose difficulty:", reply_markup=kb, parse_mode="Markdown")

async def button(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    q = update.callback_query
    await q.answer()
    data = q.data
    chat_id = q.message.chat_id

    if data.startswith("level_"):
        level = data.replace("level_", "")
        problem, answer = gen_problem(level)
        problems[chat_id] = {"answer": answer, "level": level, "score": problems.get(chat_id, {}).get("score", 0)}
        kb = InlineKeyboardMarkup([[InlineKeyboardButton("Skip", callback_data="skip")]])
        await q.edit_message_text(f"**{problem}**\n\nType the answer:", reply_markup=kb, parse_mode="Markdown")

    elif data == "skip":
        info = problems.get(chat_id, {})
        answer = info.get("answer", "?")
        level = info.get("level", "easy")
        await q.edit_message_text(f"Answer was: **{answer}**", parse_mode="Markdown")
        problem, new_answer = gen_problem(level)
        problems[chat_id] = {"answer": new_answer, "level": level, "score": info.get("score", 0)}
        kb = InlineKeyboardMarkup([[InlineKeyboardButton("Skip", callback_data="skip")]])
        await ctx.bot.send_message(chat_id, f"**{problem}**\n\nType the answer:", reply_markup=kb, parse_mode="Markdown")

    elif data == "next":
        info = problems.get(chat_id, {})
        level = info.get("level", "easy")
        problem, answer = gen_problem(level)
        problems[chat_id] = {"answer": answer, "level": level, "score": info.get("score", 0)}
        kb = InlineKeyboardMarkup([[InlineKeyboardButton("Skip", callback_data="skip")]])
        await q.edit_message_text(f"**{problem}**\n\nType the answer:", reply_markup=kb, parse_mode="Markdown")

async def check_answer(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    chat_id = update.message.chat_id
    info = problems.get(chat_id)
    if not info or "answer" not in info:
        return
    try:
        user_answer = int(update.message.text.strip())
    except ValueError:
        return
    correct = info["answer"]
    level = info.get("level", "easy")
    score = info.get("score", 0)
    if user_answer == correct:
        score += 1
        problems[chat_id]["score"] = score
        kb = InlineKeyboardMarkup([[InlineKeyboardButton("Next", callback_data="next"), InlineKeyboardButton("Menu", callback_data="menu")]])
        await update.message.reply_text(f"Correct! Score: **{score}**", reply_markup=kb, parse_mode="Markdown")
    else:
        kb = InlineKeyboardMarkup([[InlineKeyboardButton("Try again", callback_data=f"level_{level}"), InlineKeyboardButton("Skip", callback_data="skip")]])
        await update.message.reply_text(f"Wrong. Try again or skip.", reply_markup=kb, parse_mode="Markdown")

async def menu_cb(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    q = update.callback_query
    await q.answer()
    if q.data == "menu":
        kb = InlineKeyboardMarkup([
            [InlineKeyboardButton("Easy", callback_data="level_easy"),
             InlineKeyboardButton("Medium", callback_data="level_medium"),
             InlineKeyboardButton("Hard", callback_data="level_hard")],
        ])
        info = problems.get(q.message.chat_id, {})
        await q.edit_message_text(f"**MathBot** | Score: {info.get('score', 0)}\n\nChoose difficulty:", reply_markup=kb, parse_mode="Markdown")

def main():
    app = Application.builder().token(TOKEN).base_url(PUSK_URL).base_file_url(PUSK_URL).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(CallbackQueryHandler(menu_cb, pattern="^menu$"))
    app.add_handler(CallbackQueryHandler(button))
    app.add_handler(MessageHandler(filters.TEXT & ~filters.COMMAND, check_answer))
    print("MathBot polling Pusk...")
    app.run_polling(drop_pending_updates=True)

if __name__ == "__main__":
    main()
