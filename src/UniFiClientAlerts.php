<?php
require_once(__DIR__ . '/Unifi-API-client/Client.php');
require_once(__DIR__ . '/Unifi-API-client/config.php');
require_once(__DIR__ . '/../vendor/autoload.php');

use GuzzleHttp\Client as GuzzleClient;

// Load environment variables
$knownMacs = explode(',', getenv('KNOWN_MACS')); // MAC addresses are comma-separated
$checkInterval = getenv('CHECK_INTERVAL') ?: 60; // Time in seconds
$telegramBotToken = getenv('TELEGRAM_BOT_TOKEN');
$telegramChatId = getenv('TELEGRAM_CHAT_ID');
$ntfyUrl = getenv('NTFY_URL');
$notificationService = getenv('NOTIFICATION_SERVICE') ?: 'Telegram'; // Default to Telegram if not set

function createUnifiClient() {
    global $controlleruser, $controllerpassword, $controllerurl, $site_id, $controllerversion;
    $unifiClient = new UniFi_API\Client($controlleruser, $controllerpassword, $controllerurl, $site_id, $controllerversion);
    $unifiClient->login();
    return $unifiClient;
}

function sendNotification($message, $telegramClient, $telegramBotToken, $telegramChatId, $ntfyUrl) {
    global $notificationService;

    if ($notificationService == 'Telegram') {
        $telegramClient->post("/bot{$telegramBotToken}/sendMessage", [
            'json' => [
                'chat_id' => $telegramChatId,
                'text' => $message
            ]
        ]);
    } elseif ($notificationService == 'Ntfy') {
        $ntfyClient = new GuzzleClient();
        $ntfyClient->post($ntfyUrl, [
                'body' => $message
        ]);
    }
}

$unifiClient = createUnifiClient();

$telegramClient = new GuzzleClient([
    'base_uri' => 'https://api.telegram.org'
]);

while (true) {
    $clients = $unifiClient->list_clients();
    
    if ($clients === false) {
        echo "Error: Failed to retrieve clients from the UniFi Controller. Retrying in 60 seconds...\n";
        sleep(60);
        $unifiClient->logout();
        $unifiClient = createUnifiClient();
        continue;
    } elseif (is_array($clients) && count($clients) > 0) {
        $newDeviceFound = false;

        foreach ($clients as $client) {
            if (!in_array($client->mac, $knownMacs)) {
                $newDeviceFound = true;
                echo "New device found. Sending a notification.\n";
                $message = "New device seen on network\n";
                $message .= "Device Name: " . ($client->name ?? 'Unknown') . "\n";
                $message .= "IP Address: " . $client->ip . "\n";
                $message .= "Hostname: " . ($client->hostname ?? 'N/A') . "\n";
                $message .= "MAC Address: " . $client->mac . "\n";
                $message .= "Connection Type: " . ($client->is_wired ? "Wired" : "Wireless") . "\n";
                $message .= "Network: " . ($client->network ?? 'N/A');

                sendNotification($message, $telegramClient, $telegramBotToken, $telegramChatId, $ntfyUrl);

                $knownMacs[] = $client->mac;
            }
        }

        if (!$newDeviceFound) {
            echo "No new devices found on the network.\n";
        }
    } else {
        echo "No clients currently connected to the network.\n";
    }

    echo "Checking in {$checkInterval} seconds...\n";
    sleep($checkInterval);
}