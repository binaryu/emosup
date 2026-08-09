package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

type QBittorrentService struct {
	store        *store.FileStore
	client       client.QBittorrentClient
	localService *LocalService
	scanService  *ScanService
}

func NewQBittorrentService(store *store.FileStore, qbClient client.QBittorrentClient, localService *LocalService, scanService *ScanService) *QBittorrentService {
	return &QBittorrentService{
		store:        store,
		client:       qbClient,
		localService: localService,
		scanService:  scanService,
	}
}

func (s *QBittorrentService) buildAccess(cfg model.AppConfig) client.QBittorrentAccess {
	return client.QBittorrentAccess{
		BaseURL:  cfg.QBittorrent.BaseURL,
		Username: cfg.QBittorrent.Username,
		Password: cfg.QBittorrent.Password,
	}
}

// defaultSavePath resolves the save path used when adding torrents:
// explicit config -> <local media root>/BT.
func (s *QBittorrentService) defaultSavePath(cfg model.AppConfig) (string, error) {
	if dir := strings.TrimSpace(cfg.QBittorrent.SavePath); dir != "" {
		abs := filepath.Clean(dir)
		return abs, utils.EnsureDir(abs)
	}
	root := s.localService.Root()
	if root == "" {
		return "", errors.New("本地媒体根目录未配置，无法确定 BT 保存路径")
	}
	dir := filepath.Join(root, "BT")
	if err := utils.EnsureDir(dir); err != nil {
		return "", fmt.Errorf("创建 BT 保存目录失败 %s: %w", dir, err)
	}
	return dir, nil
}

func (s *QBittorrentService) TestConnection(ctx context.Context) error {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return err
	}
	return s.client.Login(ctx, s.buildAccess(cfg))
}

// AddTorrents adds magnet links (or .torrent URLs) and returns the refreshed
// torrent list.
func (s *QBittorrentService) AddTorrents(ctx context.Context, urls []string) ([]client.QBittorrentTorrent, error) {
	clean := make([]string, 0, len(urls))
	for _, raw := range urls {
		raw = strings.TrimSpace(raw)
		if raw != "" {
			clean = append(clean, raw)
		}
	}
	if len(clean) == 0 {
		return nil, errors.New("no torrent urls provided")
	}

	cfg, err := s.store.LoadConfig()
	if err != nil {
		return nil, err
	}
	savePath, err := s.defaultSavePath(cfg)
	if err != nil {
		return nil, err
	}

	access := s.buildAccess(cfg)
	if err := s.client.AddTorrents(ctx, access, clean, savePath); err != nil {
		return nil, err
	}
	log.Printf("qbittorrent added torrents: count=%d save_path=%s", len(clean), savePath)
	return s.client.Torrents(ctx, access)
}

func (s *QBittorrentService) Torrents(ctx context.Context) ([]client.QBittorrentTorrent, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return nil, err
	}
	return s.client.Torrents(ctx, s.buildAccess(cfg))
}

func (s *QBittorrentService) TorrentFiles(ctx context.Context, hash string) ([]client.QBittorrentFile, error) {
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return nil, err
	}
	return s.client.TorrentFiles(ctx, s.buildAccess(cfg), hash)
}

func (s *QBittorrentService) Pause(ctx context.Context, hashes []string) error {
	return s.action(ctx, hashes, "pause")
}

func (s *QBittorrentService) Resume(ctx context.Context, hashes []string) error {
	return s.action(ctx, hashes, "resume")
}

func (s *QBittorrentService) Delete(ctx context.Context, hashes []string, deleteFiles bool) error {
	if len(hashes) == 0 {
		return errors.New("no torrent hashes provided")
	}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return err
	}
	if err := s.client.Delete(ctx, s.buildAccess(cfg), hashes, deleteFiles); err != nil {
		return err
	}
	log.Printf("qbittorrent deleted torrents: hashes=%v delete_files=%t", hashes, deleteFiles)
	return nil
}

func (s *QBittorrentService) action(ctx context.Context, hashes []string, action string) error {
	if len(hashes) == 0 {
		return errors.New("no torrent hashes provided")
	}
	cfg, err := s.store.LoadConfig()
	if err != nil {
		return err
	}
	access := s.buildAccess(cfg)
	var actionErr error
	if action == "pause" {
		actionErr = s.client.Pause(ctx, access, hashes)
	} else {
		actionErr = s.client.Resume(ctx, access, hashes)
	}
	return actionErr
}

// ScanTorrent resolves a completed torrent's download path (relative to the
// local media root) and starts a BT scan session so its files can be matched
// and uploaded.
func (s *QBittorrentService) ScanTorrent(ctx context.Context, hash string, tmdbID int64, videoType string) (model.ScanSession, error) {
	if strings.TrimSpace(hash) == "" {
		return model.ScanSession{}, errors.New("torrent hash is required")
	}
	if tmdbID <= 0 {
		return model.ScanSession{}, errors.New("tmdb_id must be greater than 0")
	}

	cfg, err := s.store.LoadConfig()
	if err != nil {
		return model.ScanSession{}, err
	}
	access := s.buildAccess(cfg)
	torrents, err := s.client.Torrents(ctx, access)
	if err != nil {
		return model.ScanSession{}, err
	}

	var target *client.QBittorrentTorrent
	for i := range torrents {
		if strings.EqualFold(torrents[i].Hash, strings.TrimSpace(hash)) {
			target = &torrents[i]
			break
		}
	}
	if target == nil {
		return model.ScanSession{}, errors.New("torrent not found: " + hash)
	}
	if target.Progress < 1 {
		return model.ScanSession{}, fmt.Errorf("torrent 尚未下载完成（%.1f%%），完成后才能扫描", target.Progress*100)
	}

	contentPath := strings.TrimSpace(target.ContentPath)
	if contentPath == "" {
		return model.ScanSession{}, errors.New("torrent 缺少下载路径，无法扫描")
	}

	root := filepath.Clean(s.localService.Root())
	rel, err := filepath.Rel(root, filepath.Clean(contentPath))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return model.ScanSession{}, fmt.Errorf(
			"BT 下载路径 %s 不在本地媒体根目录 %s 下，请在系统配置调整 qbittorrent 保存路径",
			contentPath, root,
		)
	}
	rel = filepath.ToSlash(rel)

	scan, err := s.scanService.CreateScan(ctx, CreateScanRequest{
		Path:      rel,
		Source:    "bt",
		TMDBID:    tmdbID,
		VideoType: videoType,
	})
	if err != nil {
		return model.ScanSession{}, err
	}
	log.Printf("qbittorrent scan started: hash=%s tmdb_id=%d path=%s scan=%s", hash, tmdbID, rel, scan.ID)
	return scan, nil
}
