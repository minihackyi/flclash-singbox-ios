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

func main() {
	x := loadBase()
	x["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5", "detour": "DIRECT"}},
		"final":   "remote",
	}
	data, _ := json.MarshalIndent(x, "", "  ")
	os.WriteFile("testdata/x5-singlestrat-free.json", data, 0o644)
	proc := exec.Command("go", "run", "tools/runconfig.go", "testdata/x5-singlestrat-free.json")
	logFile, _ := os.Create("run_x5.log")
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
	fmt.Printf("x5-no-strategy-single-udp: http=%d\n", code)
}
