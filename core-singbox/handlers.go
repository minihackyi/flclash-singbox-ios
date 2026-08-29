package main

import (
	"encoding/json"
	"fmt"
	"runtime"
)

// actionDataString decodes action.Data that the Dart side passed as a JSON
// string; it tolerates maps by re-encoding them.
func actionDataString(action *Action) (string, bool) {
	switch v := action.Data.(type) {
	case string:
		return v, true
	case nil:
		return "", true
	case map[string]any:
		data, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(data), true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

// actionDataBool decodes action.Data that should be a bool.
func actionDataBool(action *Action) bool {
	switch v := action.Data.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	case nil:
		return false
	default:
		return false
	}
}

func handleAction(action *Action, result ActionResult) {
	defer func() {
		if r := recover(); r != nil {
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			logError("panic in handleAction(%s): %v\n%s", action.Method, r, buf[:n])
			result.error(fmt.Sprintf("internal panic: %v", r))
		}
	}()
	switch action.Method {
	case initClashMethod:
		paramsString, ok := actionDataString(action)
		if !ok {
			result.error("invalid init params")
			return
		}
		result.success(engine.handleInitClash(paramsString))
		return
	case getIsInitMethod:
		result.success(engine.handleGetIsInit())
		return
	case forceGcMethod:
		engine.handleForceGC()
		result.success(true)
		return
	case shutdownMethod:
		result.success(engine.handleShutdown())
		return
	case validateConfigMethod:
		path, _ := actionDataString(action)
		result.success(engine.handleValidateConfig(path))
		return
	case updateConfigMethod:
		data, _ := actionDataString(action)
		result.success(engine.handleUpdateConfig([]byte(data)))
		return
	case setupConfigMethod:
		data, _ := actionDataString(action)
		result.success(engine.handleSetupConfig([]byte(data)))
		return
	case getConfigMethod:
		path, _ := actionDataString(action)
		config, err := engine.handleGetConfig(path)
		if err != nil {
			result.error(err.Error())
			return
		}
		result.success(config)
		return
	case getProxiesMethod:
		result.success(engine.handleGetProxies())
		return
	case changeProxyMethod:
		data, _ := actionDataString(action)
		engine.handleChangeProxy(data, func(value string) {
			result.success(value)
		})
		return
	case getTrafficMethod:
		result.success(engine.handleGetTraffic(actionDataBool(action)))
		return
	case getTotalTrafficMethod:
		result.success(engine.handleGetTotalTraffic(actionDataBool(action)))
		return
	case resetTrafficMethod:
		engine.handleResetTraffic()
		result.success(true)
		return
	case asyncTestDelayMethod:
		data, _ := actionDataString(action)
		engine.handleAsyncTestDelay(data, func(value string) {
			result.success(value)
		})
		return
	case getConnectionsMethod:
		result.success(engine.handleGetConnections())
		return
	case closeConnectionsMethod:
		result.success(engine.handleCloseConnections())
		return
	case resetConnectionsMethod:
		result.success(engine.handleResetConnections())
		return
	case closeConnectionMethod:
		id, _ := actionDataString(action)
		result.success(engine.handleCloseConnection(id))
		return
	case getExternalProvidersMethod:
		result.success(engine.handleGetExternalProviders())
		return
	case getExternalProviderMethod:
		externalProviderName, _ := actionDataString(action)
		result.success(engine.handleGetExternalProvider(externalProviderName))
		return
	case updateGeoDataMethod:
		geoType, _ := actionDataString(action)
		engine.handleUpdateGeoData(geoType)
		result.success("")
		return
	case updateExternalProviderMethod:
		go func() {
			result.success("")
		}()
		return
	case sideLoadExternalProviderMethod:
		go func() {
			result.success("")
		}()
		return
	case startLogMethod:
		engine.handleStartLog()
		result.success(true)
		return
	case stopLogMethod:
		engine.handleStopLog()
		result.success(true)
		return
	case startListenerMethod:
		result.success(engine.handleStartListener())
		return
	case stopListenerMethod:
		result.success(engine.handleStopListener())
		return
	case getCountryCodeMethod:
		ip, _ := actionDataString(action)
		handleGetCountryCode(ip, func(value string) {
			result.success(value)
		})
		return
	case getMemoryMethod:
		engine.handleGetMemory(func(value string) {
			result.success(value)
		})
		return
	case crashMethod:
		result.success(true)
		handleCrash()
	case deleteFileMethod:
		path, _ := actionDataString(action)
		handleDelFile(path, result)
		return
	default:
		result.success(nil)
	}
}

func handleCrash() {
	panic("handle invoke crash")
}
