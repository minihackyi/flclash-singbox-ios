package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// normalizeDnsServer maps a clash nameserver entry to a canonical
// "scheme://rest" form (bare IP kept as-is; unsupported entries dropped).
func normalizeDnsServer(name string) string {
	name = strings.TrimSpace(name)
	lower := strings.ToLower(name)
	switch {
	case strings.HasPrefix(lower, "https://"), strings.HasPrefix(lower, "tls://"),
		strings.HasPrefix(lower, "quic://"), strings.HasPrefix(lower, "udp://"):
		return name
	case strings.HasPrefix(lower, "dhcp://"), strings.HasPrefix(lower, "system://"),
		strings.HasPrefix(lower, "rcode://"):
		return ""
	default:
		if name == "" {
			return ""
		}
		return name
	}
}

func dedupeStrings(list []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range list {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

var errUnsupported = errors.New("unsupported")

// buildDns converts the Clash dns section into a sing-box (v1.12+ new format)
// dns config. Fake-ip is not emulated; tun dns-hijack is handled by the
// dns-out outbound and a protocol=dns route rule.
func buildDns(clash *ClashConfig, proxyTags map[string]bool, groupTags map[string]bool) map[string]any {
	dnsClash := clash.Dns
	dns := map[string]any{}
	servers := []map[string]any{}

	dnsEnabled := true
	if dnsClash != nil {
		if enable, ok := dnsClash["enable"].(bool); ok {
			dnsEnabled = enable
		}
	}

	bootstrapNames := []string{}
	if dnsClash != nil {
		for _, name := range asStringList(dnsClash["default-nameserver"]) {
			if address := normalizeDnsServer(name); address != "" && isPlainUdpDns(address) {
				bootstrapNames = append(bootstrapNames, address)
			}
		}
	}
	if len(bootstrapNames) == 0 {
		bootstrapNames = []string{"223.5.5.5"}
	}
	for i, address := range bootstrapNames {
		if i >= 2 {
			break
		}
		servers = append(servers, map[string]any{
			"type":   "udp",
			"tag":    "dns-bootstrap-" + strconv.Itoa(i+1),
			"server": address,
		})
	}

	remoteNames := []string{}
	if dnsClash != nil {
		for _, name := range asStringList(dnsClash["nameserver"]) {
			if address := normalizeDnsServer(name); address != "" {
				remoteNames = append(remoteNames, address)
			}
		}
		for _, name := range asStringList(dnsClash["fallback"]) {
			if address := normalizeDnsServer(name); address != "" {
				remoteNames = append(remoteNames, address)
			}
		}
	}
	remoteNames = dedupeStrings(remoteNames)
	if !dnsEnabled || len(remoteNames) == 0 {
		remoteNames = []string{"223.5.5.5", "119.29.29.29"}
	}

	for i, address := range remoteNames {
		server, err := buildRemoteDnsServer("dns-remote-"+strconv.Itoa(i+1), address)
		if err != nil {
			logError("skip dns server %s: %v", address, err)
			continue
		}
		if domain := dnsServerDomain(address); domain != "" {
			server["domain_resolver"] = "dns-bootstrap-1"
		}
		servers = append(servers, server)
	}

	servers = append(servers, map[string]any{
		"type": "local",
		"tag":  "dns-local",
	})

	dns["servers"] = servers
	dns["final"] = "dns-remote-1"
	if !clash.IPv6 {
		dns["strategy"] = "ipv4_only"
	}
	return dns
}

// buildRemoteDnsServer converts one clash nameserver entry into a new-format
// sing-box dns server. Input keeps the clash scheme prefix (udp://, tls://,
// https://, quic://) or a bare IP.
func buildRemoteDnsServer(tag string, address string) (map[string]any, error) {
	lower := strings.ToLower(address)
	server := map[string]any{"tag": tag}
	switch {
	case strings.HasPrefix(lower, "https://"):
		rest := strings.TrimPrefix(address, address[:8])
		host, path := splitHostPath(strings.TrimPrefix(address[8:], ""), "/dns-query")
		server["type"] = "https"
		server["server"] = host
		if path != "" && path != "/dns-query" {
			server["path"] = path
		}
		if strings.HasSuffix(lower, " h3") {
			server["type"] = "h3"
		}
		_ = rest
		return server, nil
	case strings.HasPrefix(lower, "tls://"):
		host := address[6:]
		server["type"] = "tls"
		server["server"] = trimPort(host)
		return server, nil
	case strings.HasPrefix(lower, "quic://"):
		host := address[7:]
		server["type"] = "quic"
		server["server"] = trimPort(host)
		return server, nil
	case strings.HasPrefix(lower, "udp://"):
		host := address[6:]
		server["type"] = "udp"
		server["server"] = trimPort(host)
		return server, nil
	default:
		if net.ParseIP(address) != nil {
			server["type"] = "udp"
			server["server"] = address
			return server, nil
		}
		return nil, fmt.Errorf("unsupported dns address %q: %w", address, errUnsupported)
	}
}

func splitHostPath(input string, defaultPath string) (string, string) {
	idx := strings.Index(input, "/")
	if idx < 0 {
		return input, defaultPath
	}
	return input[:idx], input[idx:]
}

func trimPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx > 0 && !strings.Contains(host, "]") {
		return host[:idx]
	}
	return host
}

func isPlainUdpDns(address string) bool {
	return net.ParseIP(address) != nil
}

func dnsServerDomain(address string) string {
	host := address
	switch {
	case strings.HasPrefix(strings.ToLower(address), "https://"):
		host = address[8:]
	case strings.HasPrefix(strings.ToLower(address), "tls://"):
		host = address[6:]
	case strings.HasPrefix(strings.ToLower(address), "quic://"):
		host = address[7:]
	default:
		return ""
	}
	host, _ = splitHostPath(host, "")
	host = trimPort(host)
	if net.ParseIP(host) != nil {
		return ""
	}
	return host
}
