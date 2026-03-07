package unifi

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/unpoller/unifi"
)

// UnifiClient wraps the unpoller/unifi client.
type UnifiClient struct {
	u      *unifi.Unifi
	sites  []*unifi.Site
	siteID string
	cfg    *unifi.Config
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
func (uc *UnifiClient) Login() error {
	u, err := unifi.NewUnifi(uc.cfg)
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	uc.u = u

	sites, err := u.GetSites()
	if err != nil {
		return fmt.Errorf("failed to get sites: %w", err)
	}

	// Use only the configured site; fall back to all sites if not found.
	uc.sites = filterSite(sites, uc.siteID)
	return nil
}

// Logout is a no-op — unpoller/unifi manages the session internally.
func (uc *UnifiClient) Logout() {}

// ListClients returns all active clients via the unpoller library.
func (uc *UnifiClient) ListClients() ([]NetworkClient, error) {
	clients, err := uc.u.GetClients(uc.sites)
	if err != nil {
		return nil, err
	}

	result := make([]NetworkClient, len(clients))
	for i, c := range clients {
		result[i] = NetworkClient{
			Mac:      c.Mac,
			IP:       c.IP,
			Hostname: c.Hostname,
			Name:     c.Name,
			IsWired:  c.IsWired.Val,
			Network:  c.Network,
		}
	}
	return result, nil
}

// ListTeleportClients fetches active clients from the UniFi v2 API.
// The library doesn't cover this endpoint, so we use its underlying HTTP client directly.
func (uc *UnifiClient) ListTeleportClients() ([]NetworkClient, error) {
	url := uc.cfg.URL + "/v2/api/site/" + uc.siteID + "/clients/active"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := uc.u.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("teleport client list returned HTTP %d", resp.StatusCode)
	}

	var clients []NetworkClient
	if err := json.NewDecoder(resp.Body).Decode(&clients); err != nil {
		return nil, fmt.Errorf("failed to decode teleport clients: %w", err)
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
