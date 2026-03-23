package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"emosup/backend/internal/model"
	"emosup/backend/internal/store"
	"emosup/backend/internal/utils"
)

type ScanService struct {
	store           *store.FileStore
	openListService *OpenListService
	emosService     *EmosService
	matchService    *MatchService
}

type CreateScanRequest struct {
	Path      string `json:"path"`
	TMDBID    int64  `json:"tmdb_id"`
	VideoType string `json:"video_type"`
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
	emosService *EmosService,
	matchService *MatchService,
) *ScanService {
	return &ScanService{
		store:           store,
		openListService: openListService,
		emosService:     emosService,
		matchService:    matchService,
	}
}

func (s *ScanService) CreateScan(ctx context.Context, req CreateScanRequest) (model.ScanSession, error) {
	if strings.TrimSpace(req.Path) == "" {
		return model.ScanSession{}, errors.New("path is required")
	}
	if req.TMDBID <= 0 {
		return model.ScanSession{}, errors.New("tmdb_id must be greater than 0")
	}

	now := time.Now()
	scan := model.ScanSession{
		ID:        utils.NewID("scan"),
		Path:      req.Path,
		TMDBID:    req.TMDBID,
		VideoType: req.VideoType,
		Status:    model.ScanSessionStatusProcessing,
		Items:     []model.ScanItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	log.Printf("scan started: id=%s path=%s tmdb_id=%d", scan.ID, req.Path, req.TMDBID)
	if err := s.store.SaveScan(scan); err != nil {
		return model.ScanSession{}, err
	}

	entries, err := s.openListService.ListVideoFiles(ctx, req.Path)
	if err != nil {
		log.Printf("openlist list failed: scan=%s err=%v", scan.ID, err)
		scan.Status = model.ScanSessionStatusFailed
		scan.UpdatedAt = time.Now()
		_ = s.store.SaveScan(scan)
		return model.ScanSession{}, err
	}
	log.Printf("openlist list success: scan=%s videos=%d", scan.ID, len(entries))

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

		rawURL, rawErr := s.openListService.GetRawLink(ctx, entry.Path)
		if rawErr != nil {
			item.MatchStatus = model.MatchStatusInvalid
			item.MatchReason = "获取 OpenList 直链失败: " + rawErr.Error()
			items = append(items, item)
			log.Printf("scan item raw link failed: scan=%s file=%s err=%v", scan.ID, entry.Path, rawErr)
			continue
		}

		item.RawURL = rawURL
		item.Parsed = utils.ParseEpisodeInfo(entry.Name, entry.Path)
		matchResult := s.matchService.Match(tree, item.Parsed)
		item.MatchStatus = matchResult.Status
		item.MatchReason = matchResult.Reason
		item.MatchCandidates = matchResult.Candidates
		item.SelectedItemType = matchResult.SelectedItemType
		item.SelectedItemID = matchResult.SelectedItemID
		item.SelectedTitle = matchResult.SelectedTitle

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

func (s *ScanService) UpdateScanItem(_ context.Context, scanID, itemID string, req UpdateScanItemRequest) (model.ScanSession, model.ScanItem, error) {
	updatedAt := time.Now()
	scan, err := s.store.UpdateScanItem(scanID, itemID, func(item *model.ScanItem) error {
		if req.SelectedItemType != nil {
			item.SelectedItemType = strings.TrimSpace(*req.SelectedItemType)
		}
		if req.SelectedItemID != nil {
			item.SelectedItemID = *req.SelectedItemID
		}
		if req.SelectedTitle != nil {
			item.SelectedTitle = strings.TrimSpace(*req.SelectedTitle)
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

	for _, item := range scan.Items {
		if item.ID == itemID {
			log.Printf("scan item updated: scan=%s item=%s confirmed=%v", scanID, itemID, item.Confirmed)
			return scan, item, nil
		}
	}

	return model.ScanSession{}, model.ScanItem{}, errors.New("updated item not found")
}
