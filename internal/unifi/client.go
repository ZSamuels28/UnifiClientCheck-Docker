package unifi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/unpoller/unifi"
)

// UnifiClient wraps the unpoller/unifi client.
//
// Concurrency: two mutexes protect mutable state.
//   - mu guards u and sites (written by Login, read by ListClients/ListTeleportClients/wsHeaders)
//   - wsMu guards wsURL (read/written by resolveWSURL in websocket.go)
//
// The two mutexes are always acquired independently — never while holding the other —
// so there is no deadlock risk.
type UnifiClient struct {
	mu    sync.RWMutex
	wsMu  sync.Mutex
	u     *unifi.Unifi
	sites []*unifi.Site

	siteID string
	cfg    *unifi.Config
	wsURL  string // cached WebSocket URL, resolved on first connection
}

func NewUnifiClient(user, password, baseURL, siteID string) *UnifiClient {
	if siteID == "" {
		siteID = "default"
	}
	return &UnifiClient{
		siteID: siteID,
		cfg: &unifi.Config{
			User:      user,
			Pass:      password,
			URL:       baseURL,
			VerifySSL: false,
		},
	}
}

// Login authenticates and fetches the site list.
// The unpoller library handles both legacy and UniFi OS paths automatically.
// Safe to call concurrently — acquiring the write lock ensures readers wait.
func (uc *UnifiClient) Login() error {
	// Perform the (potentially slow) network operations before holding the lock.
	u, err := unifi.NewUnifi(uc.cfg)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	sites, err := u.GetSites()
	if err != nil {
		return fmt.Errorf("failed to get sites: %w", err)
	}

	filtered := filterSite(sites, uc.siteID)

	uc.mu.Lock()
	uc.u = u
	uc.sites = filtered
	uc.mu.Unlock()

	return nil
}

// Logout is a no-op — unpoller/unifi manages the session internally.
func (uc *UnifiClient) Logout() {}

// ListClients fetches all active clients from the UniFi v2 API, including VPN clients and infrastructure devices.
// Uses unpoller's authenticated client which handles the /proxy/network prefix for UniFi OS automatically.
func (uc *UnifiClient) ListClients() ([]NetworkClient, error) {
	path := fmt.Sprintf("/v2/api/site/%s/clients/active?includeTrafficUsage=true&includeUnifiDevices=true", uc.siteID)

	uc.mu.RLock()
	u := uc.u
	uc.mu.RUnlock()

	if u == nil {
		return nil, fmt.Errorf("not authenticated; call Login() first")
	}

	body, err := u.GetJSON(path)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch clients: %w", err)
	}

	var clients []NetworkClient
	if err := json.Unmarshal(body, &clients); err != nil {
		return nil, fmt.Errorf("failed to decode clients: %w", err)
	}
	return clients, nil
}

func filterSite(sites []*unifi.Site, siteID string) []*unifi.Site {
	for _, s := range sites {
		if s.Name == siteID {
			return []*unifi.Site{s}
		}
	}
	return sites
}
