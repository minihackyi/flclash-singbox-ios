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

func save(cfg map[string]any, name string) {
	data, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile("testdata/"+name+".json", data, 0o644)
}

func test(cfg map[string]any, name string) {
	save(cfg, name)
	proc := exec.Command("go", "run", "tools/runconfig.go", "testdata/"+name+".json")
	logFile := mustCreate("run_" + name + ".log")
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

func mustCreate(name string) *os.File {
	f, _ := os.Create(name)
	return f
}

func main() {
	// x1: no strategy
	x1 := loadBase()
	x1["dns"].(map[string]any)["strategy"] = nil
	test(x1, "x1-nostrat")
	// x2: remove doh (keep udp x3 + local)
	x2 := loadBase()
	servers := x2["dns"].(map[string]any)["servers"].([]any)
	kept := []any{}
	for _, s := range servers {
		if s.(map[string]any)["address"] == "https://doh.pub/dns-query" {
			continue
		}
		kept = append(kept, s)
	}
	x2["dns"].(map[string]any)["servers"] = kept
	test(x2, "x2-nodoh")
	// x3: remove local
	x3 := loadBase()
	servers3 := x3["dns"].(map[string]any)["servers"].([]any)
	kept3 := []any{}
	for _, s := range servers3 {
		if s.(map[string]any)["address"] == "local" {
			continue
		}
		kept3 = append(kept3, s)
	}
	x3["dns"].(map[string]any)["servers"] = kept3
	test(x3, "x3-nolocal")
	// x4: single udp + final remote
	x4 := loadBase()
	x4["dns"] = map[string]any{
		"servers":  []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5", "detour": "DIRECT"}},
		"final":    "remote",
		"strategy": "ipv4_only",
	}
	test(x4, "x4-singlestrat")
}
