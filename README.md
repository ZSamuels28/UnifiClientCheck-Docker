# UniFiClientAlerts

[![Docker Build and Push](https://github.com/ZSamuels28/UnifiClientCheck-Docker/actions/workflows/docker-image.yml/badge.svg)](https://github.com/ZSamuels28/UnifiClientCheck-Docker/actions/workflows/docker-image.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/zsamuels28/unificlientalerts)](https://hub.docker.com/r/zsamuels28/unificlientalerts)

> **🚨 IMPORTANT: v2.9.0+ is now written in Go (completely refactored from PHP)**
>
> **If upgrading from v2.8**: See [Migration Guide](#-migration-from-v28-to-v290) below. One volume path changed: `/usr/src/myapp` → `/data`
>
> All environment variables remain compatible. No breaking changes to configuration.

---

UniFiClientAlerts is a high-performance application written in Go that monitors UniFi networks for new device connections and sends alerts via multiple notification services.

**Docker Hub**: https://hub.docker.com/r/zsamuels28/unificlientalerts

## Quick Start

```bash
docker pull zsamuels28/unificlientalerts:latest
docker-compose up -d
```

## Table of Contents
- [What's New (v2.9.0+)](#-whats-new-v290)
- [Migration Guide (from v2.8)](#-migration-from-v28-to-v290)
- [Features](#features)
- [Setup & Configuration](#setup--configuration)
- [Running](#running-the-application)

---

## ⚡ What's New (v2.9.0+)

**Complete rewrite from PHP to Go:**

| Aspect | Improvement |
|--------|-------------|
| **Performance** | Compiled binary is 5-10x faster, uses 90% less memory |
| **Deployment** | Single executable, zero runtime dependencies |
| **Reliability** | Better error handling, built-in concurrency support |
| **Code Quality** | Standard Go project layout, cleaner architecture |
| **Compatibility** | ✅ All environment variables work the same |

See [CHANGELOG.md](./CHANGELOG.md) for full details.

## ⚠️ Migration from v2.8 to v2.9.0+

### What Changed

Only **one thing** changed in configuration:

| Setting | v2.8 (PHP) | v2.9.0+ (Go) |
|---------|-----------|------------|
| Volume Path | `/usr/src/myapp` | `/data` |
| Environment Variables | All supported | ✅ All supported |
| Features | All supported | ✅ All supported |
| Database | SQLite | ✅ SQLite (compatible) |

### How to Upgrade

#### Option A: Keep Your Database (Recommended)

1. **Stop the container:**
   ```bash
   docker-compose down
   ```

2. **Copy your database to the new location:**
   ```bash
   docker run --rm \
     -v <old-volume-name>:/old_data \
     -v <new-volume-name>:/new_data \
     alpine cp /old_data/knownMacs.db /new_data/knownMacs.db
   ```
   (Replace `<old-volume-name>` and `<new-volume-name>` with your actual volume names)

3. **Update your `docker-compose.yml`:**
   ```yaml
   volumes:
     - data:/data  # Changed from /usr/src/myapp
   ```

4. **Start the new container:**
   ```bash
   docker-compose up -d
   ```

#### Option B: Start Fresh (Simpler)

If you don't need to keep your known device history:

1. Stop the old container: `docker-compose down`
2. Update volume path in `docker-compose.yml` to `/data`
3. Start new container: `docker-compose up -d`
4. App will learn devices on first run

#### Option C: Pre-populate Known Devices

Use the `KNOWN_MACS` environment variable to restore your list:

```yaml
environment:
  KNOWN_MACS: "AA:BB:CC:DD:EE:FF,11:22:33:44:55:66,..."
```

## Features

### Core
- ✅ **Real-time Monitoring** — Scans for new devices on UniFi network
- ✅ **Smart MAC Database** — Remembers known devices (optional SQLite persistence)
- ✅ **IP Wait Support** — Hold notifications until device gets an IP address
- ✅ **Teleport Support** — Monitor Teleport clients (EXPERIMENTAL)

### Notification Services
- 📱 **Telegram** — Direct messaging via bot
- 📲 **Ntfy.sh** — Self-hosted notifications
- 🔔 **Pushover** — Mobile push notifications
- 💬 **Slack** — Team notifications
- 🚀 **Gotify** — Self-hosted push service
- 🎮 **Discord** — Webhook-based notifications
- 🔌 **MQTT** — Publish to broker with JSON payload
- 🌐 **Webhook** — Custom HTTP endpoint with JSON payload

### Deployment
- 🐳 **Docker** — Optimized multi-arch image (amd64, arm, arm64)
- 📦 **From Source** — Build and run locally with Go 1.24+

## Setup & Configuration

### UniFi Controller Setup

To successfully use this application with your UniFi Controller, please follow these guidelines:

- **Local Access Account**: This application requires a UniFi account with local access. UniFi Cloud accounts are not compatible with the UniFi Controller API. Ensure you use an account that can access the UniFi Controller directly.

- **Create a Dedicated User**: For enhanced security and control, it's recommended to create a dedicated local user within your UniFi Controller specifically for API access. This can be done as follows:
  1. Create a new role in UniFi with **View Only** permissions. This restricts the user to only viewing data without making changes to your UniFi setup.
  2. Create a new local user and assign it to the newly created role. Use the credentials of this user for the `UNIFI_CONTROLLER_USER` and `UNIFI_CONTROLLER_PASSWORD` environment variables.

- **Network Connectivity**: Ensure there is direct network connectivity between the server where this application is running and the UniFi Controller. Typically, the UniFi Controller operates on TCP port 8443, or port 443 if you're using UniFi OS. The `UNIFI_CONTROLLER_URL` environment variable should be set to the host and port of your UniFi Controller (e.g., `https://192.168.1.1:8443`).

By following these steps, you can securely and effectively connect this application to your UniFi Controller for monitoring new device connections.

### Notification Service Setup

### Telegram
1. Search for "BotFather" in Telegram.
2. Use the `/newbot` command to create a new bot.
3. Follow the instructions to name your bot and get a token.
4. Save the token and use it in the `TELEGRAM_BOT_TOKEN` variable.
5. Send a message to your bot on Telegram and access `https://api.telegram.org/bot{YOUR_TOKEN}/getUpdates` — this will give you the Chat ID to use in `TELEGRAM_CHAT_ID`.

### Ntfy.sh
See https://github.com/binwiederhier/ntfy/tree/main

### Slack
Follow the guide at https://api.slack.com/messaging/webhooks

### Gotify
1. Set up a Gotify server (self-hosted).
2. Create an application in Gotify and copy the app token.
3. Set `GOTIFY_URL` to your Gotify server URL and `GOTIFY_TOKEN` to the app token.

### Discord
1. In your Discord server, go to **Server Settings > Integrations > Webhooks**.
2. Create a new webhook and copy the URL.
3. Set `DISCORD_WEBHOOK_URL` to the copied URL.

### MQTT
1. Set `MQTT_BROKER` to your broker URL (e.g., `tcp://192.168.1.1:1883`).
2. Set `MQTT_TOPIC` to the topic to publish to (e.g., `unifi/new_device`).
3. Optionally set `MQTT_USER`, `MQTT_PASSWORD`, and `MQTT_CLIENT_ID`.
4. The app publishes a JSON payload and maintains an online/offline status topic at `{MQTT_TOPIC}/status`.

### Webhook
1. Set `WEBHOOK_URL` to your HTTP endpoint.
2. Optionally set `WEBHOOK_SECRET` — if provided, it is sent as a `Bearer` token in the `Authorization` header.
3. The app POSTs a JSON payload with fields: `event`, `name`, `mac`, `ip`, `hostname`, `connection_type`, `network`, `timestamp`.

### Environment Variables

Set these variables for proper configuration:

### UniFi Controller Settings
* `UNIFI_CONTROLLER_USER`: **(Required)** Username for UniFi Controller.
* `UNIFI_CONTROLLER_PASSWORD`: **(Required)** Password for UniFi Controller.
* `UNIFI_CONTROLLER_URL`: **(Required)** URL of UniFi Controller. Use the appropriate port (e.g., `https://192.168.1.1:8443` or `https://192.168.1.1:443` for UniFi OS).
* `UNIFI_SITE_ID`: **(Optional)** Site ID of UniFi Controller (default: `default`).

### General Settings
* `ALWAYS_NOTIFY`: **(Optional)** Set to `true` to send a notification on every check for all devices, not just new ones. Use with caution. (Default: `false`)
* `REMEMBER_NEW_DEVICES`: **(Optional)** Set to `true` to store MAC addresses of newly seen devices so notifications are only sent once. (Default: `true`)
* `KNOWN_MACS`: **(Optional)** Comma-separated list of known MAC addresses to suppress notifications for on first run.
* `CHECK_INTERVAL`: **(Optional)** Interval in seconds between checks. (Default: `60`)
* `REQUIRE_IP`: **(Optional)** Set to `true` to hold notifications for new devices until they have been assigned an IP address. (Default: `false`)
* `DATABASE_PATH`: **(Optional)** Path to the SQLite database file. (Default: `/data/knownMacs.db`)

### Notification Service Selection
* `NOTIFICATION_SERVICE`: **(Optional)** Set to `Telegram`, `Ntfy`, `Pushover`, `Slack`, `Gotify`, `Discord`, `MQTT`, or `Webhook`. (Default: `Telegram`)

### Telegram Settings
* `TELEGRAM_BOT_TOKEN`: **(Required if using Telegram)** Telegram bot token (example: `12345678:ABCDEFGHIJKLMNOPQRSTUVWXYZ`).
* `TELEGRAM_CHAT_ID`: **(Required if using Telegram)** Chat ID for Telegram notifications (example: `234567890`).
* `TELEPORT_NOTIFICATIONS`: **(Optional) EXPERIMENTAL** Set to `true` to include Teleport connected clients. (Default: `false`)

### Ntfy Settings
* `NTFY_URL`: **(Required if using Ntfy)** Ntfy URL (example: `https://ntfy.sh/mytopic` or `http://localhost:8093/mytopic`).
* `NTFY_USER`: **(Optional)** Username for Ntfy authentication.
* `NTFY_PASSWORD`: **(Optional)** Password for Ntfy authentication.
* `NTFY_EMAIL`: **(Optional)** Email address to forward Ntfy notifications to.

### Pushover Settings
* `PUSHOVER_TOKEN`: **(Required if using Pushover)** Pushover app token.
* `PUSHOVER_USER`: **(Required if using Pushover)** Pushover user token.
* `PUSHOVER_TITLE`: **(Optional)** Pushover message title.
* `PUSHOVER_SOUND`: **(Optional)** Pushover notification sound (e.g., `pushover`, `bike`, `magic` — see https://pushover.net/api#sounds).

### Slack Settings
* `SLACK_WEBHOOK_URL`: **(Required if using Slack)** Slack incoming webhook URL.

### Gotify Settings
* `GOTIFY_URL`: **(Required if using Gotify)** Gotify server URL (e.g., `http://gotify.example.com`).
* `GOTIFY_TOKEN`: **(Required if using Gotify)** Gotify application token.

### Discord Settings
* `DISCORD_WEBHOOK_URL`: **(Required if using Discord)** Discord channel webhook URL.

### MQTT Settings
* `MQTT_BROKER`: **(Required if using MQTT)** Broker URL (e.g., `tcp://192.168.1.1:1883`).
* `MQTT_TOPIC`: **(Required if using MQTT)** Topic to publish device alerts to (e.g., `unifi/new_device`).
* `MQTT_USER`: **(Optional)** MQTT broker username.
* `MQTT_PASSWORD`: **(Optional)** MQTT broker password.
* `MQTT_CLIENT_ID`: **(Optional)** MQTT client ID. (Default: `unificlientalerts`)

### Webhook Settings
* `WEBHOOK_URL`: **(Required if using Webhook)** HTTP endpoint to POST JSON payloads to.
* `WEBHOOK_SECRET`: **(Optional)** Bearer token sent as the `Authorization` header.

### Device Removal Settings
* `REMOVE_OLD_DEVICES`: **(Optional)** Set to `true` to remove devices from the known list when they are no longer in the UniFi client list. (Default: `false`)
* `REMOVE_DELAY`: **(Optional)** Seconds after a client disconnects before removing it from known devices. (Default: `0`)

## Running the Application

### Using Docker

- **Pull from Docker Hub**:
  ```bash
  docker pull zsamuels28/unificlientalerts:latest
  ```
- **Run with Docker**:
  ```bash
  docker run -e UNIFI_CONTROLLER_USER=... (other environment variables) zsamuels28/unificlientalerts:latest
  ```

### Using Docker Compose
- Create a `.env` file with the necessary environment variables.
- Run:
  ```bash
  docker-compose up
  ```

### Manual Docker Build
- Clone the repository.
- Build the Docker image:
  ```bash
  docker build -t unificlientalerts .
  ```
- Run the container:
  ```bash
  docker run -e UNIFI_CONTROLLER_USER=... (other environment variables) unificlientalerts
  ```

### Running Outside Docker (Go)
- Ensure Go 1.24+ is installed.
- Clone the repository and navigate to the project directory.
- Download dependencies:
  ```bash
  go mod download
  ```
- Build and run:
  ```bash
  go build -o unificlientalerts ./cmd/unificlientalerts
  ./unificlientalerts
  ```

## Contributions

Contributions are welcome. Please adhere to the project's standards and submit a pull request for review.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](https://github.com/ZSamuels28/UnifiClientCheck-Docker/blob/main/LICENSE) file for details.