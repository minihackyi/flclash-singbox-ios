package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singjson "github.com/sagernet/sing/common/json"
)

type Engine struct {
	mu sync.Mutex

	homeDir   string
	version   int
	isInit    bool
	isRunning bool

	clash       *ClashConfig
	testURL     string
	selectedMap map[string]string

	box      *box.Box
	boxCtx   context.Context
	boxClose context.CancelFunc

	apiBase  string
	apiToken string

	// traffic accounting fed by the /traffic websocket pump.
	lastUp    int64
	lastDown  int64
	totalUp   int64
	totalDown int64

	// connection pump state for request events.
	knownConns map[string]struct{}
	connMu     sync.Mutex

	logCancel     context.CancelFunc
	trafficCancel context.CancelFunc
	connCancel    context.CancelFunc

	apiPort int
}

var engine = &Engine{knownConns: map[string]struct{}{}}

func shutdownEngine() {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.stopPumpsLocked()
	engine.stopBoxLocked()
}

func (e *Engine) handleInitClash(paramsString string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	var params = InitParams{}
	if err := json.Unmarshal([]byte(paramsString), &params); err != nil {
		return false
	}
	e.version = params.Version
	e.homeDir = params.HomeDir
	e.isInit = true
	return true
}

func (e *Engine) handleGetIsInit() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.isInit
}

func (e *Engine) handleForceGC() {
	runtime.GC()
	debug.FreeOSMemory()
}

func (e *Engine) handleShutdown() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopPumpsLocked()
	e.stopBoxLocked()
	e.isInit = false
	return true
}

func (e *Engine) stopPumpsLocked() {
	if e.logCancel != nil {
		e.logCancel()
		e.logCancel = nil
	}
	if e.trafficCancel != nil {
		e.trafficCancel()
		e.trafficCancel = nil
	}
	if e.connCancel != nil {
		e.connCancel()
		e.connCancel = nil
	}
}

func (e *Engine) stopBoxLocked() {
	if e.box != nil {
		_ = e.box.Close()
		e.box = nil
	}
	e.isRunning = false
}

func (e *Engine) handleValidateConfig(path string) string {
	clash, err := readClashConfig(path)
	if err != nil {
		return err.Error()
	}
	_, err = convertToSingBox(clash, e.homeDir, map[string]string{}, "", clash.Mode)
	if err != nil {
		return err.Error()
	}
	return ""
}

func (e *Engine) handleGetConfig(path string) (map[string]any, error) {
	return readConfigRawMap(path)
}

func (e *Engine) handleSetupConfig(data []byte) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.isInit {
		return "not initialized"
	}
	var params = SetupParams{TestURL: "https://www.gstatic.com/generate_204"}
	if err := unmarshalUseNumber(data, &params); err != nil {
		return err.Error()
	}
	if params.TestURL == "" {
		params.TestURL = "https://www.gstatic.com/generate_204"
	}
	if err := e.applyConfigLocked(params); err != nil {
		return err.Error()
	}
	return ""
}

func (e *Engine) applyConfigLocked(params SetupParams) error {
	configPath := filepath.Join(e.homeDir, "config.yaml")
	clash, err := readClashConfig(configPath)
	if err != nil {
		return err
	}
	e.clash = clash
	e.testURL = params.TestURL
	if params.SelectedMap != nil {
		e.selectedMap = params.SelectedMap
	} else {
		e.selectedMap = map[string]string{}
	}
	return e.startBoxLocked()
}

func (e *Engine) handleUpdateConfig(data []byte) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.isInit || e.clash == nil {
		return "not initialized"
	}
	var params = UpdateParams{}
	if err := unmarshalUseNumber(data, &params); err != nil {
		return err.Error()
	}
	clash := e.clash
	if params.MixedPort != nil {
		clash.MixedPort = *params.MixedPort
	}
	if params.AllowLan != nil {
		clash.AllowLan = *params.AllowLan
	}
	if params.Mode != nil {
		clash.Mode = strings.ToLower(*params.Mode)
	}
	if params.LogLevel != nil {
		clash.LogLevel = *params.LogLevel
	}
	if params.IPv6 != nil {
		clash.IPv6 = *params.IPv6
	}
	if params.ExternalController != nil && *params.ExternalController != "" {
		clash.ExternalController = *params.ExternalController
	}
	if params.Tun != nil {
		if clash.Tun == nil {
			clash.Tun = map[string]any{}
		}
		if params.Tun.Enable != nil {
			clash.Tun["enable"] = *params.Tun.Enable
		}
		if params.Tun.Device != nil {
			clash.Tun["device"] = *params.Tun.Device
		}
		if params.Tun.Stack != nil {
			clash.Tun["stack"] = *params.Tun.Stack
		}
		if params.Tun.DNSHijack != nil {
			clash.Tun["dns-hijack"] = *params.Tun.DNSHijack
		}
		if params.Tun.AutoRoute != nil {
			clash.Tun["auto-route"] = *params.Tun.AutoRoute
		}
		if params.Tun.RouteAddress != nil {
			clash.Tun["route-address"] = *params.Tun.RouteAddress
		}
	}
	// Re-parse through yaml round trip is unnecessary; convert directly.
	if e.isRunning {
		if err := e.startBoxLocked(); err != nil {
			return err.Error()
		}
	}
	return ""
}

func (e *Engine) handleStartListener() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.box != nil {
		return true
	}
	if e.clash == nil {
		// No profile applied yet; just mark as running.
		e.isRunning = true
		return true
	}
	if err := e.startBoxLocked(); err != nil {
		logError("startListener error: %v", err)
		return true
	}
	return true
}

func (e *Engine) handleStopListener() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.stopPumpsLocked()
	e.stopBoxLocked()
	return true
}

func (e *Engine) startBoxLocked() error {
	e.stopPumpsLocked()
	e.stopBoxLocked()

	converted, err := convertToSingBox(e.clash, e.homeDir, e.selectedMap, e.testURL, e.clash.Mode)
	if err != nil {
		return err
	}
	// Persist the generated config for debugging.
	_ = os.WriteFile(filepath.Join(e.homeDir, "singbox-config.json"), mustJsonPretty(converted.config), 0o644)

	apiAddress := e.clash.ExternalController
	if apiAddress == "" {
		port, err := pickFreePort()
		if err != nil {
			return err
		}
		apiAddress = "127.0.0.1:" + itoa(port)
	}
	experimental, _ := converted.config["experimental"].(map[string]any)
	if experimental == nil {
		experimental = map[string]any{}
		converted.config["experimental"] = experimental
	}
	clashApi, _ := experimental["clash_api"].(map[string]any)
	if clashApi == nil {
		clashApi = map[string]any{}
		experimental["clash_api"] = clashApi
	}
	clashApi["external_controller"] = apiAddress
	if secret := e.clash.Secret; secret != "" {
		clashApi["secret"] = secret
		e.apiToken = secret
	} else {
		delete(clashApi, "secret")
		e.apiToken = ""
	}
	clashApi["default_mode"] = "rule"
	e.apiBase = "http://" + apiAddress

	registryCtx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(), include.DNSTransportRegistry(), include.ServiceRegistry())
	// On iOS this registers the NetworkExtension-backed platform interface
	// (tun inbound + interface binding); no-op elsewhere.
	registryCtx = withIOSPlatform(registryCtx)
	options, err := singjson.UnmarshalExtendedContext[option.Options](registryCtx, mustJson(converted.config))
	if err != nil {
		return fmt.Errorf("parse sing-box options: %w", err)
	}
	ctx, cancel := context.WithCancel(registryCtx)
	ctx = box.Context(ctx, include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry(), include.DNSTransportRegistry(), include.ServiceRegistry())
	instance, err := box.New(box.Options{
		Options: options,
		Context: ctx,
	})
	if err != nil {
		cancel()
		return fmt.Errorf("create sing-box: %w", err)
	}
	if err := instance.Start(); err != nil {
		_ = instance.Close()
		cancel()
		return fmt.Errorf("start sing-box: %w", err)
	}
	e.box = instance
	e.boxCtx = ctx
	e.boxClose = cancel
	e.isRunning = true

	// Wait briefly for the clash api to come up, then start pumps.
	go func() {
		if waitHttpReady(e.apiBase+"/version", 5*time.Second) {
			engine.startPumps()
		} else {
			logError("clash api not reachable at %s", e.apiBase)
		}
	}()

	return nil
}

func (e *Engine) startPumps() {
	ctx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.logCancel != nil {
		e.logCancel()
	}
	e.logCancel = cancel
	e.mu.Unlock()
	go pumpLogs(ctx, e.apiBase, e.apiToken)

	ctx2, cancel2 := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.trafficCancel != nil {
		e.trafficCancel()
	}
	e.trafficCancel = cancel2
	e.mu.Unlock()
	go pumpTraffic(ctx2, e.apiBase, e.apiToken)

	ctx3, cancel3 := context.WithCancel(context.Background())
	e.mu.Lock()
	if e.connCancel != nil {
		e.connCancel()
	}
	e.connCancel = cancel3
	e.mu.Unlock()
	go pumpConnections(ctx3, e.apiBase, e.apiToken)
}

func (e *Engine) handleGetProxies() map[string]any {
	data := fetchClashApiMap(e.apiBase+"/proxies", e.apiToken)
	if data == nil {
		return map[string]any{"all": []string{}, "proxies": map[string]any{}}
	}
	proxiesRaw, _ := data["proxies"].(map[string]any)
	proxies := map[string]any{}
	all := []string{}
	for name, raw := range proxiesRaw {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		proxyType := asString(entry["type"])
		normalized := map[string]any{
			"name": name,
			"type": proxyType,
		}
		if now, ok := entry["now"]; ok {
			normalized["now"] = now
		}
		if history, ok := entry["history"]; ok {
			normalized["history"] = history
		}
		if udp, ok := entry["udp"]; ok {
			normalized["udp"] = udp
		}
		if isGroupType(proxyType) {
			if memberList, ok := entry["all"]; ok {
				normalized["all"] = memberList
			}
			normalized["type"] = normalizeGroupType(proxyType)
			if name != "GLOBAL" {
				all = append(all, name)
			}
		}
		proxies[name] = normalized
	}
	if globalEntry, ok := proxiesRaw["GLOBAL"].(map[string]any); ok {
		proxies["GLOBAL"] = map[string]any{
			"name": "GLOBAL",
			"type": "select",
			"now":  globalEntry["now"],
			"all":  globalEntry["all"],
		}
	} else {
		all = append([]string{"GLOBAL"}, all...)
	}
	return map[string]any{"all": all, "proxies": proxies}
}

func isGroupType(proxyType string) bool {
	switch strings.ToLower(proxyType) {
	case "selector", "urltest", "url-test", "fallback", "loadbalance", "load-balance", "relay":
		return true
	}
	return false
}

func normalizeGroupType(proxyType string) string {
	switch strings.ToLower(proxyType) {
	case "selector":
		return "select"
	case "urltest", "url-test":
		return "url-test"
	case "fallback":
		return "fallback"
	case "loadbalance", "load-balance":
		return "load-balance"
	case "relay":
		return "relay"
	}
	return "select"
}

func (e *Engine) handleChangeProxy(data string, fn func(string)) {
	go func() {
		var params = &ChangeProxyParams{}
		if err := json.Unmarshal([]byte(data), params); err != nil {
			fn(err.Error())
			return
		}
		groupName := *params.GroupName
		proxyName := *params.ProxyName
		url := fmt.Sprintf("%s/proxies/%s", e.apiBase, urlPathEscape(groupName))
		body := map[string]any{"name": proxyName}
		status, respErr := callClashApi("PUT", url, e.apiToken, body)
		if respErr != nil || status/100 != 2 {
			fn(fmt.Sprintf("change proxy failed: status=%d err=%v", status, respErr))
			return
		}
		fn("")
	}()
}

func (e *Engine) handleAsyncTestDelay(paramsString string, fn func(string)) {
	go func() {
		var params = &TestDelayParams{}
		if err := json.Unmarshal([]byte(paramsString), params); err != nil {
			fn("")
			return
		}
		testUrl := params.TestUrl
		if testUrl == "" {
			e.mu.Lock()
			testUrl = e.testURL
			e.mu.Unlock()
		}
		if testUrl == "" {
			testUrl = "https://www.gstatic.com/generate_204"
		}
		timeoutMs := params.Timeout
		if timeoutMs <= 0 {
			timeoutMs = 5000
		}
		delayData := &Delay{
			Url:  testUrl,
			Name: params.ProxyName,
		}
		url := fmt.Sprintf("%s/proxies/%s/delay?timeout=%d&url=%s", e.apiBase, urlPathEscape(params.ProxyName), timeoutMs, urlQueryEscape(testUrl))
		body := fetchClashApiMap(url, e.apiToken)
		if body == nil {
			delayData.Value = -1
		} else if delay, ok := body["delay"].(float64); ok {
			delayData.Value = int32(delay)
		} else {
			delayData.Value = -1
		}
		data, _ := json.Marshal(delayData)
		fn(string(data))
	}()
}

func (e *Engine) handleGetConnections() string {
	return fetchClashApiRaw(e.apiBase+"/connections", e.apiToken)
}

func (e *Engine) handleCloseConnection(id string) bool {
	url := fmt.Sprintf("%s/connections/%s", e.apiBase, urlPathEscape(id))
	status, _ := callClashApi("DELETE", url, e.apiToken, nil)
	return status/100 == 2
}

func (e *Engine) handleCloseConnections() bool {
	status, _ := callClashApi("DELETE", e.apiBase+"/connections", e.apiToken, nil)
	return status/100 == 2
}

func (e *Engine) handleResetConnections() bool {
	e.connMu.Lock()
	e.knownConns = map[string]struct{}{}
	e.connMu.Unlock()
	return true
}

func (e *Engine) handleGetTraffic(onlyStatisticsProxy bool) string {
	e.mu.Lock()
	up, down := e.lastUp, e.lastDown
	e.mu.Unlock()
	data, err := json.Marshal(map[string]int64{"up": up, "down": down})
	if err != nil {
		return ""
	}
	return string(data)
}

func (e *Engine) handleGetTotalTraffic(onlyStatisticsProxy bool) string {
	e.mu.Lock()
	up, down := e.totalUp, e.totalDown
	e.mu.Unlock()
	data, err := json.Marshal(map[string]int64{"up": up, "down": down})
	if err != nil {
		return ""
	}
	return string(data)
}

func (e *Engine) handleResetTraffic() {
	e.mu.Lock()
	e.totalUp = 0
	e.totalDown = 0
	e.mu.Unlock()
}

func (e *Engine) handleGetMemory(fn func(value string)) {
	go func() {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		fn(strconv.FormatUint(mem.HeapAlloc, 10))
	}()
}

func (e *Engine) handleStartLog() {
	// Pumps are restarted by the api-ready hook after box (re)start.
	e.startPumps()
}

func (e *Engine) handleStopLog() {
	e.mu.Lock()
	cancel := e.logCancel
	e.logCancel = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (e *Engine) handleUpdateGeoData(geoType string) {
	sendMessage(Message{
		Type: GeoUpdateMessage,
		Data: GeoUpdateStatus{Type: geoType, Updating: false, Skipped: true},
	})
}

func (e *Engine) handleGetExternalProviders() string {
	return "[]"
}

func (e *Engine) handleGetExternalProvider(name string) string {
	return ""
}

func pickFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func mustJson(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func mustJsonPretty(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return data
}

func unmarshalUseNumber(data []byte, v any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	return decoder.Decode(v)
}

func waitHttpReady(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, _ := callClashApi("GET", url, "", nil)
		if status/100 == 2 {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func handleDelFile(path string, result ActionResult) {
	go func() {
		fileInfo, err := os.Stat(path)
		if err != nil {
			if !os.IsNotExist(err) {
				result.success(err.Error())
				return
			}
			result.success("")
			return
		}
		if fileInfo.IsDir() {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil {
			result.success(err.Error())
			return
		}
		result.success("")
	}()
}
