package advanced

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type NetworkInterface struct {
	Name      string   `json:"name"`
	State     string   `json:"state"`
	MTU       int      `json:"mtu"`
	MAC       string   `json:"mac,omitempty"`
	Addresses []string `json:"addresses"`
}

type NetworkRoute struct {
	Destination string `json:"destination"`
	Gateway     string `json:"gateway,omitempty"`
	Device      string `json:"device"`
	Protocol    string `json:"protocol,omitempty"`
	Scope       string `json:"scope,omitempty"`
	Metric      int    `json:"metric,omitempty"`
}

type Listener struct {
	Protocol string `json:"protocol"`
	Local    string `json:"local"`
	Peer     string `json:"peer"`
	Process  string `json:"process,omitempty"`
}

type NetworkOverview struct {
	Available   bool               `json:"available"`
	Interfaces  []NetworkInterface `json:"interfaces"`
	Routes      []NetworkRoute     `json:"routes"`
	Listeners   []Listener         `json:"listeners"`
	Truncated   bool               `json:"truncated"`
	CollectedAt time.Time          `json:"collected_at"`
}

const networkCommand = `printf '@@addr@@\n'
ip -j addr show 2>/dev/null || true
printf '@@route@@\n'
ip -j route show 2>/dev/null || true
printf '@@listen@@\n'
ss -H -tulpn 2>/dev/null || true`

func (s *Service) Network(ctx context.Context, ownerID, serverID string) (*NetworkOverview, error) {
	client, _, err := s.servers.Connect(ctx, ownerID, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = client.Close() }()
	output, stderr, err := s.run(ctx, client, networkCommand)
	if err != nil && output == "" {
		return nil, classify(stderr, err)
	}
	sections := splitSections(output)
	overview := &NetworkOverview{
		Available:   strings.TrimSpace(sections["addr"]) != "" || strings.TrimSpace(sections["route"]) != "",
		Interfaces:  parseInterfaces(sections["addr"]),
		Routes:      parseRoutes(sections["route"]),
		Listeners:   parseListeners(sections["listen"], 500),
		CollectedAt: time.Now().UTC(),
	}
	if !overview.Available {
		return nil, fmt.Errorf("%w: ip is unavailable", ErrUnavailable)
	}
	return overview, nil
}

func parseInterfaces(raw string) []NetworkInterface {
	var links []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &links); err != nil {
		return []NetworkInterface{}
	}
	result := make([]NetworkInterface, 0, len(links))
	for _, link := range links {
		item := NetworkInterface{
			Name: anyString(link["ifname"]), State: anyString(link["operstate"]),
			MAC: anyString(link["address"]),
		}
		if mtu, ok := link["mtu"].(float64); ok {
			item.MTU = int(mtu)
		}
		if addresses, ok := link["addr_info"].([]any); ok {
			for _, address := range addresses {
				value, _ := address.(map[string]any)
				if value == nil {
					continue
				}
				text := anyString(value["local"])
				if prefix, ok := value["prefixlen"].(float64); ok {
					text += fmt.Sprintf("/%d", int(prefix))
				}
				if text != "" {
					item.Addresses = append(item.Addresses, text)
				}
			}
		}
		result = append(result, item)
	}
	return result
}

func parseRoutes(raw string) []NetworkRoute {
	var routes []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &routes); err != nil {
		return []NetworkRoute{}
	}
	result := make([]NetworkRoute, 0, len(routes))
	for _, route := range routes {
		item := NetworkRoute{
			Destination: anyString(route["dst"]), Gateway: anyString(route["gateway"]),
			Device: anyString(route["dev"]), Protocol: anyString(route["protocol"]),
			Scope: anyString(route["scope"]),
		}
		if metric, ok := route["metric"].(float64); ok {
			item.Metric = int(metric)
		}
		if item.Destination == "" {
			item.Destination = "default"
		}
		result = append(result, item)
	}
	return result
}

func parseListeners(raw string, max int) []Listener {
	result := make([]Listener, 0, 32)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}
		listener := Listener{Protocol: fields[0], Local: fields[4], Peer: fields[5]}
		if len(fields) > 6 {
			listener.Process = strings.Join(fields[6:], " ")
		}
		result = append(result, listener)
		if len(result) >= max {
			break
		}
	}
	return result
}
