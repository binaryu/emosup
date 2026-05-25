package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"emosup/backend/internal/client"
	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

type ScanService struct {
	store           *store.FileStore
	openListService *OpenListService
	localService    *LocalService
	emosService     *EmosService
	matchService    *MatchService
}

type CreateScanRequest struct {
	Path      string   `json:"path"`
	FilePath  string   `json:"file_path"`
	FilePaths []string `json:"file_paths"`
	Source    string   `json:"source"`
	TMDBID    int64    `json:"tmdb_id"`
	VideoType string   `json:"video_type"`
}

type UpdateScanItemRequest struct {
	SelectedItemType *string `json:"selected_item_type"`
	SelectedItemID   *int64  `json:"selected_item_id"`
	SelectedTitle    *string `json:"selected_title"`
	Confirmed        *bool   `json:"confirmed"`
}

func NewScanService(
	store *store.FileStore,
	openListService *OpenListService,
	localService *LocalService,
	emosService *EmosService,
	matchService *MatchService,
) *ScanService {
	return &ScanService{
		store:           store,
		openListService: openListService,
		localService:    localService,
		emosService:     emosService,
		matchService:    matchService,
	}
}

func (s *ScanService) CreateScan(ctx context.Context, req CreateScanRequest) (model.ScanSession, error) {
	singleFile := strings.TrimSpace(req.FilePath)
	multiFiles := req.FilePaths
	if singleFile == "" && len(multiFiles) == 0 && strings.TrimSpace(req.Path) == "" {
		return model.ScanSession{}, errors.New("path, file_path or file_paths is required")
	}
	if req.TMDBID <= 0 {
		return model.ScanSession{}, errors.New("tmdb_id must be greater than 0")
	}

	scanPath := req.Path
	if singleFile != "" && scanPath == "" {
		scanPath = singleFile
	}

	now := time.Now()
	scan := model.ScanSession{
		ID:        utils.NewID("scan"),
		Source:    req.Source,
		Path:      scanPath,
		TMDBID:    req.TMDBID,
		VideoType: req.VideoType,
		Status:    model.ScanSessionStatusProcessing,
		Items:     []model.ScanItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	log.Printf("scan started: id=%s path=%s file_path=%s tmdb_id=%d", scan.ID, req.Path, singleFile, req.TMDBID)
	if err := s.store.SaveScan(scan); err != nil {
		return model.ScanSession{}, err
	}

	var entries []client.OpenListEntry
	var err error
	isLocal := strings.EqualFold(req.Source, "local")
	if isLocal && len(multiFiles) > 0 {
		// Local multi-file mode
		for _, fp := range multiFiles {
			fp = strings.TrimSpace(fp)
			if fp == "" {
				continue
			}
			localEntry, localErr := s.localService.GetFileInfo(ctx, fp)
			if localErr != nil {
				log.Printf("local file info failed: scan=%s file=%s err=%v", scan.ID, fp, localErr)
				continue
			}
			entries = append(entries, client.OpenListEntry{
				Name:  localEntry.Name,
				Path:  localEntry.Path,
				IsDir: false,
				Size:  localEntry.Size,
			})
		}
		log.Printf("local multi-file: scan=%s files=%d", scan.ID, len(entries))
	} else if !isLocal && len(multiFiles) > 0 {
		// OpenList multi-file mode
		for _, fp := range multiFiles {
			fp = strings.TrimSpace(fp)
			if fp == "" {
				continue
			}
			fileEntries, fileErr := s.openListService.GetFileInfo(ctx, fp)
			if fileErr != nil {
				log.Printf("openlist file info failed: scan=%s file=%s err=%v", scan.ID, fp, fileErr)
				continue
			}
			entries = append(entries, fileEntries...)
		}
		log.Printf("openlist multi-file: scan=%s files=%d", scan.ID, len(entries))
	} else if isLocal && singleFile != "" {
		// Local single file mode
		localEntry, localErr := s.localService.GetFileInfo(ctx, singleFile)
		if localErr != nil {
			log.Printf("local file info failed: scan=%s file=%s err=%v", scan.ID, singleFile, localErr)
			scan.Status = model.ScanSessionStatusFailed
			scan.UpdatedAt = time.Now()
			_ = s.store.SaveScan(scan)
			return model.ScanSession{}, localErr
		}
		entries = []client.OpenListEntry{{
			Name:  localEntry.Name,
			Path:  localEntry.Path,
			IsDir: false,
			Size:  localEntry.Size,
		}}
		log.Printf("local single file: scan=%s file=%s size=%d", scan.ID, singleFile, localEntry.Size)
	} else if isLocal {
		// Local directory mode: list video files recursively from download dir
		entries = s.listLocalVideosRecursive(ctx, req.Path)
		log.Printf("local list success: scan=%s videos=%d", scan.ID, len(entries))
	} else if singleFile != "" {
		// OpenList single file mode
		entries, err = s.openListService.GetFileInfo(ctx, singleFile)
		if err != nil {
			log.Printf("openlist file info failed: scan=%s file=%s err=%v", scan.ID, singleFile, err)
			scan.Status = model.ScanSessionStatusFailed
			scan.UpdatedAt = time.Now()
			_ = s.store.SaveScan(scan)
			return model.ScanSession{}, err
		}
		log.Printf("openlist single file: scan=%s file=%s", scan.ID, singleFile)
	} else {
		entries, err = s.openListService.ListVideoFiles(ctx, req.Path)
		if err != nil {
			log.Printf("openlist list failed: scan=%s err=%v", scan.ID, err)
			scan.Status = model.ScanSessionStatusFailed
			scan.UpdatedAt = time.Now()
			_ = s.store.SaveScan(scan)
			return model.ScanSession{}, err
		}
		log.Printf("openlist list success: scan=%s videos=%d", scan.ID, len(entries))
	}

	tree, err := s.emosService.GetVideoTree(ctx, req.TMDBID, req.VideoType)
	if err != nil {
		log.Printf("emos tree failed: scan=%s err=%v", scan.ID, err)
		scan.Status = model.ScanSessionStatusFailed
		scan.UpdatedAt = time.Now()
		_ = s.store.SaveScan(scan)
		return model.ScanSession{}, err
	}
	log.Printf("emos tree success: scan=%s video_type=%s seasons=%d", scan.ID, tree.VideoType, len(tree.Seasons))

	if scan.VideoType == "" {
		scan.VideoType = tree.VideoType
	}
	if tree.VideoType == "" {
		tree.VideoType = scan.VideoType
	}

	items := make([]model.ScanItem, 0, len(entries))
	for _, entry := range entries {
		item := model.ScanItem{
			ID:              utils.NewID("item"),
			ScanSessionID:   scan.ID,
			OpenListPath:    entry.Path,
			FileName:        entry.Name,
			FileSize:        entry.Size,
			IsVideo:         true,
			MatchCandidates: []model.MatchCandidate{},
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		if isLocal {
			// Local files: use the local path as raw_url, file is already downloaded
			item.RawURL = entry.Name
		} else {
			rawURL, rawErr := s.openListService.GetRawLink(ctx, entry.Path)
			if rawErr != nil {
				item.MatchStatus = model.MatchStatusInvalid
				item.MatchReason = "获取 OpenList 直链失败: " + rawErr.Error()
				items = append(items, item)
				log.Printf("scan item raw link failed: scan=%s file=%s err=%v", scan.ID, entry.Path, rawErr)
				continue
			}
			item.RawURL = rawURL
		}
		item.Parsed = utils.ParseEpisodeInfo(entry.Name, entry.Path)
		matchResult := s.matchService.Match(tree, item.Parsed)
		item.MatchStatus = matchResult.Status
		item.MatchReason = matchResult.Reason
		item.MatchCandidates = matchResult.Candidates
		item.SelectedItemType = matchResult.SelectedItemType
		item.SelectedItemID = matchResult.SelectedItemID
		item.SelectedTitle = matchResult.SelectedTitle
		item.Confirmed = matchResult.Status == model.MatchStatusMatched &&
			item.SelectedItemID > 0 &&
			strings.TrimSpace(item.SelectedItemType) != ""

		// Look up has_media from tree for matched episodes
		if item.SelectedItemID > 0 && tree.VideoType == "tv" {
		item.HasMedia = lookupHasMedia(&tree, item.SelectedItemID)
		}
		log.Printf(
			"scan item parsed and matched: scan=%s file=%s season=%v episode=%v status=%s",
			scan.ID,
			entry.Name,
			item.Parsed.Season,
			item.Parsed.Episode,
			item.MatchStatus,
		)

		items = append(items, item)
	}

	scan.Items = items
	scan.TotalCount = len(items)
	for _, item := range items {
		if item.MatchStatus == model.MatchStatusMatched {
			scan.MatchedCount++
			continue
		}
		scan.UnmatchedCount++
	}
	scan.Status = model.ScanSessionStatusCompleted
	scan.UpdatedAt = time.Now()

	if err := s.store.SaveScan(scan); err != nil {
		log.Printf("scan save failed: scan=%s err=%v", scan.ID, err)
		return model.ScanSession{}, err
	}

	log.Printf("scan saved: id=%s total=%d matched=%d unmatched=%d", scan.ID, scan.TotalCount, scan.MatchedCount, scan.UnmatchedCount)
	return scan, nil
}

func (s *ScanService) ListScans(_ context.Context) ([]model.ScanSession, error) {
	return s.store.ListScans()
}

func (s *ScanService) GetScan(_ context.Context, id string) (model.ScanSession, error) {
	return s.store.GetScan(id)
}

func (s *ScanService) listLocalVideosRecursive(ctx context.Context, path string) []client.OpenListEntry {
	var result []client.OpenListEntry
	s.localListRecursive(ctx, path, &result)
	return result
}

func (s *ScanService) localListRecursive(ctx context.Context, path string, result *[]client.OpenListEntry) {
	_, entries, err := s.localService.Browse(ctx, path)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir {
			s.localListRecursive(ctx, e.Path, result)
			continue
		}
		if IsVideoFile(e.Name) {
			*result = append(*result, client.OpenListEntry{
				Name:  e.Name,
				Path:  e.Path,
				IsDir: false,
				Size:  e.Size,
			})
		}
	}
}

func (s *ScanService) DeleteScan(_ context.Context, id string) error {
	err := s.store.DeleteScan(id)
	if err != nil {
		log.Printf("scan delete failed: id=%s err=%v", id, err)
		return err
	}

	log.Printf("scan deleted: id=%s", id)
	return nil
}

func (s *ScanService) DeleteScanItem(_ context.Context, scanID, itemID string) (model.ScanSession, error) {
	scan, err := s.store.DeleteScanItem(scanID, itemID)
	if err != nil {
		log.Printf("scan item delete failed: scan=%s item=%s err=%v", scanID, itemID, err)
		return model.ScanSession{}, err
	}

	log.Printf("scan item deleted: scan=%s item=%s remaining=%d", scanID, itemID, scan.TotalCount)
	return scan, nil
}

func (s *ScanService) UpdateScanItem(ctx context.Context, scanID, itemID string, req UpdateScanItemRequest) (model.ScanSession, model.ScanItem, error) {
	updatedAt := time.Now()
	var shouldFetchTitle bool
	scan, err := s.store.UpdateScanItem(scanID, itemID, func(item *model.ScanItem) error {
		if req.SelectedItemType != nil {
			newType := strings.TrimSpace(*req.SelectedItemType)
			if newType != "" && newType != item.SelectedItemType {
				shouldFetchTitle = true
			}
			item.SelectedItemType = newType
		}
		if req.SelectedItemID != nil {
			if *req.SelectedItemID > 0 && *req.SelectedItemID != item.SelectedItemID {
				shouldFetchTitle = true
			}
			item.SelectedItemID = *req.SelectedItemID
		}
		if req.SelectedTitle != nil {
			item.SelectedTitle = strings.TrimSpace(*req.SelectedTitle)
			// Only suppress auto-fetch if user explicitly provided a non-empty title
			if strings.TrimSpace(*req.SelectedTitle) != "" {
				shouldFetchTitle = false
			}
		}
		if req.Confirmed != nil {
			item.Confirmed = *req.Confirmed
		}
		item.UpdatedAt = updatedAt
		return nil
	})
	if err != nil {
		log.Printf("scan item update failed: scan=%s item=%s err=%v", scanID, itemID, err)
		return model.ScanSession{}, model.ScanItem{}, err
	}

	var updatedItem model.ScanItem
	for _, item := range scan.Items {
		if item.ID == itemID {
			updatedItem = item
			break
		}
	}

	// Auto-fetch title from Emos when item_type/item_id changed OR title is still empty
	autoFetch := shouldFetchTitle || strings.TrimSpace(updatedItem.SelectedTitle) == ""
	if autoFetch && strings.TrimSpace(updatedItem.SelectedItemType) != "" && updatedItem.SelectedItemID > 0 {
		base, fetchErr := s.emosService.GetVideoBase(ctx, updatedItem.SelectedItemType, updatedItem.SelectedItemID)
		if fetchErr != nil {
			log.Printf("scan item auto-fetch title failed: scan=%s item=%s err=%v", scanID, itemID, fetchErr)
		} else if strings.TrimSpace(base.Title) != "" {
			scan, err = s.store.UpdateScanItem(scanID, itemID, func(item *model.ScanItem) error {
				item.SelectedTitle = base.Title
				item.UpdatedAt = time.Now()
				return nil
			})
			if err != nil {
				log.Printf("scan item auto-fetch title save failed: scan=%s item=%s err=%v", scanID, itemID, err)
			} else {
				for _, item := range scan.Items {
					if item.ID == itemID {
						updatedItem = item
						break
					}
				}
				log.Printf("scan item title auto-filled: scan=%s item=%s title=%s", scanID, itemID, base.Title)
			}
		}
	}

	log.Printf("scan item updated: scan=%s item=%s confirmed=%v", scanID, itemID, updatedItem.Confirmed)
	return scan, updatedItem, nil
}

func lookupHasMedia(tree *client.EmosVideoTree, itemID int64) *bool {
	for _, season := range tree.Seasons {
		for _, episode := range season.Episodes {
			if episode.ItemID == itemID {
				hasMedia := episode.HasMedia
				return &hasMedia
			}
		}
	}
	return nil
}
