package main

import (
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// buildRouteRules converts Clash rules to sing-box route rules and picks the
// final outbound based on the current mode.
func buildRouteRules(clash *ClashConfig, homeDir string, groupTags map[string]bool, proxyTags map[string]bool, unsupported map[string]bool) (map[string]any, string) {
	resolveTarget := func(target string) string {
		switch target {
		case "DIRECT":
			return "DIRECT"
		case "REJECT", "REJECT-DROP":
			return "REJECT"
		case "PASS":
			return ""
		default:
			if groupTags[target] || proxyTags[target] {
				return target
			}
			return ""
		}
	}

	appendRuleLine := func(rules []map[string]any, line string, policyOverride string) []map[string]any {
		ruleType, value, policy, flags := parseClashRule(line)
		if ruleType == "" || value == "" {
			return rules
		}
		if policyOverride != "" {
			policy = policyOverride
		}
		outbound := resolveTarget(policy)
		if outbound == "" {
			return rules
		}
		if ruleType == "RULE-SET" {
			return appendRuleSet(rules, value, outbound, clash, homeDir)
		}
		if rule := convertSingleRule(homeDir, ruleType, value, outbound, flags); rule != nil {
			rules = append(rules, rule)
		}
		return rules
	}

	var convertAnyRule func(rules []map[string]any, raw any) []map[string]any
	convertAnyRule = func(rules []map[string]any, raw any) []map[string]any {
		line, ok := raw.(string)
		if !ok {
			return rules
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return rules
		}
		upper := strings.ToUpper(line)
		if strings.HasPrefix(upper, "AND,") || strings.HasPrefix(upper, "OR,") || strings.HasPrefix(upper, "NOT,") {
			if rule := convertLogicRule(homeDir, line, resolveTarget); rule != nil {
				rules = append(rules, rule)
			}
			return rules
		}
		if strings.HasPrefix(upper, "SUB-RULE,") {
			parts := strings.SplitN(line, ",", 3)
			if len(parts) >= 3 {
				if sub, ok := clash.SubRules[strings.TrimSpace(parts[1])]; ok {
					for _, subRaw := range sub {
						rules = convertAnyRule(rules, subRaw)
					}
				}
			}
			return rules
		}
		return appendRuleLine(rules, line, "")
	}

	rules := []map[string]any{
		// Sniff outbound connections (migrated from legacy inbound sniff).
		{"action": "sniff"},
		// DNS hijack for tun clients (migrated from dns-out outbound).
		{"protocol": "dns", "action": "hijack-dns"},
	}
	for _, raw := range clash.Rules {
		rules = convertAnyRule(rules, raw)
	}

	mode := strings.ToLower(clash.Mode)
	finalOutbound := "DIRECT"
	switch mode {
	case "global":
		finalOutbound = "GLOBAL"
	case "direct":
		finalOutbound = "DIRECT"
	default:
		for _, raw := range clash.Rules {
			if line, ok := raw.(string); ok && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "MATCH,") {
				parts := strings.SplitN(line, ",", 3)
				if len(parts) >= 2 {
					if target := resolveTarget(strings.TrimSpace(parts[1])); target != "" {
						finalOutbound = target
					}
				}
			}
		}
	}

	// Bind to the default interface only when running as a TUN gateway; in
	// plain proxy mode binding picks up dead virtual adapters (Radmin/VMware)
	// and breaks DNS with EINVAL on Windows.
	route := map[string]any{
		"rules": rules,
		"final": finalOutbound,
	}
	if clash.Tun != nil && asBool(clash.Tun["enable"]) {
		route["auto_detect_interface"] = true
	}
	return route, finalOutbound
}

// parseClashRule parses "TYPE,VALUE,POLICY(,no-resolve|src)" and "MATCH,POLICY".
func parseClashRule(line string) (ruleType, value, policy string, flags map[string]bool) {
	flags = map[string]bool{}
	parts := strings.Split(line, ",")
	if len(parts) < 2 {
		return "", "", "", nil
	}
	ruleType = strings.ToUpper(strings.TrimSpace(parts[0]))
	tail := strings.ToLower(strings.TrimSpace(parts[len(parts)-1]))
	if tail == "no-resolve" || tail == "src" || tail == "no-drop" {
		flags[tail] = true
		parts = parts[:len(parts)-1]
	}
	if len(parts) < 2 {
		return "", "", "", nil
	}
	if ruleType == "MATCH" {
		return ruleType, "", strings.TrimSpace(parts[1]), flags
	}
	value = strings.TrimSpace(parts[1])
	if len(parts) > 2 {
		policy = strings.TrimSpace(parts[len(parts)-1])
	}
	return ruleType, value, policy, flags
}

// convertSingleRule maps one Clash rule to a sing-box rule entry.
func convertSingleRule(homeDir string, ruleType string, value string, outbound string, flags map[string]bool) map[string]any {
	rule := map[string]any{"outbound": outbound}
	switch ruleType {
	case "DOMAIN":
		rule["domain"] = []string{value}
	case "DOMAIN-SUFFIX":
		rule["domain_suffix"] = []string{value}
	case "DOMAIN-KEYWORD":
		rule["domain_keyword"] = []string{value}
	case "DOMAIN-REGEX":
		rule["domain_regex"] = []string{value}
	case "GEOSITE":
		// Expand from the bundled GEOSITE.dat (offline, no sing geosite db).
		entries := expandGeosite(homeDir, value)
		if len(entries) == 0 {
			logError("geosite %s not found in GEOSITE.dat", value)
			return nil
		}
		// v2ray types: Plain=0 (substring), Regex=1, Domain=2 (subdomain),
		// Full=3 (exact). sing-box fields: domain_keyword / domain_regex /
		// domain_suffix / domain.
		if list := geositeValues(entries, 3); len(list) > 0 {
			rule["domain"] = list
		}
		if list := geositeValues(entries, 2); len(list) > 0 {
			rule["domain_suffix"] = list
		}
		if list := geositeValues(entries, 0); len(list) > 0 {
			rule["domain_keyword"] = list
		}
		if list := geositeValues(entries, 1); len(list) > 0 {
			rule["domain_regex"] = list
		}
	case "GEOIP":
		invert := strings.HasPrefix(value, "!")
		code := strings.TrimPrefix(value, "!")
		prefixes := expandGeoIPPrefixes(homeDir, code)
		if len(prefixes) == 0 {
			logError("geoip %s not found in GEOIP.dat", code)
			return nil
		}
		cidrs := make([]string, 0, len(prefixes))
		for _, prefix := range prefixes {
			cidrs = append(cidrs, prefix.String())
		}
		rule["ip_cidr"] = cidrs
		if invert {
			rule["invert"] = true
		}
	case "SRC-GEOIP":
		rule["source_geoip"] = []string{value}
	case "IP-CIDR", "IP-CIDR6":
		rule["ip_cidr"] = []string{normalizeCidr(value)}
	case "SRC-IP-CIDR":
		rule["source_ip_cidr"] = []string{normalizeCidr(value)}
	case "IP-ASN":
		rule["ip_asn"] = []string{strings.TrimPrefix(value, "!")}
	case "SRC-IP-ASN":
		rule["source_ip_asn"] = []string{strings.TrimPrefix(value, "!")}
	case "SRC-PORT":
		rule["source_port"] = splitPortList(value)
	case "DST-PORT":
		rule["port"] = splitPortList(value)
	case "PROCESS-NAME":
		rule["process_name"] = []string{value}
	case "PROCESS-PATH":
		rule["process_path"] = []string{value}
	case "NETWORK":
		rule["network"] = []string{strings.ToLower(value)}
	default:
		return nil
	}
	return rule
}

// appendRuleSet inlines a Clash rule-provider (classical/domain/ipcidr yaml
// file, pre-downloaded by the Dart side) as expanded rules.
func appendRuleSet(rules []map[string]any, name string, outbound string, clash *ClashConfig, homeDir string) []map[string]any {
	provider, ok := clash.RuleProviders[name]
	if !ok {
		logError("rule-set %s not found", name)
		return rules
	}
	path := providerPath(homeDir, provider["path"])
	if path == "" {
		return rules
	}
	data, err := os.ReadFile(path)
	if err != nil {
		logError("rule-set %s read error: %v", name, err)
		return rules
	}
	behavior := strings.ToLower(asString(provider["behavior"]))
	var parsed struct {
		Payload []any `yaml:"payload"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		logError("rule-set %s parse error: %v", name, err)
		return rules
	}
	for _, raw := range parsed.Payload {
		line, ok := raw.(string)
		if !ok {
			continue
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ruleType, value string
		var flags map[string]bool
		switch behavior {
		case "domain":
			ruleType = "DOMAIN-SUFFIX"
			if strings.HasPrefix(line, "+.") {
				value = strings.TrimPrefix(line, "+.")
			} else if strings.HasPrefix(line, ".") {
				value = strings.TrimPrefix(line, ".")
			} else {
				ruleType = "DOMAIN"
				value = line
			}
		case "ipcidr":
			ruleType = "IP-CIDR"
			value = line
		default:
			ruleType, value, _, flags = parseClashRule(line)
			if ruleType == "MATCH" {
				continue
			}
		}
		if ruleType == "" || value == "" {
			continue
		}
		if rule := convertSingleRule(homeDir, ruleType, value, outbound, flags); rule != nil {
			rules = append(rules, rule)
		}
	}
	return rules
}

func convertLogicRule(homeDir string, line string, resolveTarget func(string) string) map[string]any {
	comma := strings.Index(line, ",")
	if comma <= 0 {
		return nil
	}
	mode := strings.ToLower(line[:comma])
	rest := line[comma+1:]
	if !strings.HasPrefix(rest, "(") {
		return nil
	}
	depth := 0
	end := -1
	for i, ch := range rest {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return nil
	}
	inner := rest[1:end]
	tail := strings.TrimSpace(rest[end+1:])
	tail = strings.TrimPrefix(tail, ",")
	policy := ""
	for _, part := range strings.Split(tail, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			policy = part
			break
		}
	}
	outbound := resolveTarget(policy)
	if outbound == "" {
		return nil
	}

	subRules := []map[string]any{}
	for _, sub := range splitTopLevel(inner, ',') {
		sub = strings.TrimSpace(sub)
		sub = strings.TrimSuffix(sub, ",")
		if sub == "" {
			continue
		}
		upper := strings.ToUpper(sub)
		if strings.HasPrefix(upper, "AND,") || strings.HasPrefix(upper, "OR,") || strings.HasPrefix(upper, "NOT,") {
			if logic := convertLogicRule(homeDir, sub, resolveTarget); logic != nil {
				logic["outbound"] = outbound
				subRules = append(subRules, logic)
			}
			continue
		}
		ruleType, value, _, flags := parseClashRule(sub)
		if ruleType == "" || value == "" {
			continue
		}
		if rule := convertSingleRule(homeDir, ruleType, value, outbound, flags); rule != nil {
			subRules = append(subRules, rule)
		}
	}
	if len(subRules) == 0 {
		return nil
	}
	return map[string]any{
		"type":  "logical",
		"mode":  mode,
		"rules": subRules,
	}
}

func splitTopLevel(input string, sep rune) []string {
	var out []string
	depth := 0
	current := strings.Builder{}
	for _, ch := range input {
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
		}
		if ch == sep && depth == 0 {
			out = append(out, current.String())
			current.Reset()
			continue
		}
		current.WriteRune(ch)
	}
	out = append(out, current.String())
	return out
}

func normalizeCidr(value string) string {
	if strings.Contains(value, "/") {
		return value
	}
	if strings.Contains(value, ":") {
		return value + "/128"
	}
	return value + "/32"
}

func splitPortList(value string) []int {
	out := []int{}
	for _, part := range strings.Split(value, "/") {
		if port := asInt(strings.TrimSpace(part), 0); port > 0 {
			out = append(out, port)
		}
	}
	if len(out) == 0 {
		out = append(out, asInt(value, 0))
	}
	return out
}

func itoa(number int) string {
	return strconv.Itoa(number)
}

// geositeValues extracts values whose proto type equals targetType.
func geositeValues(entries []domainEntry, targetType int) []string {
	out := []string{}
	for _, entry := range entries {
		if entry.dtype == targetType {
			out = append(out, entry.value)
		}
	}
	return dedupeStrings(out)
}
