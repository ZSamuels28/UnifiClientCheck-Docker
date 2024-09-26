<?php
// Notifier.php

use GuzzleHttp\Client;
use GuzzleHttp\Exception\RequestException;

class Notifier {
    private $telegramBotToken;
    private $telegramChatId;
    private $ntfyUrl;
    private $pushOverToken;
    private $pushOverUser;
    private $pushOverUrl;
    private $pushOverTitle;
    private $slackWebhookUrl;

    public function __construct($telegramBotToken, $telegramChatId, $ntfyUrl, $pushOverToken, $pushOverUser, $pushOverTitle, $slackWebhookUrl) {
        $this->telegramBotToken = $telegramBotToken;
        $this->telegramChatId = $telegramChatId;
        $this->ntfyUrl = $ntfyUrl;
        $this->pushOverToken = $pushOverToken;
        $this->pushOverUser = $pushOverUser;
        $this->pushOverTitle = $pushOverTitle;
        $this->pushOverUrl = "https://api.pushover.net/1/messages.json";
        $this->slackWebhookUrl = $slackWebhookUrl;
    }

    public function sendNotification($message, $notificationService) {
        $client = new Client();
        $maxRetries = 5;
        $retryCount = 0;

        do {
            try {
                if ($notificationService == 'Telegram') {
                    $response = $client->post("https://api.telegram.org/bot{$this->telegramBotToken}/sendMessage", [
                        'json' => [
                            'chat_id' => $this->telegramChatId,
                            'text' => $message
                        ]
                    ]);
                } elseif ($notificationService == 'Ntfy') {
                    $response = $client->post($this->ntfyUrl, [
                        'body' => $message,
                        'headers' => ['Content-Type' => 'text/plain']
                    ]);
                } elseif ($notificationService == 'Pushover') {
                    $response = $client->post($this->pushOverUrl, [
                        'form_params' => [
                            'token' => $this->pushOverToken,
                            'user' => $this->pushOverUser,
                            'title' => $this->pushOverTitle,
                            'message' => $message
                        ]
                    ]);
                }
                elseif ($notificationService == 'Slack') {
                    $response = $client->post($this->slackWebhookUrl, [
                        'json' => [
                            'text' => $message
                        ]
                    ]);
                }
                
                // Exit loop if the request is successful
                break;
            } catch (RequestException $e) {
                $response = $e->getResponse();
                if ($response && $response->getStatusCode() == 429) {
                    $retryAfter = json_decode($response->getBody()->getContents(), true)['parameters']['retry_after'] ?? 1;
                    echo "Rate limited. Retrying after {$retryAfter} seconds\n";
                    sleep($retryAfter);
                    $retryCount++;
                } else {
                    // Print the error message and stop the program for non-rate limit errors
                    echo "An error occurred while sending the notification: " . $e->getMessage();
                    exit(1);
                }
            }
        } while ($retryCount < $maxRetries);

        if ($retryCount == $maxRetries) {
            echo "Failed to send notification after {$maxRetries} retries.\n";
            exit(1);
        }
    }
}
