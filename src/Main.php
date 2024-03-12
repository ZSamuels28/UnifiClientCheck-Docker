<?php
require_once(__DIR__ . '/Database.php');
require_once(__DIR__ . '/Notifier.php');
require_once(__DIR__ . '/Unifi-API-client/Client.php');
require_once(__DIR__ . '/Unifi-API-client/config.php');
require_once(__DIR__ . '/../vendor/autoload.php');

// Environment configuration
$envKnownMacs = array_map('trim', explode(',', getenv('KNOWN_MACS') ?: ''));
$checkInterval = getenv('CHECK_INTERVAL') ?: 60;
$notificationService = getenv('NOTIFICATION_SERVICE') ?: 'Telegram';
$alwaysNotify = filter_var(getenv('ALWAYS_NOTIFY') ?: False, FILTER_VALIDATE_BOOLEAN);
$rememberNewDevices = filter_var(getenv('REMEMBER_NEW_DEVICES') ?: True, FILTER_VALIDATE_BOOLEAN);

// Validate critical environment configurations
if (!in_array($notificationService, ['Telegram', 'Ntfy'])) {
    echo "Error: Invalid notification service specified. Please set NOTIFICATION_SERVICE to either 'Telegram' or 'Ntfy'.\n";
    exit(1);
}

// Initialize Database, Notifier, and UniFiClient
$database = new Database(__DIR__ . '/knownMacs.db');
$knownMacs = $database->loadKnownMacs($envKnownMacs);
$notifier = new Notifier(getenv('TELEGRAM_BOT_TOKEN'), getenv('TELEGRAM_CHAT_ID'), getenv('NTFY_URL'));

function createUnifiClient() {
    global $controlleruser, $controllerpassword, $controllerurl, $site_id, $controllerversion;
    $unifiClient = new UniFi_API\Client($controlleruser, $controllerpassword, $controllerurl, $site_id, $controllerversion);
    $unifiClient->login();
    return $unifiClient;
}

$unifiClient = createUnifiClient();

// Main loop
while (true) {
    try {
        $clients = $unifiClient->list_clients();
        $newDeviceFound = false; // Initialize flag to track new device detection

        if ($clients === false) {
            echo "Error: Failed to retrieve clients from the UniFi Controller. Retrying in 60 seconds...\n";
            sleep(60);
            $unifiClient->logout();
            $unifiClient = createUnifiClient();
            continue;
        }

        if (empty($clients)) {
            echo "No devices currently connected to the network.\n";
            continue;
        }

        foreach ($clients as $client) {
            $isNewDevice = !in_array($client->mac, $knownMacs);
            if ($isNewDevice) {
                echo "New device found. Sending a notification.\n";
                $newDeviceFound = true;
            }

            if ($alwaysNotify || $isNewDevice) {
                $message = "Device seen on network:\n";
                $message .= "Device Name: " . ($client->name ?? 'Unknown') . "\n";
                $message .= "IP Address: " . $client->ip . "\n";
                $message .= "Hostname: " . ($client->hostname ?? 'N/A') . "\n";
                $message .= "MAC Address: " . $client->mac . "\n";
                $message .= "Connection Type: " . ($client->is_wired ? "Wired" : "Wireless") . "\n";
                $message .= "Network: " . ($client->network ?? 'N/A');

                // Send notification if it's a new device or if alwaysNotify is true
                $notifier->sendNotification($message, $notificationService);
                
                // Update known MACs if it's a new device and rememberNewDevices is true
                if ($isNewDevice && $rememberNewDevices) {
                    $database->updateKnownMacs($client->mac);
                    $knownMacs[] = $client->mac; // Update local cache
                }
            }
        }

        if (!$newDeviceFound) {
            echo "No new devices found on the network.\n";
        }
        
    } catch (Exception $e) {
        echo "An error occurred: " . $e->getMessage() . "\n";
    }

    echo "Checking again in $checkInterval seconds...\n";
    sleep($checkInterval);
}
