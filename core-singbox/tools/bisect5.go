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
	// T4: bare "223.5.5.5" with detour (no udp://)
	t4 := loadBase()
	t4["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "223.5.5.5", "detour": "DIRECT"}},
		"final":   "remote",
	}
	test(t4, "t4-bare-detour")

	// T5: udp:// WITHOUT detour
	t5 := loadBase()
	t5["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "udp://223.5.5.5"}},
		"final":   "remote",
	}
	test(t5, "t5-udp-nodetour")

	// T6: bare IP WITHOUT detour
	t6 := loadBase()
	t6["dns"] = map[string]any{
		"servers": []any{map[string]any{"tag": "remote", "address": "223.5.5.5"}},
		"final":   "remote",
	}
	test(t6, "t6-bare-nodetour")
}
