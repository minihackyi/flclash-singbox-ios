package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Minimal clash API client: REST + the /traffic and /logs websockets
// (implemented over chunked streaming HTTP, which the clash api also accepts
// via its ws wrapper -- in practice we use the HTTP upgrade endpoint through
// a plain client since golang.org/x/net/websocket is vendored by sing).

var apiHTTPClient = &http.Client{
	Timeout: 15 * time.Second,
}

func callClashApi(method string, rawURL string, token string, body any) (int, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = strings.NewReader(string(data))
	}
	request, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := apiHTTPClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode, nil
}

func fetchClashApiRaw(rawURL string, token string) string {
	request, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return ""
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := apiHTTPClient.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode/100 != 2 {
		return ""
	}
	return string(data)
}

func fetchClashApiMap(rawURL string, token string) map[string]any {
	raw := fetchClashApiRaw(rawURL, token)
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func urlPathEscape(value string) string {
	return url.PathEscape(value)
}

func urlQueryEscape(value string) string {
	return url.QueryEscape(value)
}

// pumpStreams connects the /traffic and /logs streaming endpoints and
// forwards data as IPC messages.
func pumpTraffic(ctx context.Context, apiBase string, token string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streamClashApi(ctx, apiBase+"/traffic", token, func(line []byte) {
			var traffic map[string]int64
			if err := json.Unmarshal(line, &traffic); err != nil {
				return
			}
			up, _ := traffic["up"]
			down, _ := traffic["down"]
			engine.mu.Lock()
			engine.totalUp += up
			engine.totalDown += down
			engine.mu.Unlock()
			// The UI polls getTraffic at 1s; lastUp/lastDown are refreshed by
			// the sampler below. Store instantaneous speed here anyway.
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// startTrafficSampler converts the cumulative clash /traffic stream into
// 1-second speed samples for getTraffic.
func startTrafficSampler(ctx context.Context) {
	var prevUp, prevDown int64
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			engine.mu.Lock()
			// totalUp/totalDown were updated by the pump; approximate speed as
			// delta over the tick.
			up := engine.totalUp - prevUp
			down := engine.totalDown - prevDown
			if up < 0 {
				up = 0
			}
			if down < 0 {
				down = 0
			}
			engine.lastUp = up
			engine.lastDown = down
			prevUp = engine.totalUp
			prevDown = engine.totalDown
			engine.mu.Unlock()
		}
	}
}

var _ = bufio.NewReader
var _ = fmt.Sprintf
