package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type QBittorrentAccess struct {
	BaseURL  string
	Username string
	Password string
}

type QBittorrentTorrent struct {
	Hash         string  `json:"hash"`
	Name         string  `json:"name"`
	State        string  `json:"state"`
	Progress     float64 `json:"progress"`
	Size         int64   `json:"size"`
	Downloaded   int64   `json:"downloaded"`
	Uploaded     int64   `json:"uploaded"`
	Ratio        float64 `json:"ratio"`
	SavePath     string  `json:"save_path"`
	ContentPath  string  `json:"content_path"`
	Category     string  `json:"category"`
	AddedOn      int64   `json:"added_on"`
	CompletionOn int64   `json:"completion_on"`
	DLSpeed      int64   `json:"dlspeed"`
	UPSpeed      int64   `json:"upspeed"`
}

type QBittorrentFile struct {
	Index      int     `json:"index"`
	Name       string  `json:"name"`
	Size       int64   `json:"size"`
	Progress   float64 `json:"progress"`
	Priority   int     `json:"priority"`
	IsSeed     bool    `json:"is_seed"`
	PieceRange []int   `json:"piece_range"`
}

type QBittorrentClient interface {
	Login(ctx context.Context, access QBittorrentAccess) error
	AddTorrents(ctx context.Context, access QBittorrentAccess, urls []string, savePath string) error
	Torrents(ctx context.Context, access QBittorrentAccess) ([]QBittorrentTorrent, error)
	TorrentFiles(ctx context.Context, access QBittorrentAccess, hash string) ([]QBittorrentFile, error)
	Pause(ctx context.Context, access QBittorrentAccess, hashes []string) error
	Resume(ctx context.Context, access QBittorrentAccess, hashes []string) error
	Delete(ctx context.Context, access QBittorrentAccess, hashes []string, deleteFiles bool) error
}

type HTTPQBittorrentClient struct {
	httpClient *http.Client
}

func NewHTTPQBittorrentClient() *HTTPQBittorrentClient {
	jar, _ := cookiejar.New(nil)
	return &HTTPQBittorrentClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
	}
}

func (c *HTTPQBittorrentClient) base(access QBittorrentAccess) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(access.BaseURL), "/")
	if baseURL == "" {
		return "", errors.New("qbittorrent base_url is not configured")
	}
	return baseURL, nil
}

func (c *HTTPQBittorrentClient) Login(ctx context.Context, access QBittorrentAccess) error {
	baseURL, err := c.base(access)
	if err != nil {
		return err
	}
	if strings.TrimSpace(access.Username) == "" {
		return errors.New("qbittorrent username is not configured")
	}

	form := url.Values{}
	form.Set("username", access.Username)
	form.Set("password", access.Password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qbittorrent login failed: http %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if !strings.EqualFold(strings.TrimSpace(string(body)), "Ok.") {
		return errors.New("qbittorrent login failed: " + strings.TrimSpace(string(body)))
	}
	return nil
}

// ensureLoggedIn performs a login so the session cookie (SID) is fresh.
// qBittorrent invalidates sessions after idle, so login before every batch.
func (c *HTTPQBittorrentClient) ensureLoggedIn(ctx context.Context, access QBittorrentAccess) error {
	return c.Login(ctx, access)
}

func (c *HTTPQBittorrentClient) AddTorrents(ctx context.Context, access QBittorrentAccess, urls []string, savePath string) error {
	baseURL, err := c.base(access)
	if err != nil {
		return err
	}
	if err := c.ensureLoggedIn(ctx, access); err != nil {
		return err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if err := writer.WriteField("urls", raw); err != nil {
			return err
		}
	}
	if strings.TrimSpace(savePath) != "" {
		if err := writer.WriteField("savepath", savePath); err != nil {
			return err
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v2/torrents/add", &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent add torrent failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qbittorrent add torrent failed: http %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}
	// API returns "Ok." on success, or "Fails." when the URL is invalid.
	if strings.Contains(strings.ToLower(string(bodyBytes)), "fails") {
		return fmt.Errorf("qbittorrent rejected torrent url(s): %s", strings.TrimSpace(string(bodyBytes)))
	}
	return nil
}

func (c *HTTPQBittorrentClient) Torrents(ctx context.Context, access QBittorrentAccess) ([]QBittorrentTorrent, error) {
	baseURL, err := c.base(access)
	if err != nil {
		return nil, err
	}
	if err := c.ensureLoggedIn(ctx, access); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v2/torrents/info", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent list torrents failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qbittorrent list torrents failed: http %d", resp.StatusCode)
	}
	var torrents []QBittorrentTorrent
	if err := json.NewDecoder(resp.Body).Decode(&torrents); err != nil {
		return nil, fmt.Errorf("qbittorrent list torrents decode failed: %w", err)
	}
	return torrents, nil
}

func (c *HTTPQBittorrentClient) TorrentFiles(ctx context.Context, access QBittorrentAccess, hash string) ([]QBittorrentFile, error) {
	baseURL, err := c.base(access)
	if err != nil {
		return nil, err
	}
	if err := c.ensureLoggedIn(ctx, access); err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("hash", strings.TrimSpace(hash))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v2/torrents/files", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("qbittorrent list files failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qbittorrent list files failed: http %d", resp.StatusCode)
	}
	var files []QBittorrentFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("qbittorrent list files decode failed: %w", err)
	}
	return files, nil
}

func (c *HTTPQBittorrentClient) Pause(ctx context.Context, access QBittorrentAccess, hashes []string) error {
	return c.torrentsAction(ctx, access, "pause", hashes)
}

func (c *HTTPQBittorrentClient) Resume(ctx context.Context, access QBittorrentAccess, hashes []string) error {
	return c.torrentsAction(ctx, access, "resume", hashes)
}

func (c *HTTPQBittorrentClient) Delete(ctx context.Context, access QBittorrentAccess, hashes []string, deleteFiles bool) error {
	baseURL, err := c.base(access)
	if err != nil {
		return err
	}
	if err := c.ensureLoggedIn(ctx, access); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("hashes", strings.Join(hashes, "|"))
	if deleteFiles {
		form.Set("deleteFiles", "true")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v2/torrents/delete", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent delete failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qbittorrent delete failed: http %d", resp.StatusCode)
	}
	return nil
}

func (c *HTTPQBittorrentClient) torrentsAction(ctx context.Context, access QBittorrentAccess, action string, hashes []string) error {
	baseURL, err := c.base(access)
	if err != nil {
		return err
	}
	if err := c.ensureLoggedIn(ctx, access); err != nil {
		return err
	}

	form := url.Values{}
	form.Set("hashes", strings.Join(hashes, "|"))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/v2/torrents/"+action, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("qbittorrent %s failed: %w", action, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qbittorrent %s failed: http %d", action, resp.StatusCode)
	}
	return nil
}
