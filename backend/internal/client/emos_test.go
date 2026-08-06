package client

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetUploadTokenParsesFlexibleData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		response    string
		wantType    string
		wantFileID  string
		wantURL     string
		wantMinSize int64
		wantMaxSize int64
	}{
		{
			name:       "data as string",
			response:   `{"type":"r2","file_id":"file-r2","data":"https://upload.example/put"}`,
			wantType:   "r2",
			wantFileID: "file-r2",
			wantURL:    "https://upload.example/put",
		},
		{
			name:        "data object with multipart size bounds",
			response:    `{"type":"multipart","file_id":"file-mp","data":{"multipart_size":{"min":5242880,"max":1073741824}}}`,
			wantType:    "multipart",
			wantFileID:  "file-mp",
			wantMinSize: 5242880,
			wantMaxSize: 1073741824,
		},
		{
			name:        "multipart size as string",
			response:    `{"type":"multipart","file_id":"file-mp","data":{"multipart_size":"1048576"}}`,
			wantType:    "multipart",
			wantFileID:  "file-mp",
			wantMinSize: 1048576,
			wantMaxSize: 1048576,
		},
		{
			name:       "raw upload url",
			response:   `{"type":"r2","file_id":"file-r2","upload_url":"https://upload.example/raw","data":{}}`,
			wantType:   "r2",
			wantFileID: "file-r2",
			wantURL:    "https://upload.example/raw",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/upload/getUploadToken" {
					t.Errorf("unexpected path %s", r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer demo-token" {
					t.Errorf("unexpected authorization header %q", r.Header.Get("Authorization"))
				}
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			result, err := NewHTTPEmosClient().GetUploadToken(context.Background(), EmosAccess{
				BaseURL: server.URL,
				Token:   "demo-token",
			}, EmosUploadTokenRequest{
				FileStorage: "zn_r2_upload",
			})
			if err != nil {
				t.Fatalf("get upload token: %v", err)
			}
			if result.UploadType != tt.wantType {
				t.Fatalf("expected upload type %q, got %q", tt.wantType, result.UploadType)
			}
			if result.FileID != tt.wantFileID {
				t.Fatalf("expected file id %q, got %q", tt.wantFileID, result.FileID)
			}
			if result.UploadURL != tt.wantURL {
				t.Fatalf("expected upload url %q, got %q", tt.wantURL, result.UploadURL)
			}
			if result.MultipartSizeMin != tt.wantMinSize || result.MultipartSizeMax != tt.wantMaxSize {
				t.Fatalf("expected multipart size %d..%d, got %d..%d", tt.wantMinSize, tt.wantMaxSize, result.MultipartSizeMin, result.MultipartSizeMax)
			}
		})
	}
}

func TestUploadFileSendsExpectedHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		uploadType  string
		statusCode  int
		wantRanges  []string
		wantNoRange bool
	}{
		{
			name:       "onedrive",
			uploadType: "onedrive",
			statusCode: http.StatusAccepted,
			wantRanges: []string{
				"bytes 0-3/10",
				"bytes 4-7/10",
				"bytes 8-9/10",
			},
		},
		{
			name:        "r2",
			uploadType:  "r2",
			statusCode:  http.StatusNoContent,
			wantNoRange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			var ranges []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read upload body: %v", err)
				}
				if len(body) == 0 {
					t.Errorf("upload body is empty")
				}
				ranges = append(ranges, r.Header.Get("Content-Range"))
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			filePath := filepath.Join(t.TempDir(), "demo.mkv")
			if err := os.WriteFile(filePath, []byte("0123456789"), 0o644); err != nil {
				t.Fatalf("write upload file: %v", err)
			}

			err := NewHTTPEmosClient().UploadFile(
				context.Background(),
				tt.uploadType,
				server.URL,
				filePath,
				4,
				0,
				nil,
			)
			if err != nil {
				t.Fatalf("upload file: %v", err)
			}
			if calls != 3 {
				t.Fatalf("expected 3 upload calls, got %d", calls)
			}
			if tt.wantNoRange {
				for _, value := range ranges {
					if value != "" {
						t.Fatalf("expected no Content-Range, got %q", value)
					}
				}
			} else {
				for i, want := range tt.wantRanges {
					if ranges[i] != want {
						t.Fatalf("expected range %q, got %q", want, ranges[i])
					}
				}
			}
		})
	}
}

func TestUploadFileRetriesTemporaryFailure(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "demo.mkv")
	if err := os.WriteFile(filePath, []byte("abc"), 0o644); err != nil {
		t.Fatalf("write upload file: %v", err)
	}

	err := NewHTTPEmosClient().UploadFile(
		context.Background(),
		"r2",
		server.URL,
		filePath,
		1024,
		0,
		nil,
	)
	if err != nil {
		t.Fatalf("upload file: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("expected 2 calls after retry, got %d", calls.Load())
	}
}

func TestUploadMultipartPartReturnsUnquotedETag(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Range") != "" {
			t.Errorf("multipart part should not include Content-Range")
		}
		w.Header().Set("ETag", `"etag-123"`)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "demo.mkv")
	if err := os.WriteFile(filePath, []byte("part-data"), 0o644); err != nil {
		t.Fatalf("write upload file: %v", err)
	}

	etag, err := NewHTTPEmosClient().UploadMultipartPart(context.Background(), EmosMultipartPart{
		Number:    1,
		UploadURL: server.URL,
	}, filePath, 0, 9)
	if err != nil {
		t.Fatalf("upload multipart part: %v", err)
	}
	if etag != "etag-123" {
		t.Fatalf("expected unquoted etag, got %q", etag)
	}
}

// TestUploadStallAbortsFast simulates the mainland-VPS → R2 scenario: the
// server accepts the connection but never responds. The upload must fail with
// a clear "stalled" error within the (shrunk) stall timeout instead of
// hanging for the whole client timeout.
func TestUploadStallAbortsFast(t *testing.T) {
	oldTimeout := uploadStallTimeout
	uploadStallTimeout = 500 * time.Millisecond
	t.Cleanup(func() { uploadStallTimeout = oldTimeout })

	// A raw http.Server (not httptest) so Close() force-kills the stalled
	// connections without waiting for the never-responding handler.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})}
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })

	filePath := filepath.Join(t.TempDir(), "stall.mkv")
	if err := os.WriteFile(filePath, make([]byte, 8<<20), 0o644); err != nil {
		t.Fatalf("write upload file: %v", err)
	}

	start := time.Now()
	_, err = NewHTTPEmosClient().UploadMultipartPart(context.Background(), EmosMultipartPart{
		Number:    1,
		UploadURL: "http://" + ln.Addr().String() + "/part",
	}, filePath, 0, 8<<20)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected stalled upload to return an error")
	}
	if !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("expected 'stalled' error, got: %v", err)
	}
	if elapsed > 10*time.Second {
		t.Fatalf("stalled upload took %s, expected fast failure", elapsed)
	}
}

func TestCompleteMultipartOmitsUploadURL(t *testing.T) {
	t.Parallel()

	var payload struct {
		Parts []map[string]any `json:"parts"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer demo-token" {
			t.Errorf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode complete payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := NewHTTPEmosClient().CompleteMultipart(context.Background(), EmosAccess{
		BaseURL: server.URL,
		Token:   "demo-token",
	}, "file-mp", []EmosMultipartPart{
		{Number: 1, UploadURL: "https://should-not-be-sent.example", ETag: "etag-1"},
		{Number: 2, ETag: "etag-2"},
	})
	if err != nil {
		t.Fatalf("complete multipart: %v", err)
	}
	if len(payload.Parts) != 2 {
		t.Fatalf("expected 2 complete parts, got %d", len(payload.Parts))
	}
	for _, part := range payload.Parts {
		if _, ok := part["upload_url"]; ok {
			t.Fatalf("complete part should not include upload_url: %#v", part)
		}
	}
	if payload.Parts[0]["number"] != float64(1) || payload.Parts[0]["etag"] != "etag-1" {
		t.Fatalf("unexpected complete part payload: %#v", payload.Parts[0])
	}
}
