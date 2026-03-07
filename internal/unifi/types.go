package unifi

// NetworkClient represents a device connected to the UniFi network.
type NetworkClient struct {
	Mac         string `json:"mac"`
	IP          string `json:"ip"`
	Hostname    string `json:"hostname"`
	Name        string `json:"name"`
	IsWired     bool   `json:"is_wired"`
	Network     string `json:"network"`
	NetworkName string `json:"network_name"`
	Type        string `json:"type"`
	ID          string `json:"id"`
}

// Identifier returns the MAC address, falling back to ID for Teleport devices.
func (c *NetworkClient) Identifier(teleport bool) string {
	if teleport && c.Type == "TELEPORT" {
		return c.ID
	}
	if c.Mac != "" {
		return c.Mac
	}
	return c.ID
}

// StrOrDefault returns the string if non-empty, otherwise the default.
func StrOrDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// WiredStr returns "Wired" or "Wireless" based on connection type.
func WiredStr(isWired bool) string {
	if isWired {
		return "Wired"
	}
	return "Wireless"
}
