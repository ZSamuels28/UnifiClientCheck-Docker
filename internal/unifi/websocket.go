package unifi

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// wsEvent is the top-level message received from the UniFi WebSocket event stream.
type wsEvent struct {
	Meta struct {
		Message string `json:"message"`
	} `json:"meta"`
	Data []struct {
		// EVT_* event fields (wired/wireless clients)
		Key  string `json:"key"`
		User string `json:"user"`
		// client:sync fields
		Status      string `json:"status"`
		Type        string `json:"type"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		DisplayName string `json:"display_name"`
		IP          string `json:"ip"`
	} `json:"data"`
}

// isConnectionEvent returns true if the message is an "events" message with a
// _Connected key, indicating a wired/wireless client connected.
// Teleport clients are handled separately via client:sync in connectAndListen.
func (e *wsEvent) isConnectionEvent() bool {
	if e.Meta.Message != "events" {
		return false
	}
	for _, d := range e.Data {
		if strings.HasSuffix(d.Key, "_Connected") {
			return true
		}
	}
	return false
}

// ListenEvents connects to the UniFi WebSocket event stream and calls onEvent
// each time a wired/wireless client connects (EVT_*_Connected).
// Teleport client:sync events are dispatched to onTeleportSync with the full
// client payload — no REST API call needed for that path.
// It runs until ctx is cancelled, reconnecting with exponential backoff on failure.
func (uc *UnifiClient) ListenEvents(ctx context.Context, onEvent func(msgType string), onTeleportSync func(NetworkClient)) {
	const (
		initialBackoff = 5 * time.Second
		maxBackoff     = 2 * time.Minute
	)
	backoff := initialBackoff
	consecutiveFailures := 0
	for {
		if err := uc.connectAndListen(ctx, onEvent, onTeleportSync); err != nil {
			if ctx.Err() != nil {
				log.Println("WebSocket: shutting down.")
				return
			}
			log.Printf("WebSocket disconnected: %v", err)
			consecutiveFailures++
		}

		// After several consecutive failures, clear the cached WS URL so the
		// next connection attempt re-probes the controller path (modern vs legacy).
		if consecutiveFailures >= 3 {
			uc.clearWSURL()
			consecutiveFailures = 0
			log.Println("WebSocket: cleared cached URL; will re-probe on next connection.")
		}

		log.Printf("WebSocket: reconnecting in %s...", backoff)
		select {
		case <-ctx.Done():
			log.Println("WebSocket: shutting down.")
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		if loginErr := uc.Login(); loginErr != nil {
			log.Printf("WebSocket: re-login failed: %v", loginErr)
		} else {
			backoff = initialBackoff // reset backoff on successful re-auth
			consecutiveFailures = 0
		}
	}
}

// connectAndListen opens a single WebSocket connection and reads events until
// the connection drops, an error occurs, or the context is cancelled.
func (uc *UnifiClient) connectAndListen(ctx context.Context, onEvent func(msgType string), onTeleportSync func(NetworkClient)) error {
	wsURL, err := uc.resolveWSURL()
	if err != nil {
		return err
	}

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		HandshakeTimeout: 15 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, uc.wsHeaders())
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer conn.Close()

	// Close the connection when context is cancelled so ReadMessage unblocks.
	// The done channel ensures this goroutine exits when connectAndListen returns
	// normally (e.g. connection drop), preventing a goroutine leak per reconnect.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			conn.Close()
		case <-done:
		}
	}()

	log.Printf("WebSocket connected: %s", wsURL)

	// Open debug log file once for the lifetime of this connection.
	var debugFile *os.File
	if path := os.Getenv("WS_DEBUG_LOG"); path != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			log.Printf("WebSocket: could not create debug log directory: %v", err)
		} else if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); err != nil {
			log.Printf("WebSocket: could not open debug log %s: %v", path, err)
		} else {
			debugFile = f
			defer debugFile.Close()
			log.Printf("WebSocket: logging all events to %s", path)
		}
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		if debugFile != nil {
			if _, writeErr := fmt.Fprintf(debugFile, "%s\n", msg); writeErr != nil {
				log.Printf("WebSocket: debug log write failed: %v", writeErr)
			}
		}

		var event wsEvent
		if err := json.Unmarshal(msg, &event); err != nil {
			log.Printf("WebSocket: failed to parse message: %v", err)
			continue
		}

		// Handle Teleport client:sync directly — all needed data is in the payload.
		if event.Meta.Message == "client:sync" {
			for _, d := range event.Data {
				if d.Type == "TELEPORT" && d.Status == "online" {
					onTeleportSync(NetworkClient{
						Name: StrOrDefault(d.DisplayName, d.Name),
						IP:   d.IP,
						ID:   d.ID,
						Type: d.Type,
					})
				}
			}
			continue
		}

		if event.isConnectionEvent() {
			onEvent(event.Meta.Message)
		}
	}
}

// clearWSURL resets the cached WebSocket URL so the next connection re-probes.
func (uc *UnifiClient) clearWSURL() {
	uc.wsMu.Lock()
	uc.wsURL = ""
	uc.wsMu.Unlock()
}

// resolveWSURL determines the correct WebSocket URL for this controller.
// It probes the modern UniFi OS path first (/proxy/network/wss/...) and falls
// back to the legacy path (/wss/...). The result is cached after first probe.
//
// Holds wsMu for the full duration. wsHeaders() acquires mu.RLock() independently —
// no circular dependency, no deadlock risk.
func (uc *UnifiClient) resolveWSURL() (string, error) {
	uc.wsMu.Lock()
	defer uc.wsMu.Unlock()

	if uc.wsURL != "" {
		return uc.wsURL, nil
	}

	base := strings.TrimRight(uc.cfg.URL, "/")
	base = strings.Replace(base, "https://", "wss://", 1)
	base = strings.Replace(base, "http://", "ws://", 1)

	const wsParams = "?clients=v2&next_ai_notifications=true"
	modern := base + "/proxy/network/wss/s/" + uc.siteID + "/events" + wsParams
	legacy := base + "/wss/s/" + uc.siteID + "/events" + wsParams

	if probeWS(modern, uc.wsHeaders()) {
		log.Println("WebSocket: detected UniFi OS controller (modern path).")
		uc.wsURL = modern
		return modern, nil
	}

	log.Println("WebSocket: using legacy controller path.")
	uc.wsURL = legacy
	return legacy, nil
}

// probeWS does a quick dial to check whether a WebSocket URL is reachable.
func probeWS(wsURL string, header http.Header) bool {
	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		HandshakeTimeout: 5 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// wsHeaders builds HTTP headers for the WebSocket handshake, including the
// session cookies from the authenticated unpoller HTTP client.
// Acquires mu.RLock to safely read uc.u.
func (uc *UnifiClient) wsHeaders() http.Header {
	header := http.Header{}
	header.Set("Origin", uc.cfg.URL)

	parsedURL, err := url.Parse(uc.cfg.URL)
	if err != nil {
		return header
	}

	uc.mu.RLock()
	u := uc.u
	uc.mu.RUnlock()

	if u != nil && u.Jar != nil {
		for _, c := range u.Jar.Cookies(parsedURL) {
			header.Add("Cookie", c.Name+"="+c.Value)
		}
	}
	return header
}
