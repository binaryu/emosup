package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"emosup/goserver/internal/domain"
)

var videoExts = map[string]struct{}{
	".mp4": {}, ".mkv": {}, ".avi": {}, ".mov": {}, ".ts": {}, ".m4v": {}, ".webm": {},
}

type ScanService struct {
	client *http.Client
}

type treeIndex struct {
	VideoType     string
	VLID          int
	Title         string
	DefaultSeason *int
	Episodes      map[[2]int]domain.TreeEpisode
}

func NewScanService() *ScanService {
	return &ScanService{client: &http.Client{Timeout: 30 * time.Second}}
}

func (s *ScanService) WalkVideos(req domain.ScanRemoteRequest) ([]domain.ScanItem, error) {
	stack := []string{req.RootPath}
	items := make([]domain.ScanItem, 0)

	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		list, err := s.listDir(req.OpenListBaseURL, req.OpenListToken, cur)
		if err != nil {
			return nil, err
		}

		for _, raw := range list {
			name := firstString(raw["name"], raw["Name"])
			if name == "" {
				continue
			}
			isDir := firstBool(raw["is_dir"], raw["isDir"])
			size := firstInt64(raw["size"], raw["Size"])
			full := strings.ReplaceAll(strings.TrimRight(cur, "/")+"/"+name, "//", "/")
			if isDir {
				stack = append(stack, full)
				continue
			}
			if _, ok := videoExts[strings.ToLower(filepath.Ext(name))]; !ok {
				continue
			}
			sn, ep := guessSeasonEpisode(name, full)
			items = append(items, domain.ScanItem{
				Name:               name,
				Source:             "openlist",
				OLPath:             full,
				SizeBytes:          size,
				Size:               formatSize(size),
				Season:             sn,
				Episode:            ep,
				Selected:           true,
				MatchStatus:        "unchecked",
				MatchText:          "",
				ServerItemType:     "",
				ServerEpisodeTitle: "",
				ServerDateAir:      "",
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		si, ei := derefInt(items[i].Season), derefInt(items[i].Episode)
		sj, ej := derefInt(items[j].Season), derefInt(items[j].Episode)
		if si != sj {
			return si < sj
		}
		if ei != ej {
			return ei < ej
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func (s *ScanService) Precheck(req domain.PrecheckRequest, tree domain.TreeInfo) domain.PrecheckResponse {
	idx := buildTreeIndex(tree)
	files, conflicts := precheckFiles(idx, req.Files, req.MatchMode)
	return domain.PrecheckResponse{
		Title:         idx.Title,
		VLID:          idx.VLID,
		DefaultSeason: idx.DefaultSeason,
		VideoType:     idx.VideoType,
		Conflicts:     conflicts,
		Files:         files,
	}
}

func (s *ScanService) listDir(baseURL, token, path string) ([]map[string]any, error) {
	url := strings.TrimRight(baseURL, "/") + "/api/fs/list"
	payload, _ := json.Marshal(map[string]string{"path": path, "password": ""})

	makeReq := func(auth string) (*http.Request, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		return req, nil
	}

	req, err := makeReq(token)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if (resp.StatusCode == 401 || resp.StatusCode == 403) && token != "" {
		req2, err := makeReq("Bearer " + token)
		if err != nil {
			return nil, err
		}
		resp, err = s.client.Do(req2)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openlist list http %d", resp.StatusCode)
	}

	var body struct {
		Data struct {
			Content []map[string]any `json:"content"`
			Files   []map[string]any `json:"files"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	if len(body.Data.Content) > 0 {
		return body.Data.Content, nil
	}
	return body.Data.Files, nil
}

func buildTreeIndex(tree domain.TreeInfo) treeIndex {
	idx := treeIndex{
		VideoType: tree.VideoType,
		VLID:      tree.ItemID,
		Title:     tree.Title,
		Episodes:  make(map[[2]int]domain.TreeEpisode),
	}
	normalSeasons := make([]int, 0)
	for _, season := range tree.Seasons {
		if season.SeasonNumber != 0 && len(season.Episodes) > 0 {
			normalSeasons = append(normalSeasons, season.SeasonNumber)
		}
		for _, ep := range season.Episodes {
			idx.Episodes[[2]int{season.SeasonNumber, ep.ItemID}] = ep
		}
	}
	if len(normalSeasons) == 1 {
		idx.DefaultSeason = &normalSeasons[0]
	}
	return idx
}

func precheckFiles(idx treeIndex, files []domain.ScanItem, mode domain.MatchMode) ([]domain.ScanItem, []string) {
	matched := map[[2]int][]string{}
	conflicts := make([]string, 0)
	result := make([]domain.ScanItem, 0, len(files))

	for _, f := range files {
		item := f
		if item.Season == nil || item.Episode == nil {
			sn, ep := guessSeasonEpisode(item.Name, firstNonEmpty(item.LocalPath, item.OLPath, item.Name))
			if item.Season == nil {
				item.Season = sn
			}
			if item.Episode == nil {
				item.Episode = ep
			}
		}
		item.MatchStatus = "missing"
		item.MatchText = ""

		if item.ManualID != "" {
			item.MatchStatus = "ok"
			if strings.Contains(item.ManualID, "ve") {
				item.ServerItemType = "ve"
			} else {
				item.ServerItemType = "vl"
			}
			id := digitsOnlyInt(item.ManualID)
			item.ServerItemID = &id
			item.MatchText = "手动指定: " + item.ManualID
			result = append(result, item)
			continue
		}

		if idx.VideoType == "movie" {
			item.MatchStatus = "ok"
			item.ServerItemType = "vl"
			item.ServerItemID = &idx.VLID
			item.MatchText = fmt.Sprintf("vl-%d | %s", idx.VLID, idx.Title)
			result = append(result, item)
			continue
		}

		if item.Episode == nil {
			item.MatchText = "缺 episode"
			result = append(result, item)
			continue
		}
		if item.Season == nil {
			if idx.DefaultSeason != nil && mode != domain.MatchModeStrict {
				item.Season = idx.DefaultSeason
			} else {
				item.MatchText = "缺 season"
				result = append(result, item)
				continue
			}
		}

		key := [2]int{derefInt(item.Season), derefInt(item.Episode)}
		ep, ok := idx.Episodes[key]
		if !ok {
			item.MatchStatus = "not_in_tree"
			item.MatchText = fmt.Sprintf("tree 无 S%dE%d", key[0], key[1])
			result = append(result, item)
			continue
		}

		item.MatchStatus = "ok"
		item.ServerItemType = "ve"
		item.ServerItemID = &ep.ItemID
		hasMedia := ep.HasMedia
		item.ServerHasMedia = &hasMedia
		item.ServerEpisodeTitle = ep.EpisodeTitle
		item.ServerDateAir = ep.DateAir
		item.MatchText = fmt.Sprintf("S%dE%d -> ve-%d | %s | has_media=%t", key[0], key[1], ep.ItemID, ep.EpisodeTitle, ep.HasMedia)
		matched[key] = append(matched[key], itemKey(item))
		result = append(result, item)
	}

	for key, paths := range matched {
		if len(paths) <= 1 {
			continue
		}
		conflicts = append(conflicts, fmt.Sprintf("冲突：S%dE%d 被 %d 个文件匹配", key[0], key[1], len(paths)))
		for i := range result {
			if derefInt(result[i].Season) == key[0] && derefInt(result[i].Episode) == key[1] && result[i].MatchStatus == "ok" {
				result[i].MatchStatus = "conflict"
				result[i].MatchText = "冲突：" + result[i].MatchText
			}
		}
	}

	return result, conflicts
}

func itemKey(item domain.ScanItem) string {
	return firstNonEmpty(item.OLPath, item.LocalPath, item.Name)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func guessSeasonEpisode(name, source string) (*int, *int) {
	patterns := []string{
		`(?i)s(\d{1,2})e(\d{1,3})`,
		`(?i)(\d{1,2})x(\d{1,3})`,
		`(?i)第\s*(\d{1,2})\s*季.*第\s*(\d{1,3})\s*[集话]`,
		`(?i)第\s*(\d{1,3})\s*[集话]`,
	}
	text := name + " " + source
	for idx, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		m := re.FindStringSubmatch(text)
		if len(m) == 0 {
			continue
		}
		if idx == 3 {
			ep := atoiPtr(m[1])
			return nil, ep
		}
		return atoiPtr(m[1]), atoiPtr(m[2])
	}
	return nil, nil
}

func atoiPtr(s string) *int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &v
}

func digitsOnlyInt(s string) int {
	re := regexp.MustCompile(`\D`)
	clean := re.ReplaceAllString(s, "")
	v, _ := strconv.Atoi(clean)
	return v
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func formatSize(size int64) string {
	if size <= 0 {
		return "unknown"
	}
	return fmt.Sprintf("%.1f MB", float64(size)/1024.0/1024.0)
}

func firstString(values ...any) string {
	for _, v := range values {
		s, ok := v.(string)
		if ok && s != "" {
			return s
		}
	}
	return ""
}

func firstBool(values ...any) bool {
	for _, v := range values {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func firstInt64(values ...any) int64 {
	for _, v := range values {
		switch x := v.(type) {
		case float64:
			return int64(x)
		case int:
			return int64(x)
		case int64:
			return x
		}
	}
	return 0
}
