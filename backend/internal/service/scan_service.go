package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
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
	// Normalize targets: file_paths (preferred) | file_path | path (directory)
	targets := make([]string, 0, len(req.FilePaths)+2)
	for _, p := range req.FilePaths {
		if t := strings.TrimSpace(p); t != "" {
			targets = append(targets, t)
		}
	}
	if single := strings.TrimSpace(req.FilePath); single != "" {
		targets = append(targets, single)
	}
	if len(targets) == 0 {
		if dir := strings.TrimSpace(req.Path); dir != "" {
			targets = append(targets, dir)
		}
	}
	if len(targets) == 0 {
		return model.ScanSession{}, errors.New("path, file_path or file_paths is required")
	}
	if req.TMDBID <= 0 {
		return model.ScanSession{}, errors.New("tmdb_id must be greater than 0")
	}

	scanPath := strings.TrimSpace(req.Path)
	if scanPath == "" {
		scanPath = targets[0]
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

	log.Printf("scan started: id=%s path=%s targets=%d tmdb_id=%d source=%s", scan.ID, scanPath, len(targets), req.TMDBID, req.Source)
	if err := s.store.SaveScan(scan); err != nil {
		return model.ScanSession{}, err
	}

	isLocal := strings.EqualFold(req.Source, "local")
	entries, err := s.collectVideoEntries(ctx, isLocal, targets)
	if err != nil {
		log.Printf("scan collect failed: scan=%s err=%v", scan.ID, err)
		scan.Status = model.ScanSessionStatusFailed
		scan.UpdatedAt = time.Now()
		_ = s.store.SaveScan(scan)
		return model.ScanSession{}, err
	}
	log.Printf("scan collect success: scan=%s videos=%d", scan.ID, len(entries))

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

	// Load the OpenList access once so per-file raw-link fetches skip the
	// config reload + token check for every one of the (possibly hundreds of)
	// video files.
	var openListAccess client.OpenListAccess
	if !isLocal {
		cfg, cfgErr := s.store.LoadConfig()
		if cfgErr != nil {
			log.Printf("scan config load failed: scan=%s err=%v", scan.ID, cfgErr)
			scan.Status = model.ScanSessionStatusFailed
			scan.UpdatedAt = time.Now()
			_ = s.store.SaveScan(scan)
			return model.ScanSession{}, cfgErr
		}
		openListAccess = s.openListService.buildAccess(cfg)
		if err := s.openListService.ensureToken(ctx, &openListAccess); err != nil {
			log.Printf("scan openlist auth failed: scan=%s err=%v", scan.ID, err)
			scan.Status = model.ScanSessionStatusFailed
			scan.UpdatedAt = time.Now()
			_ = s.store.SaveScan(scan)
			return model.ScanSession{}, err
		}
	}

	treeIndex := s.matchService.BuildIndex(tree)

	items, err := s.processEntries(ctx, scan.ID, treeIndex, isLocal, openListAccess, entries, now)
	if err != nil {
		log.Printf("scan aborted: scan=%s err=%v", scan.ID, err)
		scan.Status = model.ScanSessionStatusFailed
		scan.UpdatedAt = time.Now()
		_ = s.store.SaveScan(scan)
		return model.ScanSession{}, err
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

// collectVideoEntries expands mixed file/dir targets into a de-duplicated video list.
func (s *ScanService) collectVideoEntries(ctx context.Context, isLocal bool, targets []string) ([]client.OpenListEntry, error) {
	seen := make(map[string]struct{})
	result := make([]client.OpenListEntry, 0)

	add := func(entry client.OpenListEntry) {
		key := strings.TrimSpace(entry.Path)
		if key == "" {
			key = entry.Name
		}
		if _, ok := seen[key]; ok {
			return
		}
		if !IsVideoFile(entry.Name) && !entry.IsDir {
			return
		}
		if entry.IsDir {
			return
		}
		seen[key] = struct{}{}
		result = append(result, entry)
	}

	for _, target := range targets {
		target = strings.TrimSpace(target)
		if target == "" {
			continue
		}

		if isLocal {
			if s.localService.IsDir(ctx, target) {
				for _, e := range s.listLocalVideosRecursive(ctx, target) {
					add(e)
				}
				continue
			}
			localEntry, err := s.localService.GetFileInfo(ctx, target)
			if err != nil {
				log.Printf("local target skip: path=%s err=%v", target, err)
				continue
			}
			add(client.OpenListEntry{
				Name:  localEntry.Name,
				Path:  localEntry.Path,
				IsDir: false,
				Size:  localEntry.Size,
			})
			continue
		}

		// OpenList: try as file first; if not found, treat as directory.
		fileEntries, fileErr := s.openListService.GetFileInfo(ctx, target)
		if fileErr == nil && len(fileEntries) > 0 {
			for _, e := range fileEntries {
				add(e)
			}
			continue
		}
		dirVideos, dirErr := s.openListService.ListVideoFiles(ctx, target)
		if dirErr != nil {
			log.Printf("openlist target skip: path=%s fileErr=%v dirErr=%v", target, fileErr, dirErr)
			continue
		}
		for _, e := range dirVideos {
			add(e)
		}
	}

	if len(result) == 0 {
		return nil, errors.New("no video files found in selected targets")
	}
	return result, nil
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

// scanItemConcurrency bounds the worker pool used to process scan entries.
// Each OpenList raw-link fetch is an independent HTTP round trip; processing
// hundreds of files sequentially would take minutes and hit request timeouts.
const scanItemConcurrency = 12

type scanItemJob struct {
	index int
	entry client.OpenListEntry
}

type scanItemResult struct {
	index int
	item  model.ScanItem
	err   error
}

// processEntries parses, matches and fetches raw links for all collected
// entries concurrently (bounded worker pool), preserving input order.
func (s *ScanService) processEntries(ctx context.Context, scanID string, treeIndex *VideoTreeIndex, isLocal bool, openListAccess client.OpenListAccess, entries []client.OpenListEntry, now time.Time) ([]model.ScanItem, error) {
	workerCount := scanItemConcurrency
	if len(entries) < workerCount {
		workerCount = len(entries)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan scanItemJob)
	results := make(chan scanItemResult, len(entries))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				item, err := s.processScanEntry(ctx, scanID, treeIndex, isLocal, openListAccess, now, job.entry)
				if err != nil {
					cancel()
					results <- scanItemResult{index: job.index, err: err}
					return
				}
				results <- scanItemResult{index: job.index, item: item}
			}
		}()
	}

sendJobs:
	for i, entry := range entries {
		select {
		case jobs <- scanItemJob{index: i, entry: entry}:
		case <-ctx.Done():
			break sendJobs
		}
	}
	close(jobs)
	go func() {
		wg.Wait()
		close(results)
	}()

	items := make([]model.ScanItem, len(entries))
	var scanErr error
	for res := range results {
		if res.err != nil {
			if scanErr == nil {
				scanErr = res.err
			}
			continue
		}
		items[res.index] = res.item
	}
	if scanErr != nil {
		return nil, scanErr
	}
	return items, nil
}

// processScanEntry builds a ScanItem for one entry: fetches the raw link
// (OpenList only), parses episode info, matches against the pre-indexed tree
// and looks up has_media. A cancelled context is fatal (the request died);
// per-file raw-link failures only mark that item invalid.
func (s *ScanService) processScanEntry(ctx context.Context, scanID string, treeIndex *VideoTreeIndex, isLocal bool, openListAccess client.OpenListAccess, now time.Time, entry client.OpenListEntry) (model.ScanItem, error) {
	if err := ctx.Err(); err != nil {
		return model.ScanItem{}, err
	}

	item := model.ScanItem{
		ID:              utils.NewID("item"),
		ScanSessionID:   scanID,
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
		rawURL, rawErr := s.openListService.GetRawLinkWithAccess(ctx, openListAccess, entry.Path)
		if rawErr != nil {
			item.MatchStatus = model.MatchStatusInvalid
			item.MatchReason = "获取 OpenList 直链失败: " + rawErr.Error()
			log.Printf("scan item raw link failed: scan=%s file=%s err=%v", scanID, entry.Path, rawErr)
			return item, nil
		}
		item.RawURL = rawURL
	}

	item.Parsed = utils.ParseEpisodeInfo(entry.Name, entry.Path)
	matchResult := treeIndex.Match(item.Parsed)
	item.MatchStatus = matchResult.Status
	item.MatchReason = matchResult.Reason
	item.MatchCandidates = matchResult.Candidates
	item.SelectedItemType = matchResult.SelectedItemType
	item.SelectedItemID = matchResult.SelectedItemID
	item.SelectedTitle = matchResult.SelectedTitle
	item.Confirmed = matchResult.Status == model.MatchStatusMatched &&
		item.SelectedItemID > 0 &&
		strings.TrimSpace(item.SelectedItemType) != ""

	// Look up has_media from the indexed tree for matched episodes
	if item.SelectedItemID > 0 && treeIndex.videoType == "tv" {
		item.HasMedia = treeIndex.LookupHasMedia(item.SelectedItemID)
	}
	log.Printf(
		"scan item parsed and matched: scan=%s file=%s season=%v episode=%v status=%s",
		scanID,
		entry.Name,
		item.Parsed.Season,
		item.Parsed.Episode,
		item.MatchStatus,
	)

	return item, nil
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
