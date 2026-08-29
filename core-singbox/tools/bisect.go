//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"time"
)

func loadBase() map[string]any {
	data, _ := os.ReadFile("testdata/singbox-config.json")
	var cfg map[string]any
	json.Unmarshal(data, &cfg)
	return cfg
}

func test(cfg map[string]any, name string) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile("testdata/"+name+".json", data, 0o644)
	cmd := exec.Command("FlClashCore.exe", "bisect-dummy") // won't connect; use runconfig approach instead
	_ = cmd
	// use go run runconfig
	proc := exec.Command("go", "run", "tools/runconfig.go", "testdata/"+name+".json")
	logFile, _ := os.Create("run_" + name + ".log")
	proc.Stdout = logFile
	proc.Stderr = logFile
	proc.Start()
	time.Sleep(8 * time.Second)
	proxyURL, _ := url.Parse("http://127.0.0.1:7897")
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 8 * time.Second}
	resp, err := client.Get("http://www.qq.com/")
	code := -1
	if err == nil {
		code = resp.StatusCode
		resp.Body.Close()
	}
	proc.Process.Kill()
	logFile.Close()
	fmt.Printf("%s: http=%d\n", name, code)
}

func main() {
	base := loadBase()
	// A: no strategy
	a := loadBase()
	dnsA := a["dns"].(map[string]any)
	delete(dnsA, "strategy")
	test(a, "nostrategy")
	// B: single udp server
	b := loadBase()
	dnsB := map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5", "detour": "DIRECT"}},
		"final":   "remote",
	}
	b["dns"] = dnsB
	test(b, "singleremote")
	// C: single + strategy
	c := loadBase()
	dnsC := map[string]any{
		"servers":  []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5", "detour": "DIRECT"}},
		"final":    "remote",
		"strategy": "ipv4_only",
	}
	c["dns"] = dnsC
	test(c, "singlestrategy")
	// D: original servers, final remote-1, no strategy
	d := loadBase()
	dnsD := d["dns"].(map[string]any)
	delete(dnsD, "strategy")
	dnsD["final"] = "dns-remote-1"
	test(d, "allserversnostrat")
	fmt.Println("done", len(base))
}
