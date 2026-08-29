package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClashConfig is the subset of the Clash/mihomo YAML profile that the
// FlClash Dart side writes to <homeDir>/config.yaml. Proxies/groups/rules are
// kept as raw maps because their schemas are open-ended.
type ClashConfig struct {
	Port               int    `yaml:"port"`
	SocksPort          int    `yaml:"socks-port"`
	MixedPort          int    `yaml:"mixed-port"`
	RedirPort          int    `yaml:"redir-port"`
	TProxyPort         int    `yaml:"tproxy-port"`
	AllowLan           bool   `yaml:"allow-lan"`
	BindAddress        string `yaml:"bind-address"`
	Mode               string `yaml:"mode"`
	LogLevel           string `yaml:"log-level"`
	IPv6               bool   `yaml:"ipv6"`
	TCPConcurrent      bool   `yaml:"tcp-concurrent"`
	UnifiedDelay       bool   `yaml:"unified-delay"`
	ExternalController string `yaml:"external-controller"`
	Secret             string `yaml:"secret"`
	FindProcessMode    string `yaml:"find-process-mode"`

	Tun     map[string]any `yaml:"tun"`
	Dns     map[string]any `yaml:"dns"`
	Sniffer map[string]any `yaml:"sniffer"`

	Proxies        []map[string]any          `yaml:"proxies"`
	ProxyGroups    []map[string]any          `yaml:"proxy-groups"`
	Rules          []any                     `yaml:"rules"`
	SubRules       map[string][]any          `yaml:"sub-rules"`
	ProxyProviders map[string]map[string]any `yaml:"proxy-providers"`
	RuleProviders  map[string]map[string]any `yaml:"rule-providers"`
}

func readClashConfig(path string) (*ClashConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseClashConfig(data)
}

func parseClashConfig(data []byte) (*ClashConfig, error) {
	config := &ClashConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}
	return config, nil
}

// toRawMap decodes the config back to a generic map, mirroring what mihomo's
// RawConfig JSON did for the Dart side (it reads keys like proxies/proxy-groups
// and rewrites general settings, then re-encodes to YAML).
func readConfigRawMap(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		raw = map[string]any{}
	}
	normalizeYamlNumbers(raw)
	return raw, nil
}

// yaml.v3 decodes unquoted ints as int; the Dart side can handle ints, but
// nested map[any]any must be converted to map[string]any for JSON encoding.
func normalizeYamlNumbers(value any) any {
	switch v := value.(type) {
	case map[any]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[fmt.Sprintf("%v", key)] = normalizeYamlNumbers(item)
		}
		return out
	case map[string]any:
		for key, item := range v {
			v[key] = normalizeYamlNumbers(item)
		}
		return v
	case []any:
		for i, item := range v {
			v[i] = normalizeYamlNumbers(item)
		}
		return v
	default:
		return value
	}
}

func providerPath(homeDir string, base any) string {
	if base == nil {
		return ""
	}
	path, _ := base.(string)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(homeDir, path)
}

var filterRegexCache = map[string]*regexp.Regexp{}

func matchFilter(filter string, name string) bool {
	if filter == "" {
		return true
	}
	re, ok := filterRegexCache[filter]
	if !ok {
		var err error
		re, err = regexp.Compile(filter)
		if err != nil {
			filterRegexCache[filter] = nil
			return true
		}
		filterRegexCache[filter] = re
	}
	if re == nil {
		return true
	}
	return re.MatchString(name)
}

func parseIntervalMs(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		value := strings.TrimSpace(v)
		if value == "" {
			return 0
		}
		if number, err := strconv.ParseInt(value, 10, 64); err == nil {
			return number
		}
		// Duration strings like "300s" / "5m".
		var seconds float64
		multiplier := 1.0
		rest := value
		switch {
		case strings.HasSuffix(value, "ms"):
			multiplier = 0.001
			rest = strings.TrimSuffix(value, "ms")
		case strings.HasSuffix(value, "s"):
			rest = strings.TrimSuffix(value, "s")
		case strings.HasSuffix(value, "m"):
			multiplier = 60
			rest = strings.TrimSuffix(value, "m")
		case strings.HasSuffix(value, "h"):
			multiplier = 3600
			rest = strings.TrimSuffix(value, "h")
		}
		if number, err := strconv.ParseFloat(rest, 64); err == nil {
			seconds = number * multiplier
			return int64(seconds * 1000)
		}
	}
	return 0
}
