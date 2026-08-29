package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Conversion from a Clash profile into a sing-box (v1.11) JSON config.

type singboxRule map[string]any

type convertResult struct {
	config        map[string]any
	proxyTags     []string
	groupTags     []string
	allTags       []string
	unsupported   []string
	finalOutbound string
}

func asString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func asInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if number, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return number
		}
	}
	return fallback
}

func asBool(value any) bool {
	enabled, _ := value.(bool)
	return enabled
}

func asStringList(value any) []string {
	items, ok := value.([]any)
	if !ok {
		if value != nil {
			return []string{asString(value)}
		}
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, asString(item))
	}
	return out
}

// convertToSingBox turns a parsed Clash config into a sing-box option JSON map.
func convertToSingBox(clash *ClashConfig, homeDir string, selectedMap map[string]string, testURL string, mode string) (*convertResult, error) {
	result := &convertResult{}
	unsupported := map[string]bool{}

	inbounds := buildInbounds(clash)
	outbounds := []map[string]any{
		{"type": "direct", "tag": "DIRECT"},
		{"type": "block", "tag": "REJECT"},
	}

	proxyTagSet := map[string]bool{}
	groupTagSet := map[string]bool{}

	// Providers are inlined as plain outbounds first.
	providerProxies := expandProxyProviders(clash, homeDir, unsupported)

	proxyTags := make([]string, 0, len(clash.Proxies)+len(providerProxies))
	for _, proxy := range clash.Proxies {
		outbound, ok := convertProxy(proxy, unsupported)
		if !ok {
			continue
		}
		name := asString(proxy["name"])
		if name == "" || proxyTagSet[name] || groupTagSet[name] {
			continue
		}
		proxyTagSet[name] = true
		proxyTags = append(proxyTags, name)
		outbounds = append(outbounds, outbound)
	}
	for _, proxy := range providerProxies {
		name := asString(proxy["name"])
		if name == "" || proxyTagSet[name] {
			continue
		}
		proxyTagSet[name] = true
		proxyTags = append(proxyTags, name)
		outbounds = append(outbounds, proxy)
	}

	groupTags := make([]string, 0, len(clash.ProxyGroups)+1)
	groupMembers := map[string][]string{}
	groupTypes := map[string]string{}
	// First pass: register every group tag so groups can reference each other.
	for _, group := range clash.ProxyGroups {
		name := asString(group["name"])
		groupType := asString(group["type"])
		if name == "" || groupTagSet[name] || proxyTagSet[name] {
			continue
		}
		groupTagSet[name] = true
		groupTags = append(groupTags, name)
		groupTypes[name] = groupType
	}
	// Second pass: resolve members now that all group tags are known.
	for _, group := range clash.ProxyGroups {
		name := asString(group["name"])
		if name == "" || !groupTagSet[name] {
			continue
		}
		groupMembers[name] = expandGroupMembers(clash, group, proxyTags, groupTagSet, homeDir, unsupported)
	}

	groupOutbounds := map[string]map[string]any{}
	for _, name := range groupTags {
		outbound, isSelector := convertGroup(name, groupTypes[name], groupMembers[name], selectedMap, testURL, clash)
		groupOutbounds[name] = outbound
		if !isSelector {
			// urltest groups resolve their own members; keep the tag valid.
		}
		outbounds = append(outbounds, outbound)
	}

	// GLOBAL group mirrors mihomo behavior.
	globalMembers := append([]string{}, groupTags...)
	globalMembers = append(globalMembers, proxyTags...)
	globalMembers = append(globalMembers, "DIRECT")
	global := map[string]any{
		"type":      "selector",
		"tag":       "GLOBAL",
		"outbounds": globalMembers,
	}
	outbounds = append(outbounds, global)

	routeRules, finalOutbound := buildRouteRules(clash, homeDir, groupTagSet, proxyTagSet, unsupported)

	allTags := append([]string{"GLOBAL"}, groupTags...)

	dnsConfig := buildDns(clash, proxyTagSet, groupTagSet)

	logLevel := normalizeLogLevel(clash.LogLevel)

	config := map[string]any{
		"log": map[string]any{
			"level":     logLevel,
			"timestamp": true,
		},
		"dns":       dnsConfig,
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"route":     routeRules,
		"experimental": map[string]any{
			"cache_file": map[string]any{"enabled": false},
		},
	}

	result.config = config
	result.proxyTags = proxyTags
	result.groupTags = groupTags
	result.allTags = allTags
	result.finalOutbound = finalOutbound
	for name := range unsupported {
		result.unsupported = append(result.unsupported, name)
	}
	return result, nil
}

func normalizeLogLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return "debug"
	case "warning", "warn":
		return "warn"
	case "error":
		return "error"
	case "silent":
		return "panic"
	default:
		return "info"
	}
}

func buildInbounds(clash *ClashConfig) []map[string]any {
	listen := "127.0.0.1"
	if clash.AllowLan {
		listen = "::"
	}

	inbounds := []map[string]any{}
	if clash.MixedPort > 0 {
		inbounds = append(inbounds, map[string]any{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      listen,
			"listen_port": clash.MixedPort,
		})
	}
	if clash.Port > 0 {
		inbounds = append(inbounds, map[string]any{
			"type":        "http",
			"tag":         "http-in",
			"listen":      listen,
			"listen_port": clash.Port,
		})
	}
	if clash.SocksPort > 0 {
		inbounds = append(inbounds, map[string]any{
			"type":        "socks",
			"tag":         "socks-in",
			"listen":      listen,
			"listen_port": clash.SocksPort,
		})
	}
	if clash.RedirPort > 0 {
		inbounds = append(inbounds, map[string]any{
			"type":        "redirect",
			"tag":         "redirect-in",
			"listen":      listen,
			"listen_port": clash.RedirPort,
		})
	}
	if clash.TProxyPort > 0 {
		inbounds = append(inbounds, map[string]any{
			"type":        "tproxy",
			"tag":         "tproxy-in",
			"listen":      listen,
			"listen_port": clash.TProxyPort,
		})
	}
	if clash.Tun != nil && asBool(clash.Tun["enable"]) {
		tun := map[string]any{
			"type":    "tun",
			"tag":     "tun-in",
			"address": tunAddresses(clash),
			"mtu":     9000,
		}
		stack := strings.ToLower(asString(clash.Tun["stack"]))
		switch stack {
		case "system":
			tun["stack"] = "system"
		case "mixed":
			tun["stack"] = "mixed"
		default:
			tun["stack"] = defaultTunStack()
		}
		if device := asString(clash.Tun["device"]); device != "" {
			tun["interface_name"] = device
		}
		if asBool(clash.Tun["auto-route"]) {
			tun["auto_route"] = true
		}
		if routeAddress := asStringList(clash.Tun["route-address"]); len(routeAddress) > 0 {
			tun["route_address"] = routeAddress
		}
		inbounds = append(inbounds, tun)
	}
	return inbounds
}

func tunAddresses(clash *ClashConfig) []string {
	address := []string{"172.19.0.1/30"}
	if clash.IPv6 {
		address = append(address, "fdfe:dcba:9876::1/126")
	}
	return address
}
