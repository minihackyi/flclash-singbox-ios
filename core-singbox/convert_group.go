package main

import (
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// convertGroup builds a selector or urltest outbound for a Clash proxy-group.
// Returns (outbound, isSelector).
func convertGroup(name string, groupType string, members []string, selectedMap map[string]string, testURL string, clash *ClashConfig) (map[string]any, bool) {
	outbounds := members
	if outbounds == nil {
		outbounds = []string{}
	}
	switch strings.ToLower(groupType) {
	case "select":
		outbound := map[string]any{
			"type":      "selector",
			"tag":       name,
			"outbounds": outbounds,
		}
		if selected, ok := selectedMap[name]; ok && selected != "" && containsString(outbounds, selected) {
			outbound["default"] = selected
		}
		return outbound, true
	case "url-test", "fallback", "load-balance", "relay":
		// sing-box has no fallback/load-balance/relay group; urltest is the
		// closest automatic substitute, selector keeps manual choice.
		outbound := map[string]any{
			"type":      "urltest",
			"tag":       name,
			"outbounds": outbounds,
		}
		if testURL != "" {
			outbound["url"] = testURL
		} else if url := asString(findGroupField(clash, name, "url")); url != "" {
			outbound["url"] = url
		}
		if interval := asInt(findGroupField(clash, name, "interval"), 300); interval > 0 {
			outbound["interval"] = formatSeconds(interval)
		}
		if tolerance := asInt(findGroupField(clash, name, "tolerance"), 0); tolerance > 0 {
			outbound["tolerance"] = tolerance
		}
		return outbound, false
	default:
		outbound := map[string]any{
			"type":      "selector",
			"tag":       name,
			"outbounds": outbounds,
		}
		if selected, ok := selectedMap[name]; ok && selected != "" && containsString(outbounds, selected) {
			outbound["default"] = selected
		}
		return outbound, true
	}
}

func findGroupField(clash *ClashConfig, name string, field string) any {
	for _, group := range clash.ProxyGroups {
		if asString(group["name"]) == name {
			return group[field]
		}
	}
	return nil
}

func formatSeconds(seconds int) string {
	return itoa(seconds) + "s"
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// expandGroupMembers resolves a group's member list: `proxies`, `use`
// (provider names), `include-all` and filters.
func expandGroupMembers(clash *ClashConfig, group map[string]any, proxyTags []string, groupTags map[string]bool, homeDir string, unsupported map[string]bool) []string {
	members := []string{}
	seen := map[string]bool{}
	addMember := func(name string) {
		if name == "" || seen[name] {
			return
		}
		if name == "DIRECT" || name == "REJECT" || proxyHasTag(clash, proxyTags, name) || groupTags[name] {
			seen[name] = true
			members = append(members, name)
		}
	}

	includeAll := asBool(group["include-all"]) || asBool(group["include-all-proxies"])
	filter := asString(group["filter"])
	excludeFilter := asString(group["exclude-filter"])
	excludeType := asString(group["exclude-type"])

	applyFilter := func(name string, proxyType string) bool {
		if filter != "" && !matchFilter(filter, name) {
			return false
		}
		if excludeFilter != "" && matchFilter(excludeFilter, name) {
			return false
		}
		if excludeType != "" {
			for _, banned := range strings.Split(excludeType, "|") {
				if strings.EqualFold(strings.TrimSpace(banned), proxyType) {
					return false
				}
			}
		}
		return true
	}

	if includeAll {
		for _, tag := range proxyTags {
			addMember(tag)
		}
		for tag := range groupTags {
			if tag != asString(group["name"]) {
				addMember(tag)
			}
		}
	}

	for _, member := range asStringList(group["proxies"]) {
		addMember(member)
	}

	// `use` references proxy providers; their proxies were already inlined as
	// plain outbounds, so re-apply each provider's node list here.
	for _, providerName := range asStringList(group["use"]) {
		for _, proxy := range providerProxiesByName(clash, homeDir, providerName, unsupported) {
			name := asString(proxy["name"])
			proxyType := strings.ToLower(asString(proxy["type"]))
			if applyFilter(name, proxyType) {
				addMember(name)
			}
		}
	}

	return members
}

func proxyHasTag(clash *ClashConfig, proxyTags []string, name string) bool {
	return containsString(proxyTags, name)
}

// expandProxyProviders inlines file-backed proxy-providers as outbounds.
func expandProxyProviders(clash *ClashConfig, homeDir string, unsupported map[string]bool) []map[string]any {
	out := []map[string]any{}
	for name := range clash.ProxyProviders {
		out = append(out, providerProxiesByName(clash, homeDir, name, unsupported)...)
	}
	return out
}

func providerProxiesByName(clash *ClashConfig, homeDir string, providerName string, unsupported map[string]bool) []map[string]any {
	provider, ok := clash.ProxyProviders[providerName]
	if !ok {
		return nil
	}
	proxies, err := readProviderProxies(clash, homeDir, provider)
	if err != nil {
		logError("provider %s read error: %v", providerName, err)
		unsupported["provider:"+providerName] = true
		return nil
	}
	out := []map[string]any{}
	for _, proxy := range proxies {
		outbound, ok := convertProxy(proxy, unsupported)
		if !ok {
			continue
		}
		out = append(out, outbound)
	}
	return out
}

func readProviderProxies(clash *ClashConfig, homeDir string, provider map[string]any) ([]map[string]any, error) {
	providerType := strings.ToLower(asString(provider["type"]))
	path := providerPath(homeDir, provider["path"])
	if path == "" {
		return nil, os.ErrNotExist
	}
	_ = providerType
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	return parsed.Proxies, nil
}
