//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

func main() {
	for i := 0; i < 3; i++ {
		proc := exec.Command("go", "run", "tools/runconfig.go", "testdata/singbox-config.json")
		logFile := osCreate(fmt.Sprintf("run_rep%d.log", i))
		proc.Stdout = logFile
		proc.Stderr = logFile
		proc.Start()
		time.Sleep(7 * time.Second)
		proxyURL, _ := url.Parse("http://127.0.0.1:7897")
		client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 8 * time.Second}
		resp, err := client.Get("http://www.qq.com/")
		code := -1
		msg := ""
		if err == nil {
			code = resp.StatusCode
			resp.Body.Close()
		} else {
			msg = err.Error()
		}
		proc.Process.Kill()
		logFile.Close()
		fmt.Printf("run%d: http=%d %s\n", i, code, msg)
	}
}
