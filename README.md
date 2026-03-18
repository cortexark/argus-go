# Argus — AI Privacy Monitor

**Single binary. Zero dependencies. Watch what your AI tools really do.**

Argus monitors file system access, network connections, and activity of AI applications (Claude, Cursor, Copilot, ChatGPT, Ollama, etc.) on your Mac or Linux machine — and stores everything in a local SQLite database.

## Install

Download the binary for your platform from [Releases](https://github.com/cortexark/argus-go/releases):

```bash
# macOS Apple Silicon
curl -L https://github.com/cortexark/argus-go/releases/latest/download/argus-darwin-arm64 -o argus
chmod +x argus && sudo mv argus /usr/local/bin/

# macOS Intel
curl -L https://github.com/cortexark/argus-go/releases/latest/download/argus-darwin-amd64 -o argus
chmod +x argus && sudo mv argus /usr/local/bin/

# Linux x86_64
curl -L https://github.com/cortexark/argus-go/releases/latest/download/argus-linux-amd64 -o argus
chmod +x argus && sudo mv argus /usr/local/bin/
```

No Node.js, no Python, no runtime required.

## Usage

```bash
argus start        # install & start background daemon (launchd on macOS, systemd on Linux)
argus status       # check daemon status
argus scan         # one-shot scan without daemon
argus report       # daily summary report
argus report -d 2025-01-15   # report for a specific date
argus logs         # recent file access alerts
argus logs -n 100  # last 100 events
argus stop         # stop the daemon
```

Web dashboard available at http://127.0.0.1:3131 while the daemon is running.

## What Argus Monitors

| Category | Details |
|----------|---------|
| **AI Processes** | Detects Claude, Cursor, Copilot, ChatGPT, Ollama, LM Studio, Windsurf, Aider, and 15+ more |
| **Sensitive Files** | SSH keys, credentials, browser cookies/history, Documents, Downloads |
| **Network** | Connections to api.anthropic.com, api.openai.com, and other AI endpoints |
| **Injection Attacks** | Clipboard and file scanning for prompt injection patterns |

## How it Works

1. **Daemon** runs in the background via launchd (macOS) or systemd (Linux)
2. **Process scanner** samples running processes every 30s using `ps`
3. **File monitor** uses `lsof` (macOS) or `/proc/PID/fd` (Linux) to find open sensitive files
4. **Network monitor** uses `netstat -anv` (macOS) or `ss -tunap` (Linux)
5. **Injection detector** scans clipboard for prompt injection patterns
6. Everything is stored locally in `~/.argus/argus.db` (SQLite)

## AI App Detection — 6-Signal Classifier

Argus classifies AI apps using 6 independent signals:

| Signal | Points | Description |
|--------|--------|-------------|
| Known registry | +60 | In the list of 21+ known AI apps |
| Keyword match | +20 | Name/cmdline contains "claude", "gpt", "copilot", etc. |
| AI port | +15 | Listening on port 11434 (Ollama), 1234 (LM Studio), etc. |
| AI parent | +20 | Spawned by a known AI process |
| AI env var | +15 | Has ANTHROPIC_*, OPENAI_*, CLAUDE_* env vars |
| LLM cmdline | +10 | Command line mentions "llm", "langchain", etc. |

Score ≥ 50 → CONFIRMED_AI · Score 30-49 → LIKELY_AI · Score < 30 → ignored

## Data Storage

All data is stored locally at `~/.argus/`:
- `argus.db` — SQLite database (WAL mode, 0600 permissions)
- `logs/` — Daemon logs

Nothing is ever sent to any server.

## Build from Source

```bash
git clone https://github.com/cortexark/argus-go
cd argus-go
go build -o argus ./cmd/argus/
./argus --help
```

Requires Go 1.21+.

## License

MIT
