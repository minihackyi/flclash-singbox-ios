package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Method string

type MessageType string

type Action struct {
	Id     string      `json:"id"`
	Method Method      `json:"method"`
	Data   interface{} `json:"data"`
}

type ActionResult struct {
	Id     string      `json:"id"`
	Method Method      `json:"method"`
	Data   interface{} `json:"data"`
	Code   int         `json:"code"`
}

func (result *ActionResult) Json() ([]byte, error) {
	data, err := json.Marshal(result)
	return data, err
}

func (result *ActionResult) success(data interface{}) {
	result.Code = 0
	result.Data = data
	result.send()
}

func (result *ActionResult) error(data interface{}) {
	result.Code = -1
	result.Data = data
	result.send()
}

const (
	messageMethod                  Method = "message"
	initClashMethod                Method = "initClash"
	getIsInitMethod                Method = "getIsInit"
	forceGcMethod                  Method = "forceGc"
	shutdownMethod                 Method = "shutdown"
	validateConfigMethod           Method = "validateConfig"
	updateConfigMethod             Method = "updateConfig"
	getProxiesMethod               Method = "getProxies"
	changeProxyMethod              Method = "changeProxy"
	getTrafficMethod               Method = "getTraffic"
	getTotalTrafficMethod          Method = "getTotalTraffic"
	resetTrafficMethod             Method = "resetTraffic"
	asyncTestDelayMethod           Method = "asyncTestDelay"
	getConnectionsMethod           Method = "getConnections"
	closeConnectionsMethod         Method = "closeConnections"
	resetConnectionsMethod         Method = "resetConnections"
	closeConnectionMethod          Method = "closeConnection"
	getExternalProvidersMethod     Method = "getExternalProviders"
	getExternalProviderMethod      Method = "getExternalProvider"
	getCountryCodeMethod           Method = "getCountryCode"
	getMemoryMethod                Method = "getMemory"
	updateGeoDataMethod            Method = "updateGeoData"
	updateExternalProviderMethod   Method = "updateExternalProvider"
	sideLoadExternalProviderMethod Method = "sideLoadExternalProvider"
	startLogMethod                 Method = "startLog"
	stopLogMethod                  Method = "stopLog"
	startListenerMethod            Method = "startListener"
	stopListenerMethod             Method = "stopListener"
	updateDnsMethod                Method = "updateDns"
	crashMethod                    Method = "crash"
	setupConfigMethod              Method = "setupConfig"
	getConfigMethod                Method = "getConfig"
	deleteFileMethod               Method = "deleteFile"
)

type InitParams struct {
	HomeDir string `json:"home-dir"`
	Version int    `json:"version"`
}

type SetupParams struct {
	SelectedMap map[string]string `json:"selected-map"`
	TestURL     string            `json:"test-url"`
}

// UpdateParams mirrors lib/models/core.dart UpdateParams.
type UpdateParams struct {
	Tun                *tunSchema `json:"tun"`
	AllowLan           *bool      `json:"allow-lan"`
	MixedPort          *int       `json:"mixed-port"`
	FindProcessMode    *string    `json:"find-process-mode"`
	Mode               *string    `json:"mode"`
	LogLevel           *string    `json:"log-level"`
	IPv6               *bool      `json:"ipv6"`
	Sniffing           *bool      `json:"sniffing"`
	TCPConcurrent      *bool      `json:"tcp-concurrent"`
	ExternalController *string    `json:"external-controller"`
	Interface          *string    `json:"interface-name"`
	UnifiedDelay       *bool      `json:"unified-delay"`
	GeoAutoUpdate      *bool      `json:"geo-auto-update"`
	GeoUpdateInterval  *int       `json:"geo-update-interval"`
}

type tunSchema struct {
	Enable       *bool     `json:"enable"`
	Device       *string   `json:"device"`
	Stack        *string   `json:"stack"`
	DNSHijack    *[]string `json:"dns-hijack"`
	AutoRoute    *bool     `json:"auto-route"`
	RouteAddress *[]string `json:"route-address"`
}

type ChangeProxyParams struct {
	GroupName *string `json:"group-name"`
	ProxyName *string `json:"proxy-name"`
}

type TestDelayParams struct {
	ProxyName string `json:"proxy-name"`
	TestUrl   string `json:"test-url"`
	Timeout   int64  `json:"timeout"`
}

type Message struct {
	Type MessageType `json:"type"`
	Data interface{} `json:"data"`
}

const (
	LogMessage       MessageType = "log"
	DelayMessage     MessageType = "delay"
	RequestMessage   MessageType = "request"
	LoadedMessage    MessageType = "loaded"
	GeoUpdateMessage MessageType = "geoUpdate"
)

type Delay struct {
	Url   string `json:"url"`
	Name  string `json:"name"`
	Value int32  `json:"value"`
}

// Log matches mihomo log.Event json tags consumed by Dart Log model.
type Log struct {
	LogLevel string `json:"LogLevel"`
	Payload  string `json:"Payload"`
}

type GeoUpdateStatus struct {
	Type     string `json:"type"`
	Updating bool   `json:"updating"`
	Skipped  bool   `json:"skipped,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (message *Message) Json() (string, error) {
	data, err := json.Marshal(message)
	return string(data), err
}

var debugError = true

func printToStderr(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format, args...)
}

func logError(format string, args ...interface{}) {
	if debugError {
		printToStderr("[ERROR] "+format+"\n", args...)
	}
}
