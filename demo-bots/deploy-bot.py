#!/usr/bin/env python3
"""Deploy bot demo for Pusk — same UI, simulated responses"""
from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import Application, CommandHandler, CallbackQueryHandler, MessageHandler, filters, ContextTypes

PUSK_URL = "https://getpusk.ru/bot"
TOKEN = "deploy-demo-token"

DEMO_PROJECT = {
    "id": "demo-001", "name": "my-portfolio", "subdomain": "portfolio",
    "stack": "Node.js (Vite)", "status": "running", "url": "https://portfolio.example.com",
    "cpu": "2.1%", "ram": "48/256 MB", "uptime": "3d 14h",
}

def project_kb(pid):
    return InlineKeyboardMarkup([
        [InlineKeyboardButton("Status", callback_data=f"nav:status:{pid}"),
         InlineKeyboardButton("Logs", callback_data=f"nav:logs:{pid}")],
        [InlineKeyboardButton("Ask AI", callback_data=f"nav:ask:{pid}"),
         InlineKeyboardButton("Restart", callback_data=f"nav:restart:{pid}")],
        [InlineKeyboardButton("Env", callback_data=f"nav:env:{pid}"),
         InlineKeyboardButton("Delete", callback_data=f"nav:delete:{pid}")],
    ])

def back_kb(pid):
    return InlineKeyboardMarkup([
        [InlineKeyboardButton("Status", callback_data=f"nav:status:{pid}"),
         InlineKeyboardButton("Logs", callback_data=f"nav:logs:{pid}")],
        [InlineKeyboardButton("Back", callback_data=f"nav:project:{pid}")],
    ])

def welcome_kb():
    return InlineKeyboardMarkup([
        [InlineKeyboardButton("Try demo", callback_data="demo:catalog")],
    ])

def demo_catalog_kb():
    return InlineKeyboardMarkup([
        [InlineKeyboardButton("Portfolio (Vite)", callback_data="demo:deploy:portfolio")],
        [InlineKeyboardButton("Todo App (React)", callback_data="demo:deploy:todo")],
        [InlineKeyboardButton("API Server (FastAPI)", callback_data="demo:deploy:api")],
        [InlineKeyboardButton("Back", callback_data="nav:start")],
    ])

async def start(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    p = DEMO_PROJECT
    await update.message.reply_text(
        f"Welcome! Your project:\n\n"
        f"[OK] **{p['name']}**\n"
        f"URL: {p['url']}\n"
        f"Stack: {p['stack']}\n\n"
        f"Send ZIP or GitHub link to update.",
        reply_markup=project_kb(p["id"]), parse_mode="Markdown"
    )

async def button(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    q = update.callback_query
    await q.answer()
    data = q.data
    p = DEMO_PROJECT
    pid = p["id"]

    if data == "nav:start":
        await q.edit_message_text(
            f"[OK] **{p['name']}**\n"
            f"URL: {p['url']}\n"
            f"Stack: {p['stack']}",
            reply_markup=project_kb(pid), parse_mode="Markdown"
        )

    elif data == "demo:catalog":
        await q.edit_message_text(
            "**Demo templates:**\n\n"
            "Deploy a ready-made project in seconds.",
            reply_markup=demo_catalog_kb(), parse_mode="Markdown"
        )

    elif data.startswith("demo:deploy:"):
        tmpl = data.split(":")[-1]
        names = {"portfolio": "my-portfolio", "todo": "todo-app", "api": "api-server"}
        stacks = {"portfolio": "Node.js (Vite)", "todo": "React", "api": "Python (FastAPI)"}
        name = names.get(tmpl, tmpl)
        stack = stacks.get(tmpl, "Unknown")
        await q.edit_message_text(
            f"**Deploying** `{name}`...\n\n"
            f"Stack: {stack}\n"
            f"Building... OK\n"
            f"Container started\n"
            f"HTTPS configured\n\n"
            f"[OK] Live at https://{name}.example.com\n"
            f"Deploy time: 12 sec",
            reply_markup=project_kb(pid), parse_mode="Markdown"
        )

    elif data.startswith("nav:status:"):
        await q.edit_message_text(
            f"**Status: {p['name']}**\n\n"
            f"Status: running\n"
            f"CPU: {p['cpu']}\n"
            f"RAM: {p['ram']}\n"
            f"Uptime: {p['uptime']}\n"
            f"URL: {p['url']}",
            reply_markup=back_kb(pid), parse_mode="Markdown"
        )

    elif data.startswith("nav:logs:"):
        await q.edit_message_text(
            f"**Logs: {p['name']}**\n\n"
            "```\n"
            "[14:30:01] Server started on :3000\n"
            "[14:30:02] Connected to database\n"
            "[14:31:15] GET / 200 12ms\n"
            "[14:31:16] GET /api/health 200 2ms\n"
            "[14:32:00] GET /assets/main.js 200 5ms\n"
            "```",
            reply_markup=back_kb(pid), parse_mode="Markdown"
        )

    elif data.startswith("nav:ask:"):
        await q.edit_message_text(
            f"**AI Diagnostics: {p['name']}**\n\n"
            "Everything looks healthy!\n\n"
            "CPU usage is low (2.1%), RAM within limits.\n"
            "No errors in logs. Application responds to health checks.\n\n"
            "Recommendation: consider adding a /health endpoint for monitoring.",
            reply_markup=back_kb(pid), parse_mode="Markdown"
        )

    elif data.startswith("nav:restart:"):
        await q.edit_message_text(
            f"**Restarting** `{p['name']}`...\n\n"
            f"Container stopped\n"
            f"Container started\n"
            f"Health check: OK\n\n"
            f"[OK] Restart complete (3 sec)",
            reply_markup=back_kb(pid), parse_mode="Markdown"
        )

    elif data.startswith("nav:env:"):
        await q.edit_message_text(
            f"**Env: {p['name']}**\n\n"
            "```\n"
            "NODE_ENV=production\n"
            "PORT=3000\n"
            "DATABASE_URL=***\n"
            "```\n\n"
            "Send `KEY=VALUE` to add/update.\n"
            "Send `-KEY` to remove.",
            reply_markup=back_kb(pid), parse_mode="Markdown"
        )

    elif data.startswith("nav:delete:"):
        await q.edit_message_text(
            f"**Deleted** `{p['name']}`\n\n"
            f"Container removed.\n"
            f"DNS entry cleaned.\n\n"
            "Deploy a new project or try a demo!",
            reply_markup=welcome_kb(), parse_mode="Markdown"
        )

    elif data.startswith("nav:project:"):
        await q.edit_message_text(
            f"[OK] **{p['name']}**\n"
            f"URL: {p['url']}\n"
            f"Stack: {p['stack']}",
            reply_markup=project_kb(pid), parse_mode="Markdown"
        )

async def error_handler(update, context):
    import traceback
    print(f"ERROR: {context.error}")
    traceback.print_exception(type(context.error), context.error, context.error.__traceback__)

def main():
    import logging
    logging.basicConfig(level=logging.DEBUG, format="%(asctime)s %(name)s %(levelname)s %(message)s")
    app = Application.builder().token(TOKEN).base_url(PUSK_URL).base_file_url(PUSK_URL).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(CallbackQueryHandler(button))
    app.add_error_handler(error_handler)
    print("Deploy Deploy demo polling Pusk...")
    app.run_polling()

if __name__ == "__main__":
    main()
