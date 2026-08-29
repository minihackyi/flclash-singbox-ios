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
	full := load("singbox-config")
	full["log"].(map[string]any)["level"] = "debug"

	// 1. full with single-remote dns (baseline fail)
	sr := load("singbox-config")
	sr["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5", "detour": "DIRECT"}},
		"final":   "remote",
	}
	test(sr, "f-singleremote")

	// 2. f-singleremote with only DIRECT/REJECT/dns-out/plain outbounds (drop groups+GLOBAL)
	ng := load("f-singleremote")
	outbs := ng["outbounds"].([]any)
	keep := []any{}
	for _, ob := range outbs {
		t := ob.(map[string]any)["type"]
		if t == "selector" || t == "urltest" {
			continue
		}
		keep = append(keep, ob)
	}
	ng["outbounds"] = keep
	test(ng, "f-nogroups")

	// 3. f-singleremute with rules stripped to minimal
	nr := load("f-singleremote")
	nr["route"].(map[string]any)["rules"] = []any{map[string]any{"protocol": "dns", "outbound": "dns-out"}}
	nr["route"].(map[string]any)["final"] = "DIRECT"
	test(nr, "f-norules")
}
