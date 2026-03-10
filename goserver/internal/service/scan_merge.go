package service

import (
	"sort"
	"strings"

	"emosup/goserver/internal/domain"
)

func (s *ScanService) MergeFiles(remoteFiles, localFiles []domain.ScanItem) []domain.ScanItem {
	merged := make(map[string]domain.ScanItem)

	for _, item := range remoteFiles {
		key := normalizeKey(item.Name)
		if key == "" {
			continue
		}
		entry := item
		if entry.Source == "" {
			entry.Source = "openlist"
		}
		merged[key] = entry
	}

	for _, item := range localFiles {
		key := normalizeKey(item.Name)
		if key == "" {
			continue
		}
		entry := item
		if entry.Source == "" {
			entry.Source = "local"
		}
		if base, ok := merged[key]; ok {
			if entry.LocalPath != "" {
				base.LocalPath = entry.LocalPath
			}
			if base.SizeBytes == 0 {
				base.SizeBytes = entry.SizeBytes
				base.Size = entry.Size
			}
			if base.Season == nil {
				base.Season = entry.Season
			}
			if base.Episode == nil {
				base.Episode = entry.Episode
			}
			base.Source = "local"
			merged[key] = base
			continue
		}
		merged[key] = entry
	}

	out := make([]domain.ScanItem, 0, len(merged))
	for _, item := range merged {
		out = append(out, item)
	}

	sort.Slice(out, func(i, j int) bool {
		si, ei := derefInt(out[i].Season), derefInt(out[i].Episode)
		sj, ej := derefInt(out[j].Season), derefInt(out[j].Episode)
		if si != sj {
			return si < sj
		}
		if ei != ej {
			return ei < ej
		}
		ni, nj := normalizeKey(out[i].Name), normalizeKey(out[j].Name)
		if ni != nj {
			return ni < nj
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		pi := firstNonEmpty(out[i].OLPath, out[i].LocalPath)
		pj := firstNonEmpty(out[j].OLPath, out[j].LocalPath)
		return pi < pj
	})

	return out
}

func normalizeKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
