package unifi

// NetworkClient represents a device connected to the UniFi network.
type NetworkClient struct {
	Mac              string `json:"mac"`
	IP               string `json:"ip"`
	Hostname         string `json:"hostname"`
	Name             string `json:"name"`
	IsWired          bool   `json:"is_wired"`
	Network          string `json:"network_id"`
	NetworkName      string `json:"network_name"`
	Type             string `json:"type"`
	ID               string `json:"id"`
	DeviceName       string `json:"device_name"`
	UserSuppliedName string `json:"user_supplied_name"`
	FixedName        string `json:"fixed_name"`
	ModelName        string `json:"model_name"`
	ESSID            string `json:"essid"`
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

// DisplayName returns the best human-readable name for the client.
// Tries Name, ModelName, DeviceName, UserSuppliedName, FixedName, then Hostname.
// Rejects any value that looks like a MAC address. Falls back to "Unknown".
func (c *NetworkClient) DisplayName() string {
	candidates := []string{c.Name, c.ModelName, c.DeviceName, c.UserSuppliedName, c.FixedName, c.Hostname}
	for _, name := range candidates {
		if name != "" && !LooksLikeMAC(name) {
			return name
		}
	}
	return "Unknown"
}

// LooksLikeMAC returns true if s matches the xx:xx:xx:xx:xx:xx MAC pattern.
func LooksLikeMAC(s string) bool {
	if len(s) != 17 {
		return false
	}
	for i, c := range s {
		switch i % 3 {
		case 0, 1:
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		case 2:
			if c != ':' {
				return false
			}
		}
	}
	return true
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
