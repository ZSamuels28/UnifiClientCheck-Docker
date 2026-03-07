package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zsamuels28/unificlientalerts/internal/config"
	"github.com/zsamuels28/unificlientalerts/internal/database"
	"github.com/zsamuels28/unificlientalerts/internal/notifier"
	"github.com/zsamuels28/unificlientalerts/internal/unifi"
)

func formatMessage(client *unifi.NetworkClient, teleport bool) string {
	if teleport && client.Type == "TELEPORT" {
		return fmt.Sprintf(
			"Teleport device seen on network:\nName: %s\nIP Address: %s\nID: %s",
			unifi.StrOrDefault(client.Name, "Unknown"),
			client.IP,
			client.ID,
		)
	}

	networkVal := client.Network
	if teleport {
		networkVal = client.NetworkName
	}

	return fmt.Sprintf(
		"Device seen on network:\nDevice Name: %s\nIP Address: `%s`\nHostname: %s\nMAC Address: `%s`\nConnection Type: %s\nNetwork: %s",
		unifi.StrOrDefault(client.Name, "Unknown"),
		unifi.StrOrDefault(client.IP, "Unassigned"),
		unifi.StrOrDefault(client.Hostname, "N/A"),
		client.Mac,
		unifi.WiredStr(client.IsWired),
		unifi.StrOrDefault(networkVal, "N/A"),
	)
}

func reconnect(uc *unifi.UnifiClient) {
	uc.Logout()
	if err := uc.Login(); err != nil {
		log.Printf("Reconnect failed: %v", err)
	}
}

func main() {
	cfg := config.Load()

	validServices := map[string]bool{"Telegram": true, "Ntfy": true, "Pushover": true, "Slack": true, "Gotify": true, "Discord": true, "MQTT": true, "Webhook": true}
	if !validServices[cfg.NotificationService] {
		log.Fatalf("Error: Invalid notification service %q. Must be Telegram, Ntfy, Pushover, Slack, Gotify, Discord, MQTT, or Webhook.", cfg.NotificationService)
	}

	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	knownMacsList, err := db.LoadKnownMacs(cfg.KnownMacs)
	if err != nil {
		log.Fatalf("Failed to load known MACs: %v", err)
	}
	knownMacs := make(map[string]struct{})
	for _, mac := range knownMacsList {
		knownMacs[mac] = struct{}{}
	}

	notif := notifier.New(notifier.Config{
		TelegramBotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
		NtfyURL:           os.Getenv("NTFY_URL"),
		NtfyUser:          os.Getenv("NTFY_USER"),
		NtfyPassword:      os.Getenv("NTFY_PASSWORD"),
		NtfyEmail:         os.Getenv("NTFY_EMAIL"),
		PushoverToken:     os.Getenv("PUSHOVER_TOKEN"),
		PushoverUser:      os.Getenv("PUSHOVER_USER"),
		PushoverTitle:     os.Getenv("PUSHOVER_TITLE"),
		PushoverSound:     os.Getenv("PUSHOVER_SOUND"),
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		GotifyURL:         os.Getenv("GOTIFY_URL"),
		GotifyToken:       os.Getenv("GOTIFY_TOKEN"),
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
		MQTTBroker:        os.Getenv("MQTT_BROKER"),
		MQTTTopic:         os.Getenv("MQTT_TOPIC"),
		MQTTUser:          os.Getenv("MQTT_USER"),
		MQTTPassword:      os.Getenv("MQTT_PASSWORD"),
		MQTTClientID:      os.Getenv("MQTT_CLIENT_ID"),
		WebhookURL:        os.Getenv("WEBHOOK_URL"),
		WebhookSecret:     os.Getenv("WEBHOOK_SECRET"),
	})

	if cfg.NotificationService == "MQTT" {
		if err := notif.ConnectMQTT(); err != nil {
			log.Fatalf("Failed to connect to MQTT broker: %v", err)
		}
	}

	uc := unifi.NewUnifiClient(
		os.Getenv("UNIFI_CONTROLLER_USER"),
		os.Getenv("UNIFI_CONTROLLER_PASSWORD"),
		os.Getenv("UNIFI_CONTROLLER_URL"),
		os.Getenv("UNIFI_SITE_ID"),
	)

	if err := uc.Login(); err != nil {
		log.Fatalf("Failed to login to UniFi controller: %v", err)
	}

	// pendingMacs holds new devices waiting for an IP assignment before notification.
	pendingMacs := make(map[string]struct{})

	for {
		var clients []unifi.NetworkClient
		var fetchErr error

		if cfg.TeleportNotifications {
			clients, fetchErr = uc.ListTeleportClients()
		} else {
			clients, fetchErr = uc.ListClients()
		}

		if fetchErr != nil {
			log.Printf("Failed to retrieve clients, retrying in 60 seconds: %v", fetchErr)
			time.Sleep(60 * time.Second)
			reconnect(uc)
			continue
		}

		if len(clients) == 0 {
			log.Println("No devices currently connected to the network.")
		} else {
			newDeviceFound := false

			for i := range clients {
				client := &clients[i]
				identifier := client.Identifier(cfg.TeleportNotifications)
				_, isKnown := knownMacs[identifier]
				_, isPending := pendingMacs[identifier]
				isNew := !isKnown && !isPending

				if !isKnown && !isPending && cfg.RequireIP && client.IP == "" {
					log.Printf("New device %s has no IP yet; holding notification until IP is assigned.", identifier)
					pendingMacs[identifier] = struct{}{}
					continue
				}

				if isPending {
					if client.IP == "" {
						continue
					}
					log.Printf("Pending device %s now has IP %s; sending notification.", identifier, client.IP)
					delete(pendingMacs, identifier)
					isNew = true
				}

				if isNew {
					log.Println("New device found; sending notification.")
					newDeviceFound = true
				}

				if cfg.AlwaysNotify || isNew {
					switch cfg.NotificationService {
					case "MQTT":
						if err := notif.SendMQTTNotification(client); err != nil {
							log.Printf("Failed to send MQTT notification: %v", err)
						}
					case "Webhook":
						if err := notif.SendWebhookNotification(client); err != nil {
							log.Printf("Failed to send webhook notification: %v", err)
						}
					default:
						message := formatMessage(client, cfg.TeleportNotifications)
						if err := notif.SendNotification(message, cfg.NotificationService); err != nil {
							log.Printf("Failed to send notification: %v", err)
						}
					}

					if isNew && cfg.RememberNewDevices {
						if err := db.UpdateKnownMacs(identifier); err != nil {
							log.Printf("Failed to store MAC %s: %v", identifier, err)
						}
						knownMacs[identifier] = struct{}{}
					}
				}
			}

			if !newDeviceFound {
				log.Println("No new devices found on the network.")
			}
		}

		if cfg.RemoveOldDevices {
			if err := db.RemoveOldMacs(clients, cfg.RemoveDelay); err != nil {
				log.Printf("Error removing old MACs: %v", err)
			}
			knownMacsList, err := db.LoadKnownMacs(cfg.KnownMacs)
			if err != nil {
				log.Printf("Error reloading known MACs: %v", err)
			} else {
				knownMacs = make(map[string]struct{})
				for _, mac := range knownMacsList {
					knownMacs[mac] = struct{}{}
				}
			}
		}

		log.Printf("Checking again in %d seconds...", cfg.CheckInterval)
		time.Sleep(time.Duration(cfg.CheckInterval) * time.Second)
	}
}
