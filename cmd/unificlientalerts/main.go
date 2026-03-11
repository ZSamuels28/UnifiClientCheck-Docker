package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/zsamuels28/unificlientalerts/internal/config"
	"github.com/zsamuels28/unificlientalerts/internal/database"
	"github.com/zsamuels28/unificlientalerts/internal/notifier"
	"github.com/zsamuels28/unificlientalerts/internal/unifi"
)

// pendingMaxAge is the maximum time a device can stay in pendingMacs without
// getting an IP before it is cleaned up to prevent unbounded map growth.
const pendingMaxAge = 10 * time.Minute

// logVerbose logs only when VERBOSE=true. Use for diagnostic/polling details.
// Normal operational messages (device detected, fallback triggered, etc.) use log.Printf directly.
func logVerbose(cfg config.Config, format string, args ...any) {
	if cfg.Verbose {
		log.Printf(format, args...)
	}
}

func formatMessage(client *unifi.NetworkClient, teleport bool) string {
	if teleport && client.Type == "TELEPORT" {
		return fmt.Sprintf(
			"Teleport device seen on network:\nName: %s\nIP Address: `%s`\nID: %s",
			client.DisplayName(),
			client.IP,
			client.ID,
		)
	}

	return fmt.Sprintf(
		"Device seen on network:\nDevice Name: %s\nIP Address: `%s`\nHostname: %s\nMAC Address: `%s`\nConnection Type: %s\nNetwork: %s",
		client.DisplayName(),
		unifi.StrOrDefault(client.IP, "Unassigned"),
		unifi.StrOrDefault(client.Hostname, "N/A"),
		unifi.StrOrDefault(client.Mac, "N/A"),
		unifi.WiredStr(client.IsWired),
		unifi.StrOrDefault(client.NetworkName, "N/A"),
	)
}

// sendAlert dispatches a notification for a client via the configured service.
func sendAlert(client *unifi.NetworkClient, cfg config.Config, notif *notifier.Notifier) {
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
		message := formatMessage(client, client.Type == "TELEPORT")
		if err := notif.SendNotification(message, cfg.NotificationService); err != nil {
			log.Printf("Failed to send notification: %v", err)
		}
	}
}

// recordDevice stores the identifier in the database and marks it known in-memory.
func recordDevice(identifier string, cfg config.Config, db *database.Database, knownMacs map[string]struct{}) {
	if cfg.RememberNewDevices {
		if err := db.UpdateKnownMacs(identifier); err != nil {
			log.Printf("Failed to store MAC %s: %v", identifier, err)
		}
	}
	knownMacs[identifier] = struct{}{}
}

// runCheck fetches the current client list and processes notifications.
// Returns the client list and whether any new device was found.
// Returns nil clients on fetch error (after internally retrying once).
func runCheck(
	uc *unifi.UnifiClient,
	cfg config.Config,
	db *database.Database,
	notif *notifier.Notifier,
	knownMacs map[string]struct{},
	pendingMacs map[string]time.Time,
	source string,
	quiet bool,
) (clients []unifi.NetworkClient, newDeviceFound bool) {
	var fetchErr error
	clients, fetchErr = uc.ListClients()

	if fetchErr != nil {
		log.Printf("Failed to retrieve clients, reconnecting: %v", fetchErr)
		if err := uc.Login(); err != nil {
			log.Printf("Reconnect failed: %v", err)
		}
		return nil, false
	}

	if len(clients) == 0 {
		log.Printf("No devices currently connected to the network.")
		return clients, false
	}

	for i := range clients {
		client := &clients[i]
		identifier := client.Identifier(true)
		_, isKnown := knownMacs[identifier]
		_, isPending := pendingMacs[identifier]
		isNew := !isKnown && !isPending

		if isKnown {
			logVerbose(cfg, "Device %s already known; skipping.", identifier)
			continue
		}

		if !isPending && cfg.RequireIP && client.IP == "" {
			log.Printf("New device %s has no IP yet; holding notification until IP is assigned.", identifier)
			pendingMacs[identifier] = time.Now()
			continue
		}

		if isPending {
			if client.IP == "" {
				continue
			}
			log.Printf("Pending device %s now has IP %s; sending notification (source: %s).", identifier, client.IP, source)
			delete(pendingMacs, identifier)
			isNew = true
		}

		if isNew {
			if client.IP != "" {
				log.Printf("New device %s detected with IP %s (source: %s); sending notification.", identifier, client.IP, source)
			} else {
				log.Printf("New device %s detected (source: %s); sending notification.", identifier, source)
			}
			newDeviceFound = true
		}

		if cfg.AlwaysNotify || isNew {
			sendAlert(client, cfg, notif)

			if isNew {
				recordDevice(identifier, cfg, db, knownMacs)
			}
		}
	}

	if !newDeviceFound && !quiet {
		logVerbose(cfg, "No new devices found on the network.")
	}
	return clients, newDeviceFound
}

// waitForDeviceIP polls for a new device to get an IP address after a WebSocket event.
// Retries with backoff: 3s, 6s, 12s, 24s.
// During each backoff sleep, teleport events are dispatched immediately and a new
// wired/wireless WS trigger breaks the sleep early so the poll runs right away.
// When a new device with an IP is found, onFound is called immediately so the alert
// is sent and the device recorded before runCheck runs — preventing a duplicate
// "source: fallback" notification for the same device.
// Returns: clients (full list), newDeviceWithIP (whether a new device got IP), newDevicesWithoutIP (devices that timed out)
func waitForDeviceIP(
	ctx context.Context,
	uc *unifi.UnifiClient,
	cfg config.Config,
	knownMacs map[string]struct{},
	pendingMacs map[string]time.Time,
	trigger <-chan struct{},
	teleportCh <-chan unifi.NetworkClient,
	onTeleport func(unifi.NetworkClient),
	onFound func(*unifi.NetworkClient),
) ([]unifi.NetworkClient, bool, []unifi.NetworkClient) {
	backoffs := []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second, 24 * time.Second}
	var lastClients []unifi.NetworkClient
	var newDevicesNoIP []unifi.NetworkClient
	seenNoIP := make(map[string]struct{}) // deduplicate across retry attempts

	// Try to fetch clients immediately, then retry with backoff
	for attempt := 0; attempt <= len(backoffs); attempt++ {
		if attempt > 0 {
			// Wait for the backoff duration. During the wait:
			//   - Teleport events are dispatched immediately via onTeleport.
			//   - A new WS trigger breaks the sleep early so the next poll runs now.
			//   - Context cancellation (shutdown) breaks out immediately.
			timer := time.NewTimer(backoffs[attempt-1])
		waitLoop:
			for {
				select {
				case <-ctx.Done():
					timer.Stop()
					return lastClients, false, newDevicesNoIP
				case <-timer.C:
					break waitLoop
				case client := <-teleportCh:
					onTeleport(client)
				case <-trigger:
					logVerbose(cfg, "New WS event during IP wait; polling immediately.")
					break waitLoop
				}
			}
			// Stop the timer and drain its channel if it fired concurrently.
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		}

		var fetchErr error
		lastClients, fetchErr = uc.ListClients()

		if fetchErr != nil {
			logVerbose(cfg, "Failed to fetch clients (attempt %d): %v", attempt+1, fetchErr)
			continue
		}

		// Look for new devices with IPs
		for i := range lastClients {
			client := &lastClients[i]
			identifier := client.Identifier(true)
			_, isKnown := knownMacs[identifier]
			_, isPending := pendingMacs[identifier]

			// If this is a new device and has an IP, notify immediately so runCheck
			// sees it as already known and does not send a duplicate notification.
			if !isKnown && !isPending && client.IP != "" {
				if attempt > 0 {
					attemptsStr := "attempt"
					if attempt > 1 {
						attemptsStr = "attempts"
					}
					log.Printf("New device %s got IP %s after %d %s (source: WebSocket); sending notification.", identifier, client.IP, attempt, attemptsStr)
				} else {
					log.Printf("New device %s detected with IP %s (source: WebSocket); sending notification.", identifier, client.IP)
				}
				onFound(client)
				return lastClients, true, newDevicesNoIP
			}

			// Track new devices without IP, deduplicated across attempts
			if !isKnown && !isPending && client.IP == "" {
				if _, seen := seenNoIP[identifier]; !seen {
					seenNoIP[identifier] = struct{}{}
					newDevicesNoIP = append(newDevicesNoIP, *client)
				}
			}
		}

		logVerbose(cfg, "No new device with IP yet (attempt %d/%d)", attempt+1, len(backoffs)+1)
	}

	// After all retries exhausted, return clients and devices that timed out without IP
	return lastClients, false, newDevicesNoIP
}

// resetFallback drains any pending trigger/fallback signals and resets the ticker
// so the next periodic check is a full interval away from the current WS event.
func resetFallback(trigger <-chan struct{}, fallbackCh <-chan time.Time, fallback *time.Ticker, fallbackInterval int) {
	select {
	case <-trigger:
	default:
	}
	select {
	case <-fallbackCh:
	default:
	}
	fallback.Reset(time.Duration(fallbackInterval) * time.Second)
}

// removeOldAndReload runs RemoveOldMacs then reloads knownMacs from the database.
// Called after each check cycle when REMOVE_OLD_DEVICES is enabled.
func removeOldAndReload(cfg config.Config, db *database.Database, clients []unifi.NetworkClient, knownMacs *map[string]struct{}) {
	if err := db.RemoveOldMacs(clients, cfg.RemoveDelay); err != nil {
		log.Printf("Error removing old MACs: %v", err)
	}
	list, err := db.LoadKnownMacs(cfg.KnownMacs)
	if err != nil {
		log.Printf("Error reloading known MACs: %v", err)
		return
	}
	m := make(map[string]struct{}, len(list))
	for _, mac := range list {
		m[mac] = struct{}{}
	}
	*knownMacs = m
}

// cleanExpiredPending removes entries from pendingMacs that have been waiting
// longer than pendingMaxAge. This prevents unbounded map growth from transient
// devices that connect briefly without ever getting an IP.
func cleanExpiredPending(cfg config.Config, pendingMacs map[string]time.Time) {
	cutoff := time.Now().Add(-pendingMaxAge)
	for id, added := range pendingMacs {
		if added.Before(cutoff) {
			log.Printf("Expiring stale pending device %s (no IP for %s)", id, pendingMaxAge)
			delete(pendingMacs, id)
		}
	}
}

// validateConfig checks that all required environment variables are set and
// prints clear fatal errors if any are missing — before any network calls are made.
func validateConfig(cfg config.Config) {
	required := func(name, value, context string) {
		if value == "" {
			log.Fatalf("Missing required environment variable: %s (%s)", name, context)
		}
	}

	required("UNIFI_CONTROLLER_URL", os.Getenv("UNIFI_CONTROLLER_URL"), "URL of your UniFi controller, e.g. https://192.168.1.1:443")
	required("UNIFI_CONTROLLER_USER", os.Getenv("UNIFI_CONTROLLER_USER"), "local UniFi account username")
	required("UNIFI_CONTROLLER_PASSWORD", os.Getenv("UNIFI_CONTROLLER_PASSWORD"), "local UniFi account password")

	switch cfg.NotificationService {
	case "Telegram":
		required("TELEGRAM_BOT_TOKEN", os.Getenv("TELEGRAM_BOT_TOKEN"), "required for Telegram notifications")
		required("TELEGRAM_CHAT_ID", os.Getenv("TELEGRAM_CHAT_ID"), "required for Telegram notifications")
	case "Ntfy":
		required("NTFY_URL", os.Getenv("NTFY_URL"), "required for Ntfy notifications, e.g. https://ntfy.sh/mytopic")
	case "Pushover":
		required("PUSHOVER_TOKEN", os.Getenv("PUSHOVER_TOKEN"), "required for Pushover notifications")
		required("PUSHOVER_USER", os.Getenv("PUSHOVER_USER"), "required for Pushover notifications")
	case "Slack":
		required("SLACK_WEBHOOK_URL", os.Getenv("SLACK_WEBHOOK_URL"), "required for Slack notifications")
	case "Gotify":
		required("GOTIFY_URL", os.Getenv("GOTIFY_URL"), "required for Gotify notifications")
		required("GOTIFY_TOKEN", os.Getenv("GOTIFY_TOKEN"), "required for Gotify notifications")
	case "Discord":
		required("DISCORD_WEBHOOK_URL", os.Getenv("DISCORD_WEBHOOK_URL"), "required for Discord notifications")
	case "MQTT":
		required("MQTT_BROKER", os.Getenv("MQTT_BROKER"), "required for MQTT notifications, e.g. tcp://192.168.1.1:1883")
		required("MQTT_TOPIC", os.Getenv("MQTT_TOPIC"), "required for MQTT notifications, e.g. unifi/new_device")
	case "Webhook":
		required("WEBHOOK_URL", os.Getenv("WEBHOOK_URL"), "required for Webhook notifications")
	}
}

func main() {
	cfg := config.Load()

	validServices := map[string]bool{"Telegram": true, "Ntfy": true, "Pushover": true, "Slack": true, "Gotify": true, "Discord": true, "MQTT": true, "Webhook": true}
	if !validServices[cfg.NotificationService] {
		log.Fatalf("Error: Invalid notification service %q. Must be Telegram, Ntfy, Pushover, Slack, Gotify, Discord, MQTT, or Webhook.", cfg.NotificationService)
	}

	validateConfig(cfg)

	// Log the forget timeout if enabled
	if cfg.RemoveOldDevices {
		log.Printf("REMOVE_OLD_DEVICES enabled: forgetting devices absent for %s", config.HumanDuration(cfg.RemoveDelay))
	}

	// Backward compatibility: check old database path if new one doesn't exist
	dbPath := cfg.DatabasePath
	oldPath := "/usr/src/myapp/knownMacs.db"
	if _, err := os.Stat(oldPath); err == nil {
		if _, err := os.Stat(dbPath); err != nil {
			// Old database exists, new one doesn't - use old path for backward compatibility
			log.Printf("Found database at %s (old path), using that for backward compatibility. Consider moving to %s.", oldPath, dbPath)
			dbPath = oldPath
		}
	}

	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	// db.Close() is called explicitly in the shutdown block below;
	// no defer here to avoid double-close.

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

	// Set up context for graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown (Docker sends SIGTERM on stop).
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-quit
		log.Printf("Received signal %s; shutting down.", sig)
		cancel()
	}()

	// pendingMacs holds new devices waiting for an IP assignment before notification.
	// Values are timestamps for expiration of stale entries.
	pendingMacs := make(map[string]time.Time)

	// trigger is buffered so the WebSocket goroutine never blocks.
	// Pre-load one signal so the first check runs immediately on startup.
	trigger := make(chan struct{}, 1)
	trigger <- struct{}{}

	// retryCh receives a single delayed signal when a WS-triggered runCheck found
	// nothing. Unlike trigger, it skips waitForDeviceIP and goes straight to runCheck,
	// keeping the main loop unblocked during the wait.
	retryCh := make(chan struct{}, 1)

	// teleportCh carries Teleport client data directly from the WS stream.
	// Buffered so the WS goroutine never blocks; the main loop deduplicates via knownMacs.
	teleportCh := make(chan unifi.NetworkClient, 8)

	// Start the WebSocket listener. Regular client connections fire onEvent (→ trigger).
	// Teleport client:sync events fire onTeleportSync (→ teleportCh) with full payload —
	// no REST API call needed for that path.
	go uc.ListenEvents(ctx, func(msgType string) {
		log.Printf("WebSocket connection event (%s)", msgType)
		select {
		case trigger <- struct{}{}:
		default: // check already queued, skip
		}
	}, func(client unifi.NetworkClient) {
		select {
		case teleportCh <- client:
		default: // already queued; next client:sync (1s later) will retry
		}
	})

	// Determine fallback interval: -1 = disabled, >0 = use that value
	fallbackInterval := cfg.FallbackInterval

	var fallback *time.Ticker
	var fallbackCh <-chan time.Time
	if fallbackInterval > 0 {
		fallback = time.NewTicker(time.Duration(fallbackInterval) * time.Second)
		defer fallback.Stop()
		fallbackCh = fallback.C
	}

	// handleTeleport processes a Teleport client event. Used both in the main select
	// and as the onTeleport callback inside waitForDeviceIP's backoff sleeps.
	handleTeleport := func(client unifi.NetworkClient) {
		identifier := client.Identifier(true)
		if _, known := knownMacs[identifier]; known {
			return
		}
		log.Printf("New device %s detected (source: Teleport); sending notification.", identifier)
		sendAlert(&client, cfg, notif)
		recordDevice(identifier, cfg, db, knownMacs)
	}

	for {
		wsTriggered := false
		wsDeviceFound := false
		wsSkippedNoUnknown := false
		var wsTimeoutDevices []unifi.NetworkClient
		var preCheckClients []unifi.NetworkClient
		checkSource := "fallback"
		select {
		case <-ctx.Done():
			goto shutdown
		case <-trigger:
			// Wait for WSEventDelay while draining teleport events so they are not
			// blocked behind the IP-assignment polling window.
			if wsDelay := time.Duration(cfg.WSEventDelay) * time.Second; wsDelay > 0 {
				wsDelayTimer := time.NewTimer(wsDelay)
			wsDelayLoop:
				for {
					select {
					case <-ctx.Done():
						wsDelayTimer.Stop()
						goto shutdown
					case <-wsDelayTimer.C:
						break wsDelayLoop
					case client := <-teleportCh:
						handleTeleport(client)
					case <-trigger:
						logVerbose(cfg, "New WS event during initial delay; proceeding immediately.")
						break wsDelayLoop
					}
				}
				if !wsDelayTimer.Stop() {
					select {
					case <-wsDelayTimer.C:
					default:
					}
				}
			}
			log.Printf("Check triggered by WebSocket event.")
			// Quick pre-check: are there actually unknown devices? If not, skip polling.
			preCheckClients, preCheckErr := uc.ListClients()
			hasUnknownDevices := false
			if preCheckErr == nil {
				for _, client := range preCheckClients {
					identifier := client.Identifier(true)
					_, isKnown := knownMacs[identifier]
					_, isPending := pendingMacs[identifier]
					if !isKnown && !isPending {
						hasUnknownDevices = true
						break
					}
				}
			}

			wsOnFound := func(client *unifi.NetworkClient) {
				sendAlert(client, cfg, notif)
				recordDevice(client.Identifier(true), cfg, db, knownMacs)
			}
			if hasUnknownDevices {
				logVerbose(cfg, "Polling for device IP assignment...")
				_, wsDeviceFound, wsTimeoutDevices = waitForDeviceIP(ctx, uc, cfg, knownMacs, pendingMacs, trigger, teleportCh, handleTeleport, wsOnFound)
			} else if preCheckErr == nil {
				log.Printf("No unknown devices in current client list; skipping notification check.")
				wsSkippedNoUnknown = true
			} else {
				log.Printf("Pre-check failed, attempting poll anyway: %v", preCheckErr)
				_, wsDeviceFound, wsTimeoutDevices = waitForDeviceIP(ctx, uc, cfg, knownMacs, pendingMacs, trigger, teleportCh, handleTeleport, wsOnFound)
			}
			// Send notifications for devices that timed out without IP
			for i := range wsTimeoutDevices {
				client := &wsTimeoutDevices[i]
				identifier := client.Identifier(true)
				log.Printf("Device %s timeout without IP after polling; notifying with IP unavailable.", identifier)
				// Set placeholder values for unavailable IP/network
				client.IP = "IP Unavailable"
				client.Network = "Device not registered to network yet"
				sendAlert(client, cfg, notif)
				recordDevice(identifier, cfg, db, knownMacs)
			}
			checkSource = "WebSocket"
			wsTriggered = true
		case client := <-teleportCh:
			handleTeleport(client)
			continue
		case <-retryCh:
			checkSource = "WebSocket retry"
			log.Printf("WS retry check: device may still be registering.")
		case <-fallbackCh:
			logVerbose(cfg, "Fallback check triggered (interval: %ds).", fallbackInterval)
		}

		// Skip normal check if pre-check found no unknown devices
		if wsTriggered && wsSkippedNoUnknown {
			if fallback != nil {
				resetFallback(trigger, fallbackCh, fallback, fallbackInterval)
			}
			// Still need to handle RemoveOldMacs with preCheckClients
			if cfg.RemoveOldDevices && len(preCheckClients) > 0 {
				removeOldAndReload(cfg, db, preCheckClients, &knownMacs)
			}
			cleanExpiredPending(cfg, pendingMacs)
			continue
		}

		clients, found := runCheck(uc, cfg, db, notif, knownMacs, pendingMacs, checkSource, wsDeviceFound)

		// If a WS event triggered this check but nothing was found yet, the device
		// may still be registering. Schedule a single non-blocking re-check via
		// retryCh rather than sleeping here and stalling the main loop.
		if wsTriggered && !found && !wsDeviceFound && clients != nil {
			go func() {
				select {
				case <-time.After(5 * time.Second):
				case <-ctx.Done():
					return
				}
				select {
				case retryCh <- struct{}{}:
				default: // already a retry queued, skip
				}
			}()
		}

		// After a WS-triggered check (and any retries), reset the fallback ticker so
		// the next periodic check is a full interval away from now — not from when the
		// WS event first fired. Drain any tick that accumulated during the check run.
		if wsTriggered && fallback != nil {
			resetFallback(trigger, fallbackCh, fallback, fallbackInterval)
		}

		if cfg.RemoveOldDevices && clients != nil {
			removeOldAndReload(cfg, db, clients, &knownMacs)
		}

		// Periodically clean up stale pending entries.
		cleanExpiredPending(cfg, pendingMacs)
	}

shutdown:
	log.Println("Shutting down gracefully...")
	if cfg.NotificationService == "MQTT" {
		notif.DisconnectMQTT()
	}
	if err := db.Close(); err != nil {
		log.Printf("Error closing database: %v", err)
	}
	log.Println("Shutdown complete.")
}
