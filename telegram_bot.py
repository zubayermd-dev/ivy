#!/usr/bin/env python3
"""
Telegram bot that bridges Telegram -> smsie -> SMS modem.
Listens for /send commands and forwards them as SMS via smsie API.
"""

import json
import time
import sys
import os
import httpx

# ── Config (load from environment variables) ──────────────────────────────
TG_BOT_TOKEN = os.environ.get("TG_BOT_TOKEN", "")
ALLOWED_CHAT_ID = int(os.environ.get("TG_CHAT_ID", "0"))
SMSIE_BASE = os.environ.get("SMSIE_BASE", "http://localhost:8080")
SMSIE_USER = os.environ.get("SMSIE_USER", "admin")
SMSIE_PASS = os.environ.get("SMSIE_PASS", "")
ICCID = os.environ.get("SMSIE_ICCID", "")
# ────────────────────────────────────────────────────────────────────────

if not TG_BOT_TOKEN or not SMSIE_PASS or not ICCID:
    print("[bot] ERROR: Missing required environment variables")
    print("[bot] Required: TG_BOT_TOKEN, SMSIE_PASS, SMSIE_ICCID")
    sys.exit(1)

TG_API = f"https://api.telegram.org/bot{TG_BOT_TOKEN}"
POLL_TIMEOUT = 30
OFFSET = 0
TOKEN = None


def tg_send(chat_id, text):
    """Send a message via Telegram Bot API."""
    httpx.post(f"{TG_API}/sendMessage", json={
        "chat_id": chat_id,
        "text": text,
        "parse_mode": "HTML",
    }, timeout=10)


def get_smsie_token():
    """Login to smsie and get JWT token."""
    r = httpx.post(f"{SMSIE_BASE}/api/v1/login", json={
        "username": SMSIE_USER,
        "password": SMSIE_PASS,
    }, timeout=10)
    r.raise_for_status()
    return r.json()["token"]


def send_sms(token, phone, message):
    """Send SMS via smsie API."""
    r = httpx.post(
        f"{SMSIE_BASE}/api/v1/modems/{ICCID}/send",
        headers={"Authorization": f"Bearer {token}"},
        json={"phone": phone, "message": message},
        timeout=30,
    )
    r.raise_for_status()
    return r.json()


def handle_command(text, chat_id):
    """Parse and handle bot commands."""
    global TOKEN

    text = text.strip()

    # /send +88017... Hello there
    if text.startswith("/send"):
        parts = text.split(None, 2)  # /send <phone> <message>
        if len(parts) < 3:
            tg_send(chat_id, "Usage: /send <phone> <message>\nExample: /send +8801711111111 Hello there")
            return

        phone = parts[1]
        message = parts[2]

        # Basic phone validation
        if not phone.startswith("+") and not phone.startswith("0"):
            tg_send(chat_id, "Phone number must start with + or 0")
            return

        try:
            if not TOKEN:
                TOKEN = get_smsie_token()
            result = send_sms(TOKEN, phone, message)
            tg_send(chat_id, f"SMS sent to {phone}")
        except httpx.HTTPStatusError as e:
            if e.response.status_code == 401:
                # Token expired, retry once
                try:
                    TOKEN = get_smsie_token()
                    result = send_sms(TOKEN, phone, message)
                    tg_send(chat_id, f"SMS sent to {phone}")
                except Exception as e2:
                    tg_send(chat_id, f"Auth failed: {e2}")
            else:
                tg_send(chat_id, f"Send failed: {e.response.text}")
            TOKEN = None
        except Exception as e:
            tg_send(chat_id, f"Error: {e}")

    elif text.startswith("/status"):
        try:
            if not TOKEN:
                TOKEN = get_smsie_token()
            r = httpx.get(
                f"{SMSIE_BASE}/api/v1/modems/{ICCID}",
                headers={"Authorization": f"Bearer {TOKEN}"},
                timeout=10,
            )
            r.raise_for_status()
            m = r.json()
            tg_send(chat_id,
                f"Modem: {m.get('status', '?')}\n"
                f"Operator: {m.get('operator', '?')}\n"
                f"Signal: {m.get('signal_strength', '?')}\n"
                f"Port: {m.get('port_name', '?')}"
            )
        except Exception as e:
            tg_send(chat_id, f"Status error: {e}")
            TOKEN = None

    elif text.startswith("/help"):
        tg_send(chat_id,
            "Commands:\n"
            "/send +phone message — Send SMS\n"
            "/status — Modem status\n"
            "/help — This message"
        )
    else:
        tg_send(chat_id, "Unknown command. Send /help for usage.")


def poll():
    """Long-poll for Telegram updates."""
    global OFFSET, TOKEN

    # Get initial token
    try:
        TOKEN = get_smsie_token()
        print("[bot] smsie token acquired")
    except Exception as e:
        print(f"[bot] Warning: initial smsie login failed: {e}")

    print("[bot] Starting polling...")

    while True:
        try:
            r = httpx.get(f"{TG_API}/getUpdates", params={
                "offset": OFFSET,
                "timeout": POLL_TIMEOUT,
            }, timeout=POLL_TIMEOUT + 10)

            data = r.json()
            if not data.get("ok"):
                print(f"[bot] getUpdates error: {data}")
                time.sleep(5)
                continue

            for update in data.get("result", []):
                OFFSET = update["update_id"] + 1
                msg = update.get("message")
                if not msg:
                    continue

                chat_id = msg["chat"]["id"]
                if chat_id != ALLOWED_CHAT_ID:
                    tg_send(chat_id, "Unauthorized")
                    continue

                text = msg.get("text", "")
                if text.startswith("/"):
                    print(f"[bot] Command from {chat_id}: {text}")
                    handle_command(text, chat_id)

        except KeyboardInterrupt:
            print("\n[bot] Shutting down")
            sys.exit(0)
        except Exception as e:
            print(f"[bot] Poll error: {e}")
            time.sleep(5)


if __name__ == "__main__":
    poll()
