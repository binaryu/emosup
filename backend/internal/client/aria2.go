package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Aria2Access struct {
	RPCURL         string
	Secret         string
	ConnectTimeout time.Duration
}

type Aria2AddURIOptions struct {
	Dir              string
	Out              string
	ContinueDownload bool
	UserAgent        string
	Header           []string
}

type Aria2Status struct {
	GID             string      `json:"gid"`
	Status          string      `json:"status"`
	TotalLength     int64       `json:"-"`
	CompletedLength int64       `json:"-"`
	DownloadSpeed   int64       `json:"-"`
	Files           []Aria2File `json:"files"`
	ErrorCode       string      `json:"errorCode"`
	ErrorMessage    string      `json:"errorMessage"`
}

type Aria2File struct {
	Path string `json:"path"`
}

type Aria2Client interface {
	AddURI(ctx context.Context, access Aria2Access, directURL string, options Aria2AddURIOptions) (string, error)
	TellStatus(ctx context.Context, access Aria2Access, gid string) (Aria2Status, error)
	ForceRemove(ctx context.Context, access Aria2Access, gid string) error
}

type HTTPAria2Client struct {
	httpClient *http.Client
}

func NewHTTPAria2Client() *HTTPAria2Client {
	return &HTTPAria2Client{
		httpClient: &http.Client{},
	}
}

func (c *HTTPAria2Client) AddURI(ctx context.Context, access Aria2Access, directURL string, options Aria2AddURIOptions) (string, error) {
	if strings.TrimSpace(directURL) == "" {
		return "", errors.New("aria2 direct url is required")
	}

	params := []any{
		[]string{directURL},
		map[string]string{
			"dir":                options.Dir,
			"out":                options.Out,
			"continue":           strconv.FormatBool(options.ContinueDownload),
			"user-agent":         options.UserAgent,
			"allow-overwrite":    "true",
			"auto-file-renaming": "false",
		},
	}

	optionMap := params[1].(map[string]string)
	if len(options.Header) > 0 {
		optionMap["header"] = strings.Join(options.Header, "\r\n")
	}

	var gid string
	if err := c.rpc(ctx, access, "aria2.addUri", params, &gid); err != nil {
		return "", err
	}

	return gid, nil
}

func (c *HTTPAria2Client) TellStatus(ctx context.Context, access Aria2Access, gid string) (Aria2Status, error) {
	if strings.TrimSpace(gid) == "" {
		return Aria2Status{}, errors.New("aria2 gid is required")
	}

	var raw struct {
		GID             string      `json:"gid"`
		Status          string      `json:"status"`
		TotalLength     string      `json:"totalLength"`
		CompletedLength string      `json:"completedLength"`
		DownloadSpeed   string      `json:"downloadSpeed"`
		Files           []Aria2File `json:"files"`
		ErrorCode       string      `json:"errorCode"`
		ErrorMessage    string      `json:"errorMessage"`
	}
	if err := c.rpc(ctx, access, "aria2.tellStatus", []any{gid}, &raw); err != nil {
		return Aria2Status{}, err
	}

	return Aria2Status{
		GID:             raw.GID,
		Status:          raw.Status,
		TotalLength:     parseAria2Int64(raw.TotalLength),
		CompletedLength: parseAria2Int64(raw.CompletedLength),
		DownloadSpeed:   parseAria2Int64(raw.DownloadSpeed),
		Files:           raw.Files,
		ErrorCode:       raw.ErrorCode,
		ErrorMessage:    raw.ErrorMessage,
	}, nil
}

func (c *HTTPAria2Client) ForceRemove(ctx context.Context, access Aria2Access, gid string) error {
	if strings.TrimSpace(gid) == "" {
		return errors.New("aria2 gid is required")
	}

	var result string
	return c.rpc(ctx, access, "aria2.forceRemove", []any{gid}, &result)
}

type aria2Request struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type aria2Response struct {
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *HTTPAria2Client) rpc(ctx context.Context, access Aria2Access, method string, params []any, target any) error {
	if strings.TrimSpace(access.RPCURL) == "" {
		return errors.New("aria2 rpc_url is required")
	}

	if access.ConnectTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, access.ConnectTimeout)
		defer cancel()
	}

	if secret := strings.TrimSpace(access.Secret); secret != "" {
		params = append([]any{"token:" + secret}, params...)
	}

	payload, err := json.Marshal(aria2Request{
		JSONRPC: "2.0",
		ID:      strconv.FormatInt(time.Now().UnixNano(), 10),
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, access.RPCURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("aria2 rpc request failed: %s", response.Status)
	}

	var rpcResponse aria2Response
	if err := json.NewDecoder(response.Body).Decode(&rpcResponse); err != nil {
		return err
	}
	if rpcResponse.Error != nil {
		return fmt.Errorf("aria2 rpc error %d: %s", rpcResponse.Error.Code, rpcResponse.Error.Message)
	}
	if target == nil || len(rpcResponse.Result) == 0 {
		return nil
	}

	return json.Unmarshal(rpcResponse.Result, target)
}

func parseAria2Int64(raw string) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return value
}
