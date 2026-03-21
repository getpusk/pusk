#!/usr/bin/env python3
"""AlertBot — AI alert analysis via Groq, connected to Pusk via Bot API getUpdates."""

import asyncio
import json
import os
import aiohttp

PUSK_URL = os.getenv("PUSK_URL", "https://getpusk.ru")
BOT_TOKEN = os.getenv("BOT_TOKEN", "alert-ai-token")
GROQ_KEY = os.getenv("GROQ_API_KEY", "")
GROQ_MODEL = os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile")
GROQ_URL = "https://api.groq.com/openai/v1/chat/completions"
PROXY = os.getenv("HTTPS_PROXY", "")

SYSTEM_PROMPT = """You are AlertBot, an AI ops assistant. You analyze monitoring alerts and provide brief, actionable recommendations.

Format:
**Summary:** one line
**Cause:** 1-2 sentences
**Action:** specific steps or commands

Be concise. Use markdown. Respond in the language of the alert."""


async def ask_groq(session, text):
    if not GROQ_KEY:
        return "AlertBot: GROQ_API_KEY not configured. Set it to enable AI analysis."
    try:
        async with session.post(GROQ_URL, proxy=PROXY, headers={
            "Authorization": f"Bearer {GROQ_KEY}",
            "Content-Type": "application/json"
        }, json={
            "model": GROQ_MODEL,
            "messages": [
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": f"Analyze this alert:\n\n{text}"}
            ],
            "max_tokens": 200,
            "temperature": 0.3
        }, timeout=aiohttp.ClientTimeout(total=15)) as resp:
            data = await resp.json()
            return data["choices"][0]["message"]["content"]
    except Exception as e:
        print(f"[groq] error: {e}")
        return f"AlertBot: AI unavailable ({e})"


async def send_message(session, chat_id, text):
    async with session.post(f"{PUSK_URL}/bot/{BOT_TOKEN}/sendMessage", json={
        "chat_id": chat_id, "text": text
    }) as resp:
        print(f"[bot] reply sent: {resp.status}")


async def main():
    offset = 0
    print(f"AlertBot (Groq) started, polling {PUSK_URL}...")
    print(f"  Proxy: {PROXY}")
    print(f"  Model: {GROQ_MODEL}")
    print(f"  API key: {'set' if GROQ_KEY else 'MISSING'}")

    async with aiohttp.ClientSession() as session:
        async with session.get(f"{PUSK_URL}/bot/{BOT_TOKEN}/getMe") as resp:
            me = await resp.json()
            print(f"[bot] Connected as {me['result']['first_name']}")

        while True:
            try:
                async with session.post(
                    f"{PUSK_URL}/bot/{BOT_TOKEN}/getUpdates",
                    json={"offset": offset, "timeout": 30},
                    timeout=aiohttp.ClientTimeout(total=35)
                ) as resp:
                    data = await resp.json()

                for update in data.get("result", []):
                    offset = update["update_id"] + 1
                    msg = update.get("message", {})
                    text = msg.get("text", "")
                    chat_id = msg.get("chat", {}).get("id", 0)

                    if not text or msg.get("from", {}).get("is_bot"):
                        continue

                    is_alert = any(kw in text.lower() for kw in [
                        "alert", "firing", "critical", "warning", "error",
                        "highcpu", "highmemory", "diskspace", "down",
                        "alertmanager", "zabbix", "grafana", "uptime"
                    ])

                    if is_alert:
                        print(f"[alert] Analyzing: {text[:60]}...")
                        analysis = await ask_groq(session, text)
                        if analysis:
                            await send_message(session, chat_id, f"**AI Analysis:**\n\n{analysis}")
                    else:
                        print(f"[skip] Not an alert: {text[:40]}")

            except asyncio.CancelledError:
                break
            except Exception as e:
                print(f"[error] {e}")
                await asyncio.sleep(5)


if __name__ == "__main__":
    asyncio.run(main())
