# UniFiClientAlerts

UniFiClientAlerts is a Dockerized application that monitors UniFi networks for new device connections and sends alerts via Telegram. It leverages PHP code from the [Art-of-WiFi/UniFi-API-client](https://github.com/Art-of-WiFi/UniFi-API-client) for interfacing with UniFi Controllers.

## Features

- **Real-time Monitoring**: Scans for new devices on the UniFi network.
- **Telegram Notifications**: Sends alerts through Telegram.
- **Flexible Deployment**: Can be run in Docker, manually, or as a standalone PHP script.

## Environment Variables

Set these variables for proper configuration:

- `UNIFI_CONTROLLER_USER`: Username for UniFi Controller.
- `UNIFI_CONTROLLER_PASSWORD`: Password for UniFi Controller.
- `UNIFI_CONTROLLER_URL`: URL of UniFi Controller (e.g., `https://192.168.1.1:8443`).
- `UNIFI_SITE_ID`: Site ID of UniFi Controller (default: `default`).
- `KNOWN_MACS`: Comma-separated list of known MAC addresses.
- `CHECK_INTERVAL`: Interval in seconds between checks (e.g., `60`).
- `TELEGRAM_BOT_TOKEN`: Telegram bot token.
- `TELEGRAM_CHAT_ID`: Chat ID for Telegram notifications.
- `CONTROLLER_VERSION`: Version of UniFi Controller software.

## Running the Application

### Using Docker

- **Pull from Docker Hub**:
  ```bash
  docker pull zsamuels28/unificlientalerts:latest
- **Run with Docker**:
  ```bash docker run -e UNIFI_CONTROLLER_USER=... (other environment variables) zsamuels28/unificlientalerts:latest

### Using Docker Compose
- Create a .env file with the necessary environment variables.
- Run: ```bash docker-compose up

### Manual Docker Build
- Clone the repository.
- Build the Docker image: ```bash docker build -t unificlientalerts .
- Run the container: ```bash docker run -e UNIFI_CONTROLLER_USER=... (other environment variables) unificlientalerts

### Running Outside Docker
- Ensure PHP and required extensions are installed.
- Clone the repository and navigate to the project directory.
- Install dependencies: ```bash composer install
- Set the necessary environment variables in your shell or use a .env file.
- Run the PHP script: ```bash UnifiClientAlerts.php

## Contributions

Contributions are welcome. Please adhere to the project's standards and submit a pull request for review.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](https://github.com/ZSamuels28/UnifiClientCheck-Docker/blob/main/LICENSE) file for details.

## Acknowledgements

This project utilizes code from the [Art-of-WiFi/UniFi-API-client](https://github.com/Art-of-WiFi/UniFi-API-client), a PHP-based client for UniFi Controller APIs.