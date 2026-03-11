package notifier

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/zsamuels28/unificlientalerts/internal/unifi"
)

// mqttPublishTimeout is the maximum time to wait for an MQTT publish to complete.
const mqttPublishTimeout = 10 * time.Second

// Config holds all configuration for the Notifier.
type Config struct {
	TelegramBotToken  string
	TelegramChatID    string
	NtfyURL           string
	NtfyUser          string
	NtfyPassword      string
	NtfyEmail         string
	PushoverToken     string
	PushoverUser      string
	PushoverTitle     string
	PushoverSound     string
	SlackWebhookURL   string
	GotifyURL         string
	GotifyToken       string
	DiscordWebhookURL string
	MQTTBroker        string
	MQTTTopic         string
	MQTTUser          string
	MQTTPassword      string
	MQTTClientID      string
	WebhookURL        string
	WebhookSecret     string
}

// Notifier sends alerts via Telegram, Ntfy, Pushover, Slack, Gotify, Discord, MQTT, or Webhook.
type Notifier struct {
	cfg      Config
	mqttConn mqtt.Client
	client   *http.Client
}

func New(cfg Config) *Notifier {
	return &Notifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// ConnectMQTT establishes a persistent connection to the MQTT broker with LWT.
func (n *Notifier) ConnectMQTT() error {
	clientID := n.cfg.MQTTClientID
	if clientID == "" {
		clientID = "unificlientalerts"
	}
	statusTopic := n.cfg.MQTTTopic + "/status"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(n.cfg.MQTTBroker)
	opts.SetClientID(clientID)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetWill(statusTopic, "offline", 1, true)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		c.Publish(statusTopic, 1, true, "online")
	})
	if n.cfg.MQTTUser != "" {
		opts.SetUsername(n.cfg.MQTTUser)
		opts.SetPassword(n.cfg.MQTTPassword)
	}

	n.mqttConn = mqtt.NewClient(opts)
	token := n.mqttConn.Connect()
	token.Wait()
	return token.Error()
}

// DisconnectMQTT cleanly disconnects the MQTT client, publishing the offline status via LWT.
func (n *Notifier) DisconnectMQTT() {
	if n.mqttConn != nil && n.mqttConn.IsConnected() {
		// Publish offline status before disconnecting
		statusTopic := n.cfg.MQTTTopic + "/status"
		token := n.mqttConn.Publish(statusTopic, 1, true, "offline")
		token.WaitTimeout(3 * time.Second)
		n.mqttConn.Disconnect(1000) // 1s quiesce period
	}
}

type mqttPayload struct {
	Name           string `json:"name"`
	MAC            string `json:"mac"`
	IP             string `json:"ip"`
	Hostname       string `json:"hostname"`
	ConnectionType string `json:"connection_type"`
	Network        string `json:"network"`
	Timestamp      string `json:"timestamp"`
}

// SendMQTTNotification publishes a JSON device alert to the configured MQTT topic.
func (n *Notifier) SendMQTTNotification(client *unifi.NetworkClient) error {
	network := client.NetworkName
	if network == "" {
		network = client.Network
	}
	payload := mqttPayload{
		Name:           client.DisplayName(),
		MAC:            client.Mac,
		IP:             unifi.StrOrDefault(client.IP, "Unassigned"),
		Hostname:       unifi.StrOrDefault(client.Hostname, "N/A"),
		ConnectionType: unifi.WiredStr(client.IsWired),
		Network:        unifi.StrOrDefault(network, "N/A"),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal mqtt payload: %w", err)
	}
	token := n.mqttConn.Publish(n.cfg.MQTTTopic, 1, false, body)
	if !token.WaitTimeout(mqttPublishTimeout) {
		return fmt.Errorf("MQTT publish timed out after %s", mqttPublishTimeout)
	}
	return token.Error()
}

const maxRetries = 5

// drainBody reads and discards the remaining response body (up to 64KB) so the
// underlying TCP connection can be reused by the HTTP client pool.
func drainBody(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
}

// validateHTTPStatus checks if the response status is successful (2xx).
func validateHTTPStatus(statusCode int, service string) error {
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("%s returned HTTP %d", service, statusCode)
	}
	return nil
}

// SendNotification delivers message via the configured service, retrying on 429.
func (n *Notifier) SendNotification(message, service string) error {
	for range maxRetries {
		retryAfter, err := n.sendOnce(message, service)
		if err == nil {
			return nil
		}
		if retryAfter > 0 {
			log.Printf("Rate limited by %s; retrying after %d seconds", service, retryAfter)
			time.Sleep(time.Duration(retryAfter) * time.Second)
			continue
		}
		return fmt.Errorf("notification failed: %w", err)
	}
	return fmt.Errorf("failed to send notification after %d retries", maxRetries)
}

// sendOnce attempts one delivery. Returns (retryAfterSeconds, error).
// retryAfterSeconds > 0 means the caller should retry after that delay.
func (n *Notifier) sendOnce(message, service string) (int, error) {
	switch service {
	case "Telegram":
		return n.sendTelegram(message)
	case "Ntfy":
		return n.sendNtfy(message)
	case "Pushover":
		return n.sendPushover(message)
	case "Slack":
		return n.sendSlack(message)
	case "Gotify":
		return n.sendGotify(message)
	case "Discord":
		return n.sendDiscord(message)
	default:
		return 0, fmt.Errorf("unknown notification service: %s", service)
	}
}

func (n *Notifier) sendTelegram(message string) (int, error) {
	data := map[string]string{
		"chat_id": n.cfg.TelegramChatID,
		"text":    message,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal telegram request: %w", err)
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", n.cfg.TelegramBotToken)
	resp, err := n.client.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	defer drainBody(resp)

	if resp.StatusCode == 429 {
		var result struct {
			Parameters struct {
				RetryAfter int `json:"retry_after"`
			} `json:"parameters"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			// If we can't decode, default to 1 second retry
			return 1, fmt.Errorf("rate limited")
		}
		retryAfter := result.Parameters.RetryAfter
		if retryAfter == 0 {
			retryAfter = 1
		}
		return retryAfter, fmt.Errorf("rate limited")
	}

	if err := validateHTTPStatus(resp.StatusCode, "telegram"); err != nil {
		return 0, err
	}
	return 0, nil
}

var deviceNameRe = regexp.MustCompile(`(?:Device Name|Name): (.+)`)

func (n *Notifier) sendNtfy(message string) (int, error) {
	deviceName := "Unknown Device"
	if m := deviceNameRe.FindStringSubmatch(message); m != nil {
		deviceName = strings.TrimSpace(m[1])
	}

	deviceType := "Device"
	if strings.Contains(message, "Teleport device") {
		deviceType = "Teleport Device"
	}

	req, err := http.NewRequest("POST", n.cfg.NtfyURL, strings.NewReader(message))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.Header.Set("Title", fmt.Sprintf("New %s: %s", deviceType, deviceName))
	req.Header.Set("Priority", "default")
	req.Header.Set("Tags", "computer,new_device")

	if n.cfg.NtfyEmail != "" {
		req.Header.Set("X-Email", n.cfg.NtfyEmail)
	}

	if n.cfg.NtfyUser != "" && n.cfg.NtfyPassword != "" {
		req.SetBasicAuth(n.cfg.NtfyUser, n.cfg.NtfyPassword)
	}

	resp, err := n.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	defer drainBody(resp)

	if resp.StatusCode == 429 {
		retryAfter := 60 // default to 60 seconds
		if retryAfterHeader := resp.Header.Get("Retry-After"); retryAfterHeader != "" {
			if seconds, err := strconv.Atoi(retryAfterHeader); err == nil && seconds > 0 {
				retryAfter = seconds
			}
		}
		return retryAfter, fmt.Errorf("rate limited")
	}

	if err := validateHTTPStatus(resp.StatusCode, "ntfy"); err != nil {
		return 0, err
	}
	return 0, nil
}

func (n *Notifier) sendPushover(message string) (int, error) {
	form := url.Values{
		"token":   {n.cfg.PushoverToken},
		"user":    {n.cfg.PushoverUser},
		"title":   {n.cfg.PushoverTitle},
		"message": {message},
	}
	if n.cfg.PushoverSound != "" {
		form.Set("sound", n.cfg.PushoverSound)
	}

	resp, err := n.client.PostForm("https://api.pushover.net/1/messages.json", form)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	defer drainBody(resp)

	if err := validateHTTPStatus(resp.StatusCode, "pushover"); err != nil {
		return 0, err
	}
	return 0, nil
}

func (n *Notifier) sendSlack(message string) (int, error) {
	data := map[string]string{"text": message}
	body, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal slack request: %w", err)
	}

	resp, err := n.client.Post(n.cfg.SlackWebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	defer drainBody(resp)

	if err := validateHTTPStatus(resp.StatusCode, "slack"); err != nil {
		return 0, err
	}
	return 0, nil
}

func (n *Notifier) sendGotify(message string) (int, error) {
	title := "New Device on Network"
	if m := deviceNameRe.FindStringSubmatch(message); m != nil {
		title = fmt.Sprintf("New Device: %s", strings.TrimSpace(m[1]))
	}

	data := map[string]any{
		"title":    title,
		"message":  message,
		"priority": 5,
	}
	body, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal gotify request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/message", strings.TrimRight(n.cfg.GotifyURL, "/"))
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gotify-Key", n.cfg.GotifyToken)

	resp, err := n.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	defer drainBody(resp)

	if err := validateHTTPStatus(resp.StatusCode, "gotify"); err != nil {
		return 0, err
	}
	return 0, nil
}

type webhookPayload struct {
	Event          string `json:"event"`
	Name           string `json:"name"`
	MAC            string `json:"mac"`
	IP             string `json:"ip"`
	Hostname       string `json:"hostname"`
	ConnectionType string `json:"connection_type"`
	Network        string `json:"network"`
	Timestamp      string `json:"timestamp"`
}

func (n *Notifier) SendWebhookNotification(client *unifi.NetworkClient) error {
	network := client.Network
	if network == "" {
		network = client.NetworkName
	}
	payload := webhookPayload{
		Event:          "client_joined",
		Name:           client.DisplayName(),
		MAC:            client.Mac,
		IP:             unifi.StrOrDefault(client.IP, "Unassigned"),
		Hostname:       unifi.StrOrDefault(client.Hostname, "N/A"),
		ConnectionType: unifi.WiredStr(client.IsWired),
		Network:        unifi.StrOrDefault(network, "N/A"),
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}
	req, err := http.NewRequest("POST", n.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if n.cfg.WebhookSecret != "" {
		req.Header.Set("Authorization", "Bearer "+n.cfg.WebhookSecret)
	}
	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	defer drainBody(resp)
	return validateHTTPStatus(resp.StatusCode, "webhook")
}

func (n *Notifier) sendDiscord(message string) (int, error) {
	data := map[string]string{"content": message}
	body, err := json.Marshal(data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal discord request: %w", err)
	}

	resp, err := n.client.Post(n.cfg.DiscordWebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	defer drainBody(resp)

	if resp.StatusCode == 429 {
		var result struct {
			RetryAfter float64 `json:"retry_after"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return 1, fmt.Errorf("rate limited")
		}
		retryAfter := int(result.RetryAfter) + 1
		return retryAfter, fmt.Errorf("rate limited")
	}

	if err := validateHTTPStatus(resp.StatusCode, "discord"); err != nil {
		return 0, err
	}
	return 0, nil
}
