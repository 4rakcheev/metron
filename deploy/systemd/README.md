# Systemd Service Files

This directory contains systemd service unit files for running Metron services.

## Files

- **[metron.service](metron.service)** - Main Metron API service
- **[metron-bot.service](metron-bot.service)** - Telegram bot service

## Installation

### 1. Build the Applications

```bash
make build        # Builds all binaries
```

### 2. Install Binaries

```bash
sudo cp bin/metron /usr/local/bin/
sudo cp bin/metron-bot /usr/local/bin/
sudo chmod +x /usr/local/bin/metron
sudo chmod +x /usr/local/bin/metron-bot
```

### 3. Create Configuration Directory

```bash
sudo mkdir -p /etc/metron
sudo cp config.example.json /etc/metron/config.json
sudo cp bot-config.example.json /etc/metron/bot-config.json
```

### 4. Edit Configuration

```bash
sudo nano /etc/metron/config.json
sudo nano /etc/metron/bot-config.json
```

Update with your actual credentials and settings.

### 5. Install Service Files

```bash
sudo cp deploy/systemd/metron.service /etc/systemd/system/
sudo cp deploy/systemd/metron-bot.service /etc/systemd/system/
```

### 6. Reload Systemd and Start Services

```bash
sudo systemctl daemon-reload
sudo systemctl enable metron
sudo systemctl enable metron-bot
sudo systemctl start metron
sudo systemctl start metron-bot
```

## Service Management

### Check Status

```bash
sudo systemctl status metron
sudo systemctl status metron-bot
```

### View Logs

```bash
# Follow logs in real-time
sudo journalctl -u metron -f
sudo journalctl -u metron-bot -f

# View recent logs
sudo journalctl -u metron -n 100
sudo journalctl -u metron-bot -n 100

# View logs since boot
sudo journalctl -u metron -b
sudo journalctl -u metron-bot -b
```

### Restart Services

```bash
sudo systemctl restart metron
sudo systemctl restart metron-bot
```

### Stop Services

```bash
sudo systemctl stop metron
sudo systemctl stop metron-bot
```

### Disable Services

```bash
sudo systemctl disable metron
sudo systemctl disable metron-bot
```

## Service Configuration

### Metron Service

- **User**: `metron` (create this user)
- **Working Directory**: `/opt/metron`
- **Config File**: `/etc/metron/config.json`
- **Database**: `/var/lib/metron/metron.db`
- **Port**: 8080 (default)

### Metron Bot Service

- **User**: `metron` (same user as main service)
- **Working Directory**: `/opt/metron`
- **Config File**: `/etc/metron/bot-config.json`
- **Port**: 8081 (default)
- **Depends On**: `metron.service` (main API must be running)

## Creating the Metron User

```bash
sudo useradd -r -s /bin/false metron
sudo mkdir -p /var/lib/metron
sudo chown metron:metron /var/lib/metron
```

## Security Considerations

1. **File Permissions**: Ensure config files are only readable by the metron user:
   ```bash
   sudo chown metron:metron /etc/metron/*.json
   sudo chmod 600 /etc/metron/*.json
   ```

2. **Database Permissions**: Ensure the database directory is writable:
   ```bash
   sudo chown metron:metron /var/lib/metron
   sudo chmod 755 /var/lib/metron
   ```

3. **API Keys**: Never commit actual API keys or tokens to version control

## Troubleshooting

### Service Won't Start

Check the logs:
```bash
sudo journalctl -u metron -n 50
```

Common issues:
- Config file not found or invalid JSON
- Database directory not writable
- Port already in use
- Missing API keys or credentials

### Service Crashes

Enable core dumps for debugging:
```bash
sudo systemctl edit metron
```

Add:
```ini
[Service]
LimitCORE=infinity
```

### Updating the Service

After code changes:
```bash
make build
sudo cp bin/metron /usr/local/bin/
sudo systemctl restart metron
```

## Production Recommendations

1. **Use a reverse proxy** (nginx/caddy) in front of Metron for HTTPS
2. **Set up log rotation** for journal logs
3. **Monitor service health** with external monitoring
4. **Backup the database** regularly from `/var/lib/metron/`
5. **Use systemd timers** for maintenance tasks

## Example Nginx Configuration

```nginx
server {
    listen 443 ssl http2;
    server_name metron.example.com;

    ssl_certificate /etc/letsencrypt/live/metron.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/metron.example.com/privkey.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```
