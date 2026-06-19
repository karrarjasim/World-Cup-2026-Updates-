# 🏆 World Cup 2026 Updates Bot

A fully automated Telegram bot that tracks the FIFA World Cup 2026 and posts
everything to a Telegram **channel**: live results with AI analysis the moment a
match ends, a daily morning fixtures digest, pre-match reminders, and breaking
Arabic sports news. Users can also chat with the bot directly to ask about
fixtures, standings, and more.

Written in **Go** with the standard library only (no external dependencies), so
it builds and runs in seconds.

---

## ✨ How it works (fixture-driven)

Instead of an endless polling loop that burns through API quota, the bot knows
the match calendar in advance and only touches the football API inside a
match's finish window:

```
[ Daily sync: pull the full calendar + standings ]   → ~1 request/day
                    │
                    ▼
[ Outside any match: idle, zero requests ]
                    │
            (inside a match finish window?)
                    │  yes
                    ▼
[ Poll only in-progress matches by id ]  → one request covers all concurrent games
                    │
            (turned FT and not sent before?)
                    ▼
[ Refresh standings → send result + group table to the LLM ]
                    ▼
[ Generate analysis → post the result to the Telegram channel ]
```

Sent events are persisted to disk (JSON), so results never repeat — even after
a restart.

---

## 📦 Packages

| Package | Role |
|---------|------|
| `internal/config` | Load configuration from environment variables |
| `internal/store` | Durable JSON storage (atomic writes + mutex locks) |
| `internal/football` | ESPN public API client + Arabic team-name mapping |
| `internal/llm` | OpenAI-compatible chat client for analysis and answers |
| `internal/telegram` | Telegram Bot API client (channel posting + safe broadcast) |
| `internal/assistant` | Local intent routing + LLM answers grounded in live context |
| `internal/news` | RSS/Atom fetching, optional World Cup filtering, dedup |
| `internal/scheduler` | The engine: sync, poll, broadcast, reminders, digest, news |
| `internal/bot` | Telegram update loop + commands |

---

## ⚙️ Setup

1. Create a bot with [@BotFather](https://t.me/BotFather) to get `TELEGRAM_TOKEN`.
2. Create a channel, add your bot as an **admin** with the "Post Messages"
   permission, and put its identifier in `CHANNEL_ID`
   (public channel: `@yourchannel`; private channel: numeric `-100...` id).
3. (Optional) Get a free LLM key at [Groq](https://console.groq.com/keys) for AI
   analysis and chat answers. Leaving it empty disables AI only — scores,
   standings, and reminders still work.
4. Copy and fill the environment file:

```bash
cp .env.example .env
# fill in at least TELEGRAM_TOKEN and CHANNEL_ID
```

### Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `TELEGRAM_TOKEN` | Bot token from @BotFather (required) | — |
| `CHANNEL_ID` | Target channel for all automatic updates | — |
| `LLM_API_KEY` | OpenAI-compatible key (Groq, OpenRouter, OpenAI, Gemini) | empty (AI off) |
| `LLM_BASE_URL` | Chat endpoint base URL | Groq |
| `LLM_MODEL` | Chat model name | `llama-3.3-70b-versatile` |
| `FOOTBALL_BASE_URL` | ESPN API host | `https://site.api.espn.com` |
| `SEASON` | Tournament year | `2026` |
| `TZ` | Display timezone | `Asia/Baghdad` |
| `DIGEST_HOUR` | Local hour (0–23) for the daily digest | `9` |
| `LIVE_POLL_SECONDS` | Poll interval inside match finish windows | `240` |
| `ADMIN_CHAT_ID` | Chat id for startup/error pings (optional) | — |
| `NEWS_FEEDS` | Comma-separated RSS/Atom feed URLs | empty (news off) |
| `NEWS_WORLDCUP_ONLY` | Only forward World-Cup-related headlines | `false` |
| `NEWS_SUMMARIZE` | Rewrite each headline into a short blurb via the LLM | `false` |

---

## ▶️ Run locally

```bash
go run ./cmd/bot
# or
go build -o mundialbot ./cmd/bot && ./mundialbot
```

## 🐳 Run with Docker

```bash
docker build -t mundialbot .
docker run -d --name mundialbot --restart unless-stopped \
  --env-file .env -v mundial_data:/app/data mundialbot
```

## 🖥️ Persistent deployment (systemd)

```bash
sudo useradd -r -m -d /opt/mundialbot mundial
sudo cp mundialbot /opt/mundialbot/ && sudo cp .env /opt/mundialbot/
sudo mkdir -p /opt/mundialbot/data && sudo chown -R mundial:mundial /opt/mundialbot
sudo cp deploy/mundialbot.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now mundialbot
sudo journalctl -u mundialbot -f   # follow logs
```

`Restart=always` in the systemd unit brings the bot back automatically after any
crash or server reboot.

---

## 💬 Commands

| Command | Description |
|---------|-------------|
| `/start` | Start a conversation with the bot |
| `/today` | Today's matches |
| `/standings` | Group standings |
| `/next <team>` | Next match for a team |
| `/stop` | Stop the conversation |

> 📢 All automatic updates (results, daily digest, news, pre-match reminders)
> are posted to the channel set by `CHANNEL_ID` — not to private chats. The bot
> itself remains available to answer questions on demand.

You can also ask the bot directly in Arabic, e.g. "who leads Group A?" or
"when does Iraq play next?".

---

## 📊 API budget (safe estimate)

- Daily sync (calendar + standings): ~2 requests/day.
- Polling only inside finish windows, every few minutes, one request covering
  all concurrent matches.
- Standings refreshed once per finished result.
- Even on the busiest group-stage days the total stays well under free-tier
  limits.

---

## 🔧 Notes and future ideas

- Current storage is JSON files (fine for hundreds of users). For larger scale
  or complex queries, the `store` layer can be swapped for SQLite without
  touching the rest of the code.
- LLM function-calling could be added so the assistant queries live data itself.
- Live goal notifications are possible by increasing poll frequency inside the
  window (watch the API budget).
