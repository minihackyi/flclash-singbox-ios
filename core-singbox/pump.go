package main

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// pumpStreams connects the clash api streaming endpoints (logs/connections)
// and forwards data as IPC messages.
func pumpLogs(ctx context.Context, apiBase string, token string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streamClashApi(ctx, apiBase+"/logs?level=debug", token, func(line []byte) {
			var entry struct {
				Type    string `json:"type"`
				Payload string `json:"payload"`
			}
			if err := json.Unmarshal(line, &entry); err != nil {
				return
			}
			message := &Message{
				Type: LogMessage,
				Data: Log{
					LogLevel: normalizeLogType(entry.Type),
					Payload:  entry.Payload,
				},
			}
			sendMessage(*message)
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func normalizeLogType(logType string) string {
	switch strings.ToLower(logType) {
	case "warning":
		return "warning"
	case "error":
		return "error"
	case "debug":
		return "debug"
	default:
		return "info"
	}
}

func pumpConnections(ctx context.Context, apiBase string, token string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		streamClashApi(ctx, apiBase+"/connections", token, func(line []byte) {
			var snapshot struct {
				Connections []map[string]any `json:"connections"`
			}
			if err := json.Unmarshal(line, &snapshot); err != nil {
				return
			}
			engine.connMu.Lock()
			alive := map[string]struct{}{}
			for _, conn := range snapshot.Connections {
				id, _ := conn["id"].(string)
				if id == "" {
					continue
				}
				alive[id] = struct{}{}
				if _, known := engine.knownConns[id]; !known {
					engine.knownConns[id] = struct{}{}
					// New connection: push a request event like mihomo did.
					data, _ := json.Marshal(conn)
					go sendMessage(Message{
						Type: RequestMessage,
						Data: json.RawMessage(data),
					})
				}
			}
			engine.knownConns = alive
			engine.connMu.Unlock()
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// streamClashApi performs a long-lived GET and calls fn per newline-delimited
// JSON chunk (the clash api streams newline-delimited JSON over its ws and
// http endpoints; with Go's http client the streaming body works directly).
func streamClashApi(ctx context.Context, rawURL string, token string, fn func(line []byte)) {
	request, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	// The clash api serves /logs and /traffic and /connections on ws only in
	// some versions; sending an upgrade attempt falls back to plain GET.
	response, err := apiHTTPClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.StatusCode/100 != 2 {
		return
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = response.Body.Close()
		case <-done:
		}
	}()
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		fn(line)
	}
	close(done)
}

var _ = time.Second
