//go:build ignore

package main

// Smoke-test client: mimics the Dart IPCCoreTransport side. Creates a named
// pipe server at the given name, launches FlClashCore.exe against it, and
// drives a full action round.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Microsoft/go-winio"
)

type frameConn struct {
	conn net.Conn
	mu   sync.Mutex
}

func (f *frameConn) write(data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	frame := make([]byte, 4+len(data))
	binary.LittleEndian.PutUint32(frame, uint32(len(data)))
	copy(frame[4:], data)
	_, err := f.conn.Write(frame)
	return err
}

func (f *frameConn) read() ([]byte, error) {
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(f.conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.LittleEndian.Uint32(lenBuf)
	data := make([]byte, length)
	if _, err := io.ReadFull(f.conn, data); err != nil {
		return nil, err
	}
	return data, nil
}

type pending struct {
	ch chan map[string]any
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: smoke <core.exe> <profile.yaml>")
		os.Exit(1)
	}
	corePath, _ := filepath.Abs(os.Args[1])
	profilePath, _ := filepath.Abs(os.Args[2])
	homeDir := filepath.Dir(profilePath)

	pipeName := `\\.\pipe\flclash-smoke-test`
	listener, err := winio.ListenPipe(pipeName, nil)
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(corePath, pipeName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	defer cmd.Process.Kill()

	conn, err := listener.Accept()
	if err != nil {
		panic(err)
	}
	fmt.Println("== core connected ==")
	fc := &frameConn{conn: conn}

	events := make(chan map[string]any, 100)
	responses := map[string]chan map[string]any{}
	var respMu sync.Mutex

	go func() {
		for {
			data, err := fc.read()
			if err != nil {
				return
			}
			var msg map[string]any
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}
			id, _ := msg["id"].(string)
			if id == "" {
				events <- msg
				continue
			}
			respMu.Lock()
			ch := responses[id]
			respMu.Unlock()
			if ch != nil {
				ch <- msg
			}
		}
	}()

	invoke := func(method string, data any, timeout time.Duration) map[string]any {
		id := fmt.Sprintf("%s#%d", method, time.Now().UnixNano())
		ch := make(chan map[string]any, 1)
		respMu.Lock()
		responses[id] = ch
		respMu.Unlock()
		action := map[string]any{"id": id, "method": method, "data": data}
		raw, _ := json.Marshal(action)
		if err := fc.write(raw); err != nil {
			panic(err)
		}
		select {
		case resp := <-ch:
			respMu.Lock()
			delete(responses, id)
			respMu.Unlock()
			return resp
		case <-time.After(timeout):
			return map[string]any{"timeout": true}
		}
	}

	expectNoError := func(resp map[string]any, what string) {
		if resp["timeout"] == true {
			fmt.Printf("[FAIL] %s: timeout\n", what)
			os.Exit(1)
		}
		if code, _ := resp["code"].(float64); code != 0 {
			fmt.Printf("[FAIL] %s: code=%v data=%v\n", what, resp["code"], resp["data"])
			os.Exit(1)
		}
	}

	// 1. init
	resp := invoke("initClash", map[string]any{"home-dir": homeDir, "version": 1}, 5*time.Second)
	expectNoError(resp, "initClash")
	fmt.Println("[OK] initClash ->", resp["data"])

	// 2. validateConfig
	resp = invoke("validateConfig", profilePath, 10*time.Second)
	expectNoError(resp, "validateConfig")
	fmt.Println("[OK] validateConfig ->", resp["data"])

	// 3. getConfig
	resp = invoke("getConfig", profilePath, 10*time.Second)
	expectNoError(resp, "getConfig")
	if data, ok := resp["data"].(map[string]any); ok {
		fmt.Println("[OK] getConfig keys count:", len(data))
	} else {
		fmt.Println("[OK] getConfig data is", resp["data"])
	}

	// 4. setupConfig
	resp = invoke("setupConfig", map[string]any{"selected-map": map[string]string{}, "test-url": "https://www.gstatic.com/generate_204"}, 30*time.Second)
	expectNoError(resp, "setupConfig")
	fmt.Println("[OK] setupConfig ->", resp["data"])

	// 5. startListener
	resp = invoke("startListener", nil, 15*time.Second)
	expectNoError(resp, "startListener")
	fmt.Println("[OK] startListener")

	// 6. getProxies
	resp = invoke("getProxies", nil, 10*time.Second)
	expectNoError(resp, "getProxies")
	if data, ok := resp["data"].(map[string]any); ok {
		all, _ := data["all"].([]any)
		proxies, _ := data["proxies"].(map[string]any)
		fmt.Printf("[OK] getProxies: %d groups + GLOBAL, %d proxies\n", len(all), len(proxies))
		for _, nameAny := range all {
			name, _ := nameAny.(string)
			if p, ok := proxies[name].(map[string]any); ok {
				fmt.Printf("     group %-20s type=%v now=%v members=%v\n", name, p["type"], p["now"], p["all"])
			}
		}
	}

	// 7. getTraffic
	resp = invoke("getTraffic", false, 5*time.Second)
	expectNoError(resp, "getTraffic")
	fmt.Println("[OK] getTraffic ->", resp["data"])

	// 8. getConnections
	resp = invoke("getConnections", nil, 5*time.Second)
	expectNoError(resp, "getConnections")
	fmt.Println("[OK] getConnections ->", truncate(fmt.Sprint(resp["data"]), 120))

	// 9. getMemory
	resp = invoke("getMemory", nil, 5*time.Second)
	expectNoError(resp, "getMemory")
	fmt.Println("[OK] getMemory ->", resp["data"])

	// 9.5 real request through the mixed port (qq.com route is DIRECT).
	// The machine running this test may host a live proxy client that
	// intermittently interferes with DNS; retry a few times.
	func() {
		proxyURL, _ := neturl.Parse("http://127.0.0.1:7897")
		client := &http.Client{
			Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
			Timeout:   8 * time.Second,
		}
		lastErr := ""
		for attempt := 0; attempt < 5; attempt++ {
			httpResp, err := client.Get("http://www.qq.com/")
			if err != nil {
				lastErr = err.Error()
			} else {
				io.Copy(io.Discard, httpResp.Body)
				httpResp.Body.Close()
				// Any HTTP response proves the proxy round trip; some servers
				// (qq.com) answer simplified relayed requests with 30x/501.
				if httpResp.StatusCode >= 200 && httpResp.StatusCode < 600 {
					fmt.Printf("[OK] request through core: status=%d (attempt %d)\n", httpResp.StatusCode, attempt+1)
					return
				}
				lastErr = fmt.Sprintf("status=%d", httpResp.StatusCode)
			}
			time.Sleep(2 * time.Second)
		}
		fmt.Println("[FAIL] request through core:", lastErr)
		os.Exit(1)
	}()

	// 10. changeProxy (pick first group and first member)
	resp = invoke("getProxies", nil, 10*time.Second)
	if data, ok := resp["data"].(map[string]any); ok {
		all, _ := data["all"].([]any)
		proxies, _ := data["proxies"].(map[string]any)
		if len(all) > 1 {
			groupName, _ := all[1].(string)
			if group, ok := proxies[groupName].(map[string]any); ok {
				if members, ok := group["all"].([]any); ok && len(members) > 0 {
					member := members[0].(string)
					resp = invoke("changeProxy", map[string]any{"group-name": groupName, "proxy-name": member}, 10*time.Second)
					expectNoError(resp, "changeProxy")
					fmt.Printf("[OK] changeProxy %s -> %s\n", groupName, member)
				}
			}
		}
	}

	// 11. asyncTestDelay on DIRECT
	resp = invoke("asyncTestDelay", map[string]any{"proxy-name": "DIRECT", "test-url": "https://www.gstatic.com/generate_204", "timeout": 5000}, 15*time.Second)
	expectNoError(resp, "asyncTestDelay")
	fmt.Println("[OK] asyncTestDelay DIRECT ->", resp["data"])

	// 12. collect events for 3s (logs / delay / request)
	fmt.Println("== collecting events for 3s ==")
	deadline := time.Now().Add(3 * time.Second)
	logCount, delayCount, requestCount := 0, 0, 0
	for time.Now().Before(deadline) {
		select {
		case ev := <-events:
			if data, ok := ev["data"].(map[string]any); ok {
				switch data["type"] {
				case "log":
					logCount++
					if logCount <= 3 {
						fmt.Printf("     [log] %v\n", truncate(fmt.Sprint(data["data"]), 100))
					}
				case "delay":
					delayCount++
					fmt.Printf("     [delay] %v\n", data["data"])
				case "request":
					requestCount++
				}
			}
		case <-time.After(100 * time.Millisecond):
		}
	}
	fmt.Printf("[OK] events: %d logs, %d delays, %d requests\n", logCount, delayCount, requestCount)

	// 13. stopListener + shutdown
	resp = invoke("stopListener", nil, 10*time.Second)
	expectNoError(resp, "stopListener")
	resp = invoke("shutdown", true, 10*time.Second)
	expectNoError(resp, "shutdown")
	fmt.Println("[OK] shutdown")

	fmt.Println("== ALL SMOKE TESTS PASSED ==")
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
