package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"
)

type Aria2Service struct {
	client *http.Client
}

type Aria2Progress struct {
	Percent float64
	Speed   string
	ETA     string
	Done    bool
}

func NewAria2Service() *Aria2Service {
	return &Aria2Service{client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *Aria2Service) CheckVersion(rpcURL, secret string) error {
	res, err := s.rpcCall(rpcURL, secret, "aria2.getVersion", []any{})
	if err != nil {
		return err
	}
	if _, ok := res["result"]; !ok {
		return fmt.Errorf("aria2 connection failed")
	}
	return nil
}

func (s *Aria2Service) DownloadAndMonitor(ctx context.Context, rpcURL, secret, url, dstPath string, threads int, onProgress func(Aria2Progress)) error {
	options := map[string]string{
		"dir":                       filepath.Dir(dstPath),
		"out":                       filepath.Base(dstPath),
		"allow-overwrite":           "true",
		"auto-file-renaming":        "false",
		"split":                     fmt.Sprintf("%d", threads),
		"max-connection-per-server": fmt.Sprintf("%d", threads),
	}
	res, err := s.rpcCall(rpcURL, secret, "aria2.addUri", []any{[]string{url}, options})
	if err != nil {
		return err
	}
	gid, _ := res["result"].(string)
	if gid == "" {
		return fmt.Errorf("failed to add download task to aria2")
	}

	for {
		select {
		case <-ctx.Done():
			_, _ = s.rpcCall(rpcURL, secret, "aria2.forceRemove", []any{gid})
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}

		status, err := s.rpcCall(rpcURL, secret, "aria2.tellStatus", []any{gid})
		if err != nil {
			continue
		}
		result, _ := status["result"].(map[string]any)
		if result == nil {
			continue
		}

		total := anyToInt64(result["totalLength"])
		completed := anyToInt64(result["completedLength"])
		speed := anyToInt64(result["downloadSpeed"])
		percent := 0.0
		if total > 0 {
			percent = float64(completed) / float64(total) * 100
		}
		eta := "N/A"
		if speed > 0 {
			remain := (total - completed) / speed
			eta = fmt.Sprintf("%dm %ds", remain/60, remain%60)
		}
		if onProgress != nil {
			onProgress(Aria2Progress{
				Percent: percent,
				Speed:   bytesToSpeed(speed),
				ETA:     eta,
				Done:    false,
			})
		}

		state, _ := result["status"].(string)
		switch state {
		case "complete":
			_, _ = s.rpcCall(rpcURL, secret, "aria2.removeDownloadResult", []any{gid})
			if onProgress != nil {
				onProgress(Aria2Progress{Percent: 100, Speed: "done", ETA: "N/A", Done: true})
			}
			return nil
		case "error", "removed":
			_, _ = s.rpcCall(rpcURL, secret, "aria2.forceRemove", []any{gid})
			return fmt.Errorf("aria2 download failed")
		}
	}
}

func (s *Aria2Service) rpcCall(rpcURL, secret, method string, params []any) (map[string]any, error) {
	rpcParams := append([]any{}, params...)
	if secret != "" {
		rpcParams = append([]any{"token:" + secret}, rpcParams...)
	}
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "goserver",
		"method":  method,
		"params":  rpcParams,
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func anyToInt64(v any) int64 {
	switch x := v.(type) {
	case string:
		var out int64
		fmt.Sscan(x, &out)
		return out
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	default:
		return 0
	}
}

func bytesToSpeed(v int64) string {
	mb := float64(v) / 1024.0 / 1024.0
	if mb >= 1 {
		return fmt.Sprintf("%.2f MB/s", mb)
	}
	kb := float64(v) / 1024.0
	return fmt.Sprintf("%.2f KB/s", kb)
}
