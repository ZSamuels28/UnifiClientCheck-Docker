# UniFiClientAlerts

![Version](https://img.shields.io/badge/version-PLACEHOLDER-blue)
[![Docker Build and Push](https://github.com/ZSamuels28/UnifiClientCheck-Docker/actions/workflows/docker-image.yml/badge.svg)](https://github.com/ZSamuels28/UnifiClientCheck-Docker/actions/workflows/docker-image.yml)
[![Docker Pulls](https://img.shields.io/docker/pulls/zsamuels28/unificlientalerts)](https://hub.docker.com/r/zsamuels28/unificlientalerts)

UniFiClientAlerts is a Dockerized application that monitors UniFi networks for new device connections and sends alerts via Telegram or [Ntfy.sh](https://github.com/binwiederhier/ntfy/tree/main). It leverages PHP code from the [Art-of-WiFi/UniFi-API-client](https://github.com/Art-of-WiFi/UniFi-API-client) for interfacing with UniFi Controllers.

This has been tested on a number of devices, and I personally have this running on Portainer on a Raspberry Pi 5.

Docker Hub Image: https://hub.docker.com/r/zsamuels28/unificlientalerts

## Features

- **Real-time Monitoring**: Scans for new devices on the UniFi network.
- **Telegram Notifications**: Sends alerts through Telegram.
- **Ntfy Notifications**: Sends alerts through Ntfy.
- **Flexible Deployment**: Can be run in Docker, manually, or as a standalone PHP script.
- **Known MAC Addresses Database (Optional)**: Creates a database of known MAC addresses to prevent repeated notifications for familiar devices, allowing users to customize their notification preferences.

## UniFi Configuration

To successfully use this application with your UniFi Controller, please follow these guidelines:

- **Local Access Account**: This application requires a UniFi account with local access. UniFi Cloud accounts are not compatible with the UniFi Controller API used in this class. Ensure you use an account that can access the UniFi Controller directly.

- **Create a Dedicated User**: For enhanced security and control, it's recommended to create a dedicated local user within your UniFi Controller specifically for API access. This can be done as follows:
  1. Create a new role in UniFi with **View Only** permissions. This restricts the user to only viewing data without making changes to your UniFi setup.
  2. Create a new local user and assign it to the newly created role. Use the credentials of this user for the `UNIFI_CONTROLLER_USER` and `UNIFI_CONTROLLER_PASSWORD` environment variables.

- **Network Connectivity**: Ensure there is direct network connectivity between the server where this application is running and the UniFi Controller. Typically, the UniFi Controller operates on TCP port 8443, or port 443 if you're using UniFi OS. The `UNIFI_CONTROLLER_URL` environment variable should be set to the host and port of your UniFi Controller (e.g., `https://192.168.1.1:8443`).

By following these steps, you can securely and effectively connect this application to your UniFi Controller for monitoring new device connections.

## Setting up an Ntfy.sh Topic
1. See https://github.com/binwiederhier/ntfy/tree/main

## Telegram Configuration and Setting Up a Telegram Bot
1. Search for "BotFather" in Telegram.
2. Use the /newbot command to create a new bot.
3. Follow the instructions to name your bot and get a token.
4. Save the token and use it in the `TELEGRAM_BOT_TOKEN` variable.
5. Send a message to your bot on Telegram and access `https://api.telegram.org/bot{YOUR_TOKEN}/getUpdates` this will give you the Chat ID to use in `TELEGRAM_CHAT_ID`

## Environment Variables

Set these variables for proper configuration:

- `UNIFI_CONTROLLER_USER`: **(Required)** Username for UniFi Controller.
- `UNIFI_CONTROLLER_PASSWORD`: **(Required)** Password for UniFi Controller.
- `UNIFI_CONTROLLER_URL`: **(Required)** URL of UniFi Controller. Use the appropriate port (e.g., `https://192.168.1.1:8443` or `https://192.168.1.1:443` for UniFi OS).
- `ALWAYS_NOTIFY`: **(Optional)** Set to true to enable constant notifications for devices not in the KNOWN_MACS list or in the REMEMBER_NEW_DEVICES list if REMEMBER_NEW_DEVICES is also set to true. Use with caution as it may result in frequent notifications. (Default: `false`)
- `REMEMBER_NEW_DEVICES`: **(Optional)** Set to true to store MAC addresses of devices seen on the network (excluding those in KNOWN_MACS). This ensures notifications are sent only once for new device connections and allows for persistent storage of the database across app or container resets. (Default: `true`)
- `UNIFI_SITE_ID`: **(Optional)** Site ID of UniFi Controller (default: `default`).
- `KNOWN_MACS`: **(Optional)** Comma-separated list of known MAC addresses. Or you can let the app run once and send you a one-time notification for everything on your network.
- `CHECK_INTERVAL`: **(Optional)** Interval in seconds between checks (default: `60`).
- `TELEGRAM_BOT_TOKEN`: **(Required if using Telegram)** Telegram bot token if using Telegram (example: `12345678:ABCDEFGHIJKLMNOPQRSTUVWXYZ`).
- `TELEGRAM_CHAT_ID`: **(Required if using Telegram)** Chat ID for Telegram notifications if using Telegram (example: `234567890`).
- `NTFY_URL`: **(Required if using Ntfy.sh)** Ntfy.sh URL if using Ntfy (example: `ntfy.sh/topic123`)
- `NOTIFICATION_SERVICE`: **(Optional)** Set to `Telegram` or `Ntfy`. (default: `Telegram`)
- `CONTROLLER_VERSION`: **(Optional)** Version of UniFi Controller software.

## Running the Application

### Using Docker

- **Pull from Docker Hub**:
  ```bash
  docker pull zsamuels28/unificlientalerts:latest
- **Run with Docker**:
  ```bash
  docker run -e UNIFI_CONTROLLER_USER=... (other environment variables) zsamuels28/unificlientalerts:latest

### Using Docker Compose
- Create a .env file with the necessary environment variables.
- Run:
  ```bash
  docker-compose up

### Manual Docker Build
- Clone the repository.
- Build the Docker image:
  ```bash
  docker build -t unificlientalerts .
- Run the container:
  ```bash
  docker run -e UNIFI_CONTROLLER_USER=... (other environment variables) unificlientalerts

### Running Outside Docker
- Ensure PHP and required extensions are installed.
- Clone the repository and navigate to the project directory.
- Install dependencies:
  ```bash
  composer install
- Set the necessary environment variables in your shell or use a .env file.
- Run the PHP script located in /src:
  ```bash
  UnifiClientAlerts.php

## Contributions

Contributions are welcome. Please adhere to the project's standards and submit a pull request for review.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](https://github.com/ZSamuels28/UnifiClientCheck-Docker/blob/main/LICENSE) file for details.

## Acknowledgements

This project utilizes code from the [Art-of-WiFi/UniFi-API-client](https://github.com/Art-of-WiFi/UniFi-API-client), a PHP-based client class to interact with the UniFi Controller API.
