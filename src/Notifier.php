<?php
// Notifier.php

use GuzzleHttp\Client;
use GuzzleHttp\Exception\RequestException;

class Notifier {
    private $telegramBotToken;
    private $telegramChatId;
    private $ntfyUrl;

    public function __construct($telegramBotToken, $telegramChatId, $ntfyUrl) {
        $this->telegramBotToken = $telegramBotToken;
        $this->telegramChatId = $telegramChatId;
        $this->ntfyUrl = $ntfyUrl;
    }

    public function sendNotification($message, $notificationService) {
        try {
            if ($notificationService == 'Telegram') {
                $client = new Client(['base_uri' => 'https://api.telegram.org']);
                $response = $client->post("/bot{$this->telegramBotToken}/sendMessage", [
                    'json' => [
                        'chat_id' => $this->telegramChatId,
                        'text' => $message
                    ]
                ]);
            } elseif ($notificationService == 'Ntfy') {
                $client = new Client(); // No base URI for Ntfy; use the full URL in the post request.
                $response = $client->post($this->ntfyUrl, [
                    'body' => $message,
                    'headers' => ['Content-Type' => 'text/plain'] // Ensure correct content type for Ntfy.
                ]);
            }
        } catch (RequestException $e) {
            // Print the error message and stop the program
            echo "An error occurred while sending the notification: " . $e->getMessage();
            exit(1);
        }
    }
}
