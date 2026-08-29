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
	// manual.json but on port 7897 — the known-good baseline.
	m := load("manual")
	m["inbounds"].([]any)[0].(map[string]any)["listen_port"] = 7897
	test(m, "manual7897")
}
