#!/usr/bin/env python3
"""DeployBot — deployment simulation bot for Pusk demo"""
import random
import time
from telegram import Update, InlineKeyboardButton, InlineKeyboardMarkup
from telegram.ext import Application, CommandHandler, CallbackQueryHandler, ContextTypes

PUSK_URL = "https://getpusk.ru/bot"
TOKEN = "deploy-bot-token"

SERVICES = {
    "api-gateway": {"version": "v2.1.0", "replicas": 3, "status": "running"},
    "web-frontend": {"version": "v1.8.2", "replicas": 2, "status": "running"},
    "worker": {"version": "v3.0.1", "replicas": 4, "status": "running"},
}

async def start(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    kb = InlineKeyboardMarkup([
        [InlineKeyboardButton("Status", callback_data="status")],
        [InlineKeyboardButton("Deploy api-gateway", callback_data="deploy_api-gateway")],
        [InlineKeyboardButton("Deploy web-frontend", callback_data="deploy_web-frontend")],
        [InlineKeyboardButton("Deploy worker", callback_data="deploy_worker")],
        [InlineKeyboardButton("Rollback", callback_data="rollback_menu")],
    ])
    await update.message.reply_text("**DeployBot**\n\nManage your deployments:", reply_markup=kb, parse_mode="Markdown")

async def button(update: Update, ctx: ContextTypes.DEFAULT_TYPE):
    q = update.callback_query
    await q.answer()
    data = q.data
    kb_back = InlineKeyboardMarkup([[InlineKeyboardButton("Back", callback_data="back")]])

    if data == "back":
        kb = InlineKeyboardMarkup([
            [InlineKeyboardButton("Status", callback_data="status")],
            [InlineKeyboardButton("Deploy api-gateway", callback_data="deploy_api-gateway")],
            [InlineKeyboardButton("Deploy web-frontend", callback_data="deploy_web-frontend")],
            [InlineKeyboardButton("Deploy worker", callback_data="deploy_worker")],
            [InlineKeyboardButton("Rollback", callback_data="rollback_menu")],
        ])
        await q.edit_message_text("**DeployBot**\n\nManage your deployments:", reply_markup=kb, parse_mode="Markdown")

    elif data == "status":
        lines = ["**Cluster Status:**\n"]
        for name, svc in SERVICES.items():
            icon = "OK" if svc["status"] == "running" else "ERR"
            lines.append(f"[{icon}] **{name}** {svc['version']} ({svc['replicas']} replicas)")
        await q.edit_message_text("\n".join(lines), reply_markup=kb_back, parse_mode="Markdown")

    elif data.startswith("deploy_"):
        svc_name = data.replace("deploy_", "")
        svc = SERVICES.get(svc_name)
        if not svc:
            await q.edit_message_text("Service not found", reply_markup=kb_back)
            return
        old_ver = svc["version"]
        parts = old_ver.split(".")
        parts[-1] = str(int(parts[-1]) + 1)
        new_ver = ".".join(parts)
        svc["version"] = new_ver
        deploy_time = random.randint(30, 60)
        await q.edit_message_text(
            f"**Deploying** `{svc_name}` {old_ver} -> {new_ver}\n\n"
            f"Cluster: *production*\n"
            f"Replicas: {svc['replicas']}/{svc['replicas']} Ready\n"
            f"Time: {deploy_time} sec\n\n"
            f"Deploy successful!",
            reply_markup=kb_back, parse_mode="Markdown"
        )

    elif data == "rollback_menu":
        kb = InlineKeyboardMarkup([
            [InlineKeyboardButton(f"Rollback {name}", callback_data=f"rollback_{name}")]
            for name in SERVICES
        ] + [[InlineKeyboardButton("Back", callback_data="back")]])
        await q.edit_message_text("**Rollback** — select service:", reply_markup=kb, parse_mode="Markdown")

    elif data.startswith("rollback_"):
        svc_name = data.replace("rollback_", "")
        svc = SERVICES.get(svc_name)
        if not svc:
            await q.edit_message_text("Service not found", reply_markup=kb_back)
            return
        cur_ver = svc["version"]
        parts = cur_ver.split(".")
        parts[-1] = str(max(0, int(parts[-1]) - 1))
        old_ver = ".".join(parts)
        svc["version"] = old_ver
        await q.edit_message_text(
            f"**Rollback** `{svc_name}` {cur_ver} -> {old_ver}\n\n"
            f"Status: rollback complete\n"
            f"Replicas: {svc['replicas']}/{svc['replicas']} Ready",
            reply_markup=kb_back, parse_mode="Markdown"
        )

def main():
    app = Application.builder().token(TOKEN).base_url(PUSK_URL).base_file_url(PUSK_URL).build()
    app.add_handler(CommandHandler("start", start))
    app.add_handler(CallbackQueryHandler(button))
    print("DeployBot polling Pusk...")
    app.run_polling(drop_pending_updates=True)

if __name__ == "__main__":
    main()
