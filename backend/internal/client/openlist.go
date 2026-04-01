package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OpenListAccess struct {
	BaseURL string
	Token   string
}

type OpenListEntry struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	IsDir      bool   `json:"is_dir"`
	Size       int64  `json:"size"`
	ModifiedAt string `json:"modified_at"`
}

type OpenListClient interface {
	List(ctx context.Context, access OpenListAccess, path string) ([]OpenListEntry, error)
	GetRawLink(ctx context.Context, access OpenListAccess, path string) (string, error)
}

type HTTPOpenListClient struct {
	httpClient *http.Client
}

func NewHTTPOpenListClient() *HTTPOpenListClient {
	return &HTTPOpenListClient{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *HTTPOpenListClient) List(ctx context.Context, access OpenListAccess, path string) ([]OpenListEntry, error) {
	var responseData struct {
		Content []struct {
			Name       string `json:"name"`
			IsDir      bool   `json:"is_dir"`
			Size       int64  `json:"size"`
			Modified   string `json:"modified"`
			ModifiedAt string `json:"modified_at"`
		} `json:"content"`
	}

	if err := c.post(ctx, access, "/api/fs/list", map[string]any{
		"path":     path,
		"password": "",
		"page":     1,
		"per_page": 0,
		"refresh":  false,
	}, &responseData); err != nil {
		return nil, err
	}

	entries := make([]OpenListEntry, 0, len(responseData.Content))
	for _, item := range responseData.Content {
		modifiedAt := item.ModifiedAt
		if modifiedAt == "" {
			modifiedAt = item.Modified
		}

		entries = append(entries, OpenListEntry{
			Name:       item.Name,
			Path:       joinOpenListPath(path, item.Name),
			IsDir:      item.IsDir,
			Size:       item.Size,
			ModifiedAt: modifiedAt,
		})
	}

	return entries, nil
}

func (c *HTTPOpenListClient) GetRawLink(ctx context.Context, access OpenListAccess, path string) (string, error) {
	var responseData struct {
		RawURL string `json:"raw_url"`
		Link   string `json:"link"`
		URL    string `json:"url"`
	}

	if err := c.post(ctx, access, "/api/fs/get", map[string]any{
		"path":     path,
		"password": "",
	}, &responseData); err != nil {
		return "", err
	}

	rawURL := firstNonEmpty(responseData.RawURL, responseData.Link, responseData.URL)
	if rawURL == "" {
		return "", errors.New("openlist raw url missing in response")
	}

	return rawURL, nil
}

type openListEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Msg     string          `json:"msg"`
	Data    json.RawMessage `json:"data"`
}

func (c *HTTPOpenListClient) post(ctx context.Context, access OpenListAccess, path string, payload any, target any) error {
	if strings.TrimSpace(access.BaseURL) == "" {
		return errors.New("openlist base_url is required")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(access.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(access.Token); token != "" {
		request.Header.Set("Authorization", token)
	}

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	var envelope openListEnvelope
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}

	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("openlist request failed: %s", firstNonEmpty(envelope.Message, envelope.Msg, response.Status))
	}

	if envelope.Code != 200 && envelope.Code != 0 {
		return fmt.Errorf("openlist api error: %s", firstNonEmpty(envelope.Message, envelope.Msg))
	}

	if len(envelope.Data) == 0 {
		return nil
	}

	return json.Unmarshal(envelope.Data, target)
}

func joinOpenListPath(parent, name string) string {
	parent = strings.TrimSpace(parent)
	name = strings.TrimSpace(name)
	if parent == "" || parent == "/" {
		return "/" + strings.TrimLeft(name, "/")
	}

	return strings.TrimRight(parent, "/") + "/" + strings.TrimLeft(name, "/")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}

	return ""
}

func ResolveMaybeRelativeURL(baseURL, raw string) string {
	if raw == "" {
		return raw
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.IsAbs() {
		return raw
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return raw
	}

	return base.ResolveReference(parsed).String()
}
