package handler

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"emosup/backend/internal/client"
	"emosup/backend/internal/store"
)

type ProxyHandler struct {
	store  *store.FileStore
	client client.OpenListClient
}

func NewProxyHandler(store *store.FileStore, olClient client.OpenListClient) *ProxyHandler {
	return &ProxyHandler{store: store, client: olClient}
}

func (h *ProxyHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.GET("/proxy/download", h.download)
}

func (h *ProxyHandler) download(c *gin.Context) {
	openlistPath := c.Query("path")
	if openlistPath == "" {
		c.String(http.StatusBadRequest, "path is required")
		return
	}

	cfg, err := h.store.LoadConfig()
	if err != nil {
		c.String(http.StatusInternalServerError, "config error")
		return
	}

	access := client.OpenListAccess{
		BaseURL:  cfg.OpenList.BaseURL,
		Username: cfg.OpenList.Username,
		Password: cfg.OpenList.Password,
		Token:    cfg.OpenList.Token,
	}

	// Auto-login if needed
	if access.Token == "" && access.Username != "" && access.Password != "" {
		token, loginErr := h.client.Login(c.Request.Context(), access)
		if loginErr != nil {
			log.Printf("proxy login failed: %v", loginErr)
		} else {
			access.Token = token
			log.Printf("proxy login ok")
		}
	}

	// Step 1: Get the raw download URL from OpenList
	rawURL, err := h.client.GetRawLink(c.Request.Context(), access, openlistPath)
	if err != nil {
		log.Printf("proxy get raw link failed for %s: %v", openlistPath, err)
		c.String(http.StatusInternalServerError, "failed to get raw link: "+err.Error())
		return
	}
	rawURL = client.ResolveMaybeRelativeURL(cfg.OpenList.BaseURL, rawURL)
	log.Printf("proxy raw url: %s", rawURL)

	// Step 2: Proxy the download with proper headers
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		c.String(http.StatusInternalServerError, "proxy request failed")
		return
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", strings.TrimRight(cfg.OpenList.BaseURL, "/")+"/")
	if access.Token != "" {
		req.Header.Set("Authorization", access.Token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("proxy download failed for %s: %v", rawURL, err)
		c.String(http.StatusBadGateway, "proxy download failed: "+err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		c.String(resp.StatusCode, fmt.Sprintf("upstream error %d: %s", resp.StatusCode, string(body)))
		return
	}

	// Stream the response to the client
	fileName := extractFileName(openlistPath, resp)
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Content-Length", resp.Header.Get("Content-Length"))
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func extractFileName(path string, resp *http.Response) string {
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		for _, part := range strings.Split(cd, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				return strings.Trim(part[9:], `"`)
			}
		}
	}
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	return parts[len(parts)-1]
}
