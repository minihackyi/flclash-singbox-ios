//go:build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	proxy := "http://127.0.0.1:7897"
	if len(os.Args) > 1 {
		proxy = os.Args[1]
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(mustParse(proxy))},
		Timeout:   10 * time.Second,
	}
	resp, err := client.Get("http://www.qq.com/")
	if err != nil {
		fmt.Println("[FAIL] request through core:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
	fmt.Printf("[OK] through-core status=%d len=%s\n", resp.StatusCode, fmt.Sprint(len(body)))
}
