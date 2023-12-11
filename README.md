# UniFiClientAlerts

UniFiClientAlerts is a Dockerized application that monitors UniFi networks for new device connections and sends alerts via Telegram. It leverages PHP code from the [Art-of-WiFi/UniFi-API-client](https://github.com/Art-of-WiFi/UniFi-API-client) for interfacing with UniFi Controllers.

## Features

- **Real-time Monitoring**: Scans for new devices on the UniFi network.
- **Telegram Notifications**: Sends alerts through Telegram.
- **Flexible Deployment**: Can be run in Docker, manually, or as a standalone PHP script.

## UniFi Configuration

To successfully use this application with your UniFi Controller, please follow these guidelines:

- **Local Access Account**: This application requires a UniFi account with local access. UniFi Cloud accounts are not compatible with the UniFi Controller API used in this class. Ensure you use an account that can access the UniFi Controller directly.

- **Create a Dedicated User**: For enhanced security and control, it's recommended to create a dedicated local user within your UniFi Controller specifically for API access. This can be done as follows:
  1. Create a new role in UniFi with **View Only** permissions. This restricts the user to only viewing data without making changes to your UniFi setup.
  2. Create a new local user and assign it to the newly created role. Use the credentials of this user for the `UNIFI_CONTROLLER_USER` and `UNIFI_CONTROLLER_PASSWORD` environment variables.

- **Network Connectivity**: Ensure there is direct network connectivity between the server where this application is running and the UniFi Controller. Typically, the UniFi Controller operates on TCP port 8443, or port 443 if you're using UniFi OS. The `UNIFI_CONTROLLER_URL` environment variable should be set to the host and port of your UniFi Controller (e.g., `https://192.168.1.1:8443`).

By following these steps, you can securely and effectively connect this application to your UniFi Controller for monitoring new device connections.

## Telegram Configuration and Setting Up a Telegram Bot
1. Search for "BotFather" in Telegram.
2. Use the /newbot command to create a new bot.
3. Follow the instructions to name your bot and get a token.
4. Save the token and use it in the `TELEGRAM_BOT_TOKEN` variable.
5. Send a message to your bot on Telegram and access `https://api.telegram.org/bot{YOUR_TOKEN}/getUpdates` this will give you the Chat ID to use in `TELEGRAM_CHAT_ID`

## Environment Variables

Set these variables for proper configuration:

- `UNIFI_CONTROLLER_USER`: Username for UniFi Controller.
- `UNIFI_CONTROLLER_PASSWORD`: Password for UniFi Controller.
- `UNIFI_CONTROLLER_URL`: URL of UniFi Controller (e.g., `https://192.168.1.1:8443 or https://192.168.1.1:443 for UniFi OS`).
- `UNIFI_SITE_ID`: Site ID of UniFi Controller (default: `default`).
- `KNOWN_MACS`: **(Optional)** Comma-separated list of known MAC addresses.
- `CHECK_INTERVAL`: Interval in seconds between checks (e.g., `60`).
- `TELEGRAM_BOT_TOKEN`: Telegram bot token.
- `TELEGRAM_CHAT_ID`: Chat ID for Telegram notifications.
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

This project utilizes code from the Art-of-WiFi/UniFi-API-client, a PHP-based client for UniFi Controller APIs.