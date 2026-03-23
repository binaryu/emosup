package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPAria2ClientRPC(t *testing.T) {
	t.Parallel()

	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		method := request["method"].(string)
		methods = append(methods, method)

		switch method {
		case "aria2.addUri":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result":  "gid-123",
			})
		case "aria2.tellStatus":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result": map[string]any{
					"gid":             "gid-123",
					"status":          "active",
					"totalLength":     "1024",
					"completedLength": "256",
					"downloadSpeed":   "128",
					"files":           []map[string]any{{"path": "/tmp/demo.mkv"}},
				},
			})
		case "aria2.forceRemove":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      request["id"],
				"result":  "OK",
			})
		default:
			http.Error(w, "unexpected method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	client := NewHTTPAria2Client()
	access := Aria2Access{
		RPCURL:         server.URL,
		Secret:         "demo",
		ConnectTimeout: 5 * time.Second,
	}

	gid, err := client.AddURI(context.Background(), access, "https://example.com/raw/demo", Aria2AddURIOptions{
		Dir:              "./downloads",
		Out:              "demo.mkv",
		ContinueDownload: true,
	})
	if err != nil {
		t.Fatalf("add uri: %v", err)
	}
	if gid != "gid-123" {
		t.Fatalf("expected gid-123, got %s", gid)
	}

	status, err := client.TellStatus(context.Background(), access, gid)
	if err != nil {
		t.Fatalf("tell status: %v", err)
	}
	if status.Status != "active" || status.TotalLength != 1024 || status.CompletedLength != 256 {
		t.Fatalf("unexpected tell status result: %#v", status)
	}

	if err := client.ForceRemove(context.Background(), access, gid); err != nil {
		t.Fatalf("force remove: %v", err)
	}

	if len(methods) != 3 {
		t.Fatalf("expected 3 rpc methods, got %d", len(methods))
	}
}
