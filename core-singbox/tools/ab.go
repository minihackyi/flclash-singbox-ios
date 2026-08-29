//go:build ignore

package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"time"
)

func run(name string) {
	proc := exec.Command("go", "run", "tools/runconfig.go", "testdata/"+name+".json")
	logFile := osCreate("run_ab_" + name + ".log")
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
	run("f-singleremote")
	run("singbox-config")
	run("f-singleremote")
	run("singbox-config")
}
