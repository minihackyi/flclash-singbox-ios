package main

import (
	"strconv"
	"strings"
)

// convertProxy maps one Clash proxy entry to a sing-box outbound. Returns
// ok=false when the proxy is unusable (missing fields or unsupported type).
func convertProxy(proxy map[string]any, unsupported map[string]bool) (map[string]any, bool) {
	name := asString(proxy["name"])
	server := asString(proxy["server"])
	port := asInt(proxy["port"], 0)
	proxyType := strings.ToLower(asString(proxy["type"]))
	if name == "" || server == "" || port == 0 {
		unsupported[name+"/missing-fields"] = true
		return nil, false
	}

	outbound := map[string]any{
		"tag":         name,
		"server":      server,
		"server_port": port,
	}

	detourOk := true
	switch proxyType {
	case "ss", "shadowsocks":
		outbound["type"] = "shadowsocks"
		outbound["method"] = asString(proxy["cipher"])
		outbound["password"] = asString(proxy["password"])
		if plugin := asString(proxy["plugin"]); plugin != "" {
			switch plugin {
			case "obfs":
				outbound["plugin"] = "obfs-local"
			case "v2ray-plugin":
				outbound["plugin"] = "v2ray-plugin"
			default:
				outbound["plugin"] = plugin
			}
			if opts := asString(proxy["plugin-opts"]); opts != "" {
				outbound["plugin_opts"] = opts
			} else if pluginOpts, ok := proxy["plugin-opts"].(map[string]any); ok {
				outbound["plugin_opts"] = buildObfsPluginOpts(pluginOpts, plugin)
			}
		}
	case "vmess":
		outbound["type"] = "vmess"
		outbound["uuid"] = asString(proxy["uuid"])
		outbound["security"] = orDefault(asString(proxy["cipher"]), "auto")
		if alterId, ok := proxy["alterId"]; ok {
			outbound["alter_id"] = asInt(alterId, 0)
		} else if alterId, ok := proxy["alter-id"]; ok {
			outbound["alter_id"] = asInt(alterId, 0)
		}
		applyTransport(outbound, proxy)
		applyTls(outbound, proxy)
	case "vless":
		outbound["type"] = "vless"
		outbound["uuid"] = asString(proxy["uuid"])
		if flow := asString(proxy["flow"]); flow != "" {
			outbound["flow"] = flow
		}
		applyTransport(outbound, proxy)
		applyTls(outbound, proxy)
	case "trojan":
		outbound["type"] = "trojan"
		outbound["password"] = asString(proxy["password"])
		applyTransport(outbound, proxy)
		applyTls(outbound, proxy)
	case "hysteria2":
		outbound["type"] = "hysteria2"
		if password := asString(proxy["password"]); password != "" {
			outbound["password"] = password
		}
		if auth := asString(proxy["auth"]); auth != "" && asString(proxy["password"]) == "" {
			outbound["password"] = auth
		}
		if up := asString(proxy["up"]); up != "" {
			if mbps := parseMbps(up); mbps > 0 {
				outbound["up_mbps"] = mbps
			}
		}
		if down := asString(proxy["down"]); down != "" {
			if mbps := parseMbps(down); mbps > 0 {
				outbound["down_mbps"] = mbps
			}
		}
		if obfs, ok := proxy["obfs"].(map[string]any); ok {
			if password := asString(obfs["password"]); password != "" {
				outbound["obfs"] = map[string]any{
					"type":     "salamander",
					"password": password,
				}
			}
		} else if obfs := asString(proxy["obfs"]); obfs != "" && obfs != "none" {
			outbound["obfs"] = map[string]any{
				"type":     "salamander",
				"password": asString(proxy["obfs-password"]),
			}
		}
		applyTls(outbound, proxy)
	case "hysteria":
		outbound["type"] = "hysteria"
		if auth := asString(proxy["auth"]); auth != "" {
			outbound["auth_str"] = auth
		}
		if up := asString(proxy["up"]); up != "" {
			if mbps := parseMbps(up); mbps > 0 {
				outbound["up_mbps"] = mbps
			}
		}
		if down := asString(proxy["down"]); down != "" {
			if mbps := parseMbps(down); mbps > 0 {
				outbound["down_mbps"] = mbps
			}
		}
		if protocol := asString(proxy["protocol"]); protocol != "" {
			outbound["network"] = protocol
		}
		applyTls(outbound, proxy)
	case "tuic":
		outbound["type"] = "tuic"
		if uuid := asString(proxy["uuid"]); uuid != "" {
			outbound["uuid"] = uuid
		}
		if password := asString(proxy["password"]); password != "" {
			outbound["password"] = password
		}
		if cc := asString(proxy["congestion-controller"]); cc != "" {
			outbound["congestion_control"] = cc
		}
		if udpRelayMode := asString(proxy["udp-relay-mode"]); udpRelayMode != "" {
			outbound["udp_relay_mode"] = udpRelayMode
		}
		if asBool(proxy["disable-sni"]) {
			outbound["disable_sni"] = true
		}
		applyTls(outbound, proxy)
	case "socks5", "socks":
		outbound["type"] = "socks"
		if username := asString(proxy["username"]); username != "" {
			outbound["username"] = username
		}
		if password := asString(proxy["password"]); password != "" {
			outbound["password"] = password
		}
		if asBool(proxy["tls"]) {
			applyTls(outbound, map[string]any{"tls": true})
		}
	case "http":
		outbound["type"] = "http"
		if username := asString(proxy["username"]); username != "" {
			outbound["username"] = username
		}
		if password := asString(proxy["password"]); password != "" {
			outbound["password"] = password
		}
		if asBool(proxy["tls"]) {
			applyTls(outbound, map[string]any{"tls": true, "sni": proxy["sni"], "skip-cert-verify": proxy["skip-cert-verify"]})
		}
	case "anytls":
		outbound["type"] = "anytls"
		outbound["password"] = asString(proxy["password"])
		applyTls(outbound, proxy)
	case "wireguard":
		outbound["type"] = "wireguard"
		outbound["private_key"] = asString(proxy["private-key"])
		outbound["peer_public_key"] = asString(proxy["public-key"])
		localAddresses := []string{}
		if ip := asString(proxy["ip"]); ip != "" {
			localAddresses = append(localAddresses, cidrOrAddress(ip, false))
		}
		if ip := asString(proxy["ipv6"]); ip != "" {
			localAddresses = append(localAddresses, cidrOrAddress(ip, true))
		}
		outbound["local_address"] = localAddresses
		if reserved := asStringList(proxy["reserved"]); len(reserved) > 0 {
			reservedInts := make([]int, 0, len(reserved))
			for _, item := range reserved {
				reservedInts = append(reservedInts, asInt(item, 0))
			}
			outbound["reserved"] = reservedInts
		}
		if mtu := asInt(proxy["mtu"], 0); mtu > 0 {
			outbound["mtu"] = mtu
		}
		// sing-box uses server/server_port as the peer endpoint, which the
		// base outbound already carries.
	default:
		unsupported[name+"/"+proxyType] = true
		return nil, false
	}

	_ = detourOk
	applyDialFields(outbound, proxy)
	return outbound, true
}

func buildObfsPluginOpts(opts map[string]any, plugin string) string {
	if plugin == "obfs" {
		mode := orDefault(asString(opts["mode"]), "http")
		host := asString(opts["host"])
		value := "obfs=" + mode
		if host != "" {
			value += ";obfs-host=" + host
		}
		return value
	}
	if plugin == "v2ray-plugin" {
		parts := []string{"mode=" + orDefault(asString(opts["mode"]), "websocket")}
		if asBool(opts["tls"]) {
			parts = append(parts, "tls")
		}
		if host := asString(opts["host"]); host != "" {
			parts = append(parts, "host="+host)
		}
		if path := asString(opts["path"]); path != "" {
			parts = append(parts, "path="+path)
		}
		return strings.Join(parts, ";")
	}
	return ""
}

func applyTls(outbound map[string]any, proxy map[string]any) {
	tlsEnabled := asBool(proxy["tls"])
	reality, hasReality := proxy["reality-opts"].(map[string]any)
	if !tlsEnabled && !hasReality {
		// hysteria(2)/tuic/anytls always use tls.
		switch asString(outbound["type"]) {
		case "hysteria", "hysteria2", "tuic", "anytls":
			tlsEnabled = true
		}
	}
	if !tlsEnabled {
		return
	}
	tls := map[string]any{"enabled": true}
	if sni := firstString(proxy["servername"], proxy["server-name"], proxy["sni"]); sni != "" {
		tls["server_name"] = sni
	}
	if asBool(proxy["skip-cert-verify"]) {
		tls["insecure"] = true
	}
	if alpn := asStringList(proxy["alpn"]); len(alpn) > 0 {
		tls["alpn"] = alpn
	}
	if fingerprint := asString(proxy["client-fingerprint"]); fingerprint != "" && fingerprint != "none" {
		tls["utls"] = map[string]any{
			"enabled":     true,
			"fingerprint": fingerprint,
		}
	}
	if hasReality {
		tls["reality"] = map[string]any{
			"enabled":    true,
			"public_key": asString(reality["public-key"]),
			"short_id":   asString(reality["short-id"]),
		}
	}
	if ech, ok := proxy["ech-opts"].(map[string]any); ok {
		tls["ech"] = map[string]any{
			"enabled":                      true,
			"pq_signature_schemes_enabled": asBool(ech["pq-signature-schemes-enabled"]),
			"config":                       asStringList(ech["config"]),
		}
	}
	outbound["tls"] = tls
}

func applyTransport(outbound map[string]any, proxy map[string]any) {
	network := strings.ToLower(asString(proxy["network"]))
	if network == "" || network == "tcp" {
		return
	}
	switch network {
	case "ws":
		transport := map[string]any{"type": "ws"}
		if opts, ok := proxy["ws-opts"].(map[string]any); ok {
			if path := asString(opts["path"]); path != "" {
				transport["path"] = path
			}
			if headers, ok := opts["headers"].(map[string]any); ok {
				normalized := map[string]any{}
				for key, value := range headers {
					normalized[key] = value
				}
				transport["headers"] = normalized
			}
			if maxEarly := asInt(opts["max-early-data"], 0); maxEarly > 0 {
				transport["max_early_data"] = maxEarly
			}
			if header := asString(opts["early-data-header-name"]); header != "" {
				transport["early_data_header_name"] = header
			}
		}
		outbound["transport"] = transport
	case "grpc":
		transport := map[string]any{"type": "grpc"}
		if opts, ok := proxy["grpc-opts"].(map[string]any); ok {
			if service := asString(opts["grpc-service-name"]); service != "" {
				transport["service_name"] = service
			}
		}
		outbound["transport"] = transport
	case "h2", "http":
		if network == "h2" {
			transport := map[string]any{"type": "http"}
			if opts, ok := proxy["h2-opts"].(map[string]any); ok {
				if host := asStringList(opts["host"]); len(host) > 0 {
					transport["host"] = host
				}
				if path := asString(opts["path"]); path != "" {
					transport["path"] = path
				}
			}
			outbound["transport"] = transport
			outbound["tls"] = ensureTlsEnabled(outbound["tls"])
			return
		}
		transport := map[string]any{"type": "http"}
		if opts, ok := proxy["http-opts"].(map[string]any); ok {
			if method := asString(opts["method"]); method != "" {
				transport["method"] = method
			}
			if paths, ok := opts["path"].([]any); ok && len(paths) > 0 {
				transport["path"] = []string{asString(paths[0])}
			}
			if headers, ok := opts["headers"].(map[string]any); ok {
				normalized := map[string]any{}
				for key, value := range headers {
					normalized[key] = value
				}
				transport["headers"] = normalized
			}
		}
		outbound["transport"] = transport
	}
}

func ensureTlsEnabled(existing any) any {
	if tls, ok := existing.(map[string]any); ok {
		tls["enabled"] = true
		return tls
	}
	return map[string]any{"enabled": true}
}

func applyDialFields(outbound map[string]any, proxy map[string]any) {
	if bindInterface := asString(proxy["interface-name"]); bindInterface != "" {
		outbound["bind_interface"] = bindInterface
	}
	if routingMark := asInt(proxy["routing-mark"], 0); routingMark > 0 {
		outbound["routing_mark"] = routingMark
	}
	if ipVersion := asString(proxy["ip-version"]); ipVersion != "" && ipVersion != "dual" {
		switch ipVersion {
		case "ipv4", "ipv4-prefer", "ipv6", "ipv6-prefer":
			outbound["domain_strategy"] = ipVersion
		}
	}
	dialerProxy := asString(proxy["dialer-proxy"])
	if dialerProxy != "" {
		outbound["detour"] = dialerProxy
	}
}

func firstString(values ...any) string {
	for _, value := range values {
		if value == nil {
			continue
		}
		if str, ok := value.(string); ok && str != "" {
			return str
		}
	}
	return ""
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseMbps(value string) int {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.TrimSuffix(value, "mbps")
	value = strings.TrimSuffix(value, "m")
	number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}
	return int(number)
}

func cidrOrAddress(value string, ipv6 bool) string {
	if strings.Contains(value, "/") {
		return value
	}
	if ipv6 {
		return value + "/128"
	}
	return value + "/32"
}
