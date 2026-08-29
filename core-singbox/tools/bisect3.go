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

func test(cfg map[string]any, name string) int {
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
	return code
}

func main() {

	// 1. + sniff
	s := load("manual7897")
	s["inbounds"].([]any)[0].(map[string]any)["sniff"] = true
	test(s, "addsniff")

	// 2. + sniff_override_destination false
	s2 := load("addsniff")
	s2["inbounds"].([]any)[0].(map[string]any)["sniff_override_destination"] = false
	test(s2, "addsniff2")

	// 3. + route rules (domain_suffix qq DIRECT + full original rules)
	r := load("manual7897")
	r["route"].(map[string]any)["rules"] = []any{
		map[string]any{"protocol": "dns", "outbound": "dns-out"},
		map[string]any{"domain_suffix": []string{"qq.com"}, "outbound": "DIRECT"},
		map[string]any{"ip_cidr": []string{"127.0.0.0/8"}, "outbound": "DIRECT"},
		map[string]any{"ip_cidr": []string{"many..."}, "outbound": "DIRECT"},
	}
	r["outbounds"] = append(r["outbounds"].([]any), map[string]any{"type": "dns", "tag": "dns-out"})
	test(r, "addrules")

	// 4. + experimental
	e := load("manual7897")
	e["experimental"] = map[string]any{"cache_file": map[string]any{"enabled": false}}
	test(e, "addexp")
}
