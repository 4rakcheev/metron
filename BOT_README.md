# Metron Telegram Bot

The Metron Telegram Bot provides a convenient interface for parents to manage their children's screen time directly from Telegram.

## Features

- 📊 **View Today's Stats** - See real-time screen time usage for all children
- ➕ **Start Sessions** - Initiate new screen time sessions with multi-step flow
- ⏱ **Extend Sessions** - Add more time to active sessions
- 👶 **Manage Children** - View configured children and their limits
- 📺 **View Devices** - List available devices and their capabilities
- 🔒 **Whitelist Security** - Only authorized users can access the bot

## Quick Start

### 1. Create a Telegram Bot

1. Open Telegram and search for [@BotFather](https://t.me/botfather)
2. Send `/newbot` command
3. Follow the prompts to create your bot
4. Copy the bot token provided by BotFather

### 2. Configure the Bot

Copy the example configuration:

```bash
cp bot-config.example.json bot-config.json
```

Edit `bot-config.json`:

```json
{
  "server": {
    "host": "0.0.0.0",
    "port": 8081
  },
  "telegram": {
    "token": "YOUR_BOT_TOKEN_FROM_BOTFATHER",
    "allowed_users": [
      123456789
    ],
    "webhook_url": "https://your-domain.com/telegram/webhook",
    "webhook_secret": "random-secret-string"
  },
  "metron": {
    "base_url": "http://localhost:8080",
    "api_key": "your-metron-api-key"
  }
}
```

**Getting your Telegram User ID:**

1. Send a message to [@userinfobot](https://t.me/userinfobot)
2. It will reply with your user ID
3. Add this ID to the `allowed_users` array

### 3. Build and Run

```bash
# Build the bot
make build-bot

# Run the bot
./bin/metron-bot -config bot-config.json
```

The bot will:
1. Start the HTTP server for webhooks (default port: 8081)
2. Register the webhook with Telegram
3. Start processing updates

## Commands

| Command | Description |
|---------|-------------|
| `/start` | Show welcome message and quick actions |
| `/today` | View today's screen time summary |
| `/newsession` | Start a new session (multi-step flow) |
| `/extend` | Extend an active session |
| `/children` | List all configured children |
| `/devices` | List available devices |

## Multi-Step Flows

### Starting a New Session

1. Send `/newsession`
2. **Step 1:** Select child (or "Shared" for multiple children)
3. **Step 2:** Select device (TV, PS5, iPad, etc.)
4. **Step 3:** Select duration (5, 15, 30, 60, or 120 minutes)
5. Bot creates the session and confirms with end time

### Extending a Session

1. Send `/extend`
2. **Step 1:** Select the active session to extend
3. **Step 2:** Select additional minutes
4. Bot extends the session and confirms new end time

## Configuration Options

### Server Settings

- **host** (optional): Server bind address (default: `0.0.0.0`)
- **port** (required): HTTP server port (e.g., `8081`)

### Telegram Settings

- **token** (required): Bot token from BotFather
- **allowed_users** (required): Array of Telegram user IDs authorized to use the bot
- **webhook_url** (required): Public HTTPS URL where Telegram will send updates
- **webhook_secret** (optional): Secret token for webhook validation

### Metron API Settings

- **base_url** (required): Metron API base URL (e.g., `http://localhost:8080`)
- **api_key** (required): Metron API key for authentication

## Deployment

### Systemd Service

1. Copy the systemd service file:

```bash
sudo cp deploy/metron-bot.service /etc/systemd/system/
```

2. Create deployment directory:

```bash
sudo mkdir -p /opt/metron-bot
sudo useradd -r -s /bin/false metron
sudo chown metron:metron /opt/metron-bot
```

3. Copy bot binary and config:

```bash
sudo cp bin/metron-bot /opt/metron-bot/
sudo cp bot-config.json /opt/metron-bot/
sudo chown metron:metron /opt/metron-bot/*
```

4. Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable metron-bot
sudo systemctl start metron-bot
```

5. Check status:

```bash
sudo systemctl status metron-bot
sudo journalctl -u metron-bot -f
```

### Webhook Requirements

The bot **requires HTTPS** for webhooks. You need:

1. **Public domain** pointing to your server
2. **Valid SSL certificate** (Let's Encrypt recommended)
3. **Reverse proxy** (nginx/caddy) forwarding to bot port

Example nginx configuration:

```nginx
server {
    listen 443 ssl;
    server_name metron-api.secueval.com;

    ssl_certificate /etc/letsencrypt/live/metron-api.secueval.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/metron-api.secueval.com/privkey.pem;

    location /telegram/webhook {
        proxy_pass http://localhost:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /health {
        proxy_pass http://localhost:8081;
    }
}
```

## Command-Line Flags

```bash
./bin/metron-bot [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `-config` | `bot-config.json` | Path to configuration file |
| `-log-format` | `json` | Log format (json or text) |
| `-log-level` | `info` | Log level (debug, info, warn, error) |

**Note:** Server host and port are configured in `bot-config.json`, not via command-line flags.

## Security

### Whitelist

Only Telegram users listed in `allowed_users` can interact with the bot. Unauthorized users receive:

```
⛔ You are not authorized to use this bot.
```

### Webhook Secret

Configure `webhook_secret` to validate that webhook requests come from Telegram:

```json
{
  "telegram": {
    "webhook_secret": "random-long-secret-string"
  }
}
```

The bot validates the `X-Telegram-Bot-Api-Secret-Token` header on incoming webhooks.

## Troubleshooting

### Bot doesn't respond

1. Check bot is running: `systemctl status metron-bot`
2. Check logs: `journalctl -u metron-bot -n 100`
3. Verify webhook is set: Check logs for "Webhook configured" message
4. Test webhook endpoint: `curl https://your-domain.com/health`

### "Unauthorized" errors

- Verify your Telegram user ID is in `allowed_users`
- Check logs for "Unauthorized access attempt" messages

### API errors

- Verify Metron API is running and accessible
- Check `base_url` points to correct Metron instance
- Verify `api_key` matches Metron configuration
- Test API manually: `curl -H "X-Metron-Key: your-key" http://localhost:8080/v1/children`

### Webhook not receiving updates

1. Verify webhook URL is publicly accessible via HTTPS
2. Check SSL certificate is valid
3. Verify Telegram can reach your server
4. Check nginx/reverse proxy logs
5. Delete webhook and re-register: Restart metron-bot service

## Development

### Running Locally

For local development without HTTPS:

1. Use [ngrok](https://ngrok.com) to create HTTPS tunnel:

```bash
ngrok http 8081
```

2. Update `webhook_url` in config with ngrok URL:

```json
{
  "telegram": {
    "webhook_url": "https://abc123.ngrok.io/telegram/webhook"
  }
}
```

3. Run the bot:

```bash
./bin/metron-bot -config bot-config.json -log-format text -log-level debug
```

### Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage
```

## Architecture

```
┌─────────────┐
│  Telegram   │
│   Server    │
└──────┬──────┘
       │ HTTPS Webhook
       ▼
┌─────────────────┐
│  metron-bot     │
│  (Gin HTTP)     │
├─────────────────┤
│ • Webhook       │
│ • Auth Filter   │
│ • Command Router│
│ • Flow Manager  │
└────────┬────────┘
         │ REST API
         ▼
┌─────────────────┐
│  Metron API     │
│  (Port 8080)    │
└─────────────────┘
```

## License

Same as Metron project.
