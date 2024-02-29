<?php
// Notifier.php

use GuzzleHttp\Client;

class Notifier {
    private $client;
    private $telegramBotToken;
    private $telegramChatId;
    private $ntfyUrl;

    public function __construct($telegramBotToken, $telegramChatId, $ntfyUrl) {
        $this->client = new Client(['base_uri' => 'https://api.telegram.org']);
        $this->telegramBotToken = $telegramBotToken;
        $this->telegramChatId = $telegramChatId;
        $this->ntfyUrl = $ntfyUrl;
    }

    public function sendNotification($message, $notificationService) {
        if ($notificationService == 'Telegram') {
            $this->client->post("/bot{$this->telegramBotToken}/sendMessage", [
                'json' => [
                    'chat_id' => $this->telegramChatId,
                    'text' => $message
                ]
            ]);
        } elseif ($notificationService == 'Ntfy') {
            $this->client->post($this->ntfyUrl, ['body' => $message]);
        }
    }
}