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
	name := "f-singleremote"
	proc := exec.Command("go", "run", "tools/runconfig.go", "testdata/"+name+".json")
	logFile := osCreate("run_" + name + "_again.log")
	proc.Stdout = logFile
	proc.Stderr = logFile
	proc.Start()
	time.Sleep(8 * time.Second)
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
	fmt.Printf("%s again: http=%d %s\n", name, code, msg)
}
