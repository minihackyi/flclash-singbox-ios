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

func load(name string) map[string]any {
	data, _ := os.ReadFile("testdata/" + name + ".json")
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
	proc := exec.Command("tools/runconfig.exe", "testdata/"+name+".json")
	logFile := osCreate("run_" + name + ".log")
	proc.Stdout = logFile
	proc.Stderr = logFile
	proc.Start()
	time.Sleep(6 * time.Second)
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
	base := load("singbox-config")
	// m1: as generated
	test(base, "m1-asgen")
	// m2: no dns.strategy
	m2 := load("m1-asgen")
	delete(m2["dns"].(map[string]any), "strategy")
	test(m2, "m2-nostrat")
	// m3: local-only dns
	m3 := load("m1-asgen")
	m3["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "local", "address": "local", "detour": "DIRECT"}},
		"final":   "local",
	}
	test(m3, "m3-localonly")
	// m4: single udp no detour no strategy
	m4 := load("m1-asgen")
	m4["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5"}},
		"final":   "remote",
	}
	test(m4, "m4-singleudp")
	// m5: re-test m1 to check order effects
	test(load("m1-asgen"), "m5-again")
}
