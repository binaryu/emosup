package service

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	"emosup/goserver/internal/domain"
)

func (s *ScanService) WalkLocal(rootPath string) ([]domain.ScanItem, error) {
	stat, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, err
	}

	items := make([]domain.ScanItem, 0)
	err = filepath.WalkDir(stat, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if _, ok := videoExts[filepath.Ext(stringsToLower(d.Name()))]; !ok {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		sn, ep := guessSeasonEpisode(d.Name(), path)
		items = append(items, domain.ScanItem{
			Name:               d.Name(),
			Source:             "local",
			LocalPath:          path,
			SizeBytes:          info.Size(),
			Size:               formatSize(info.Size()),
			Season:             sn,
			Episode:            ep,
			Selected:           true,
			MatchStatus:        "unchecked",
			MatchText:          "",
			ServerItemType:     "",
			ServerEpisodeTitle: "",
			ServerDateAir:      "",
		})
		return nil
	})
	if err != nil {
		return nil, err
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
	if len(items) == 0 {
		return nil, fmt.Errorf("未找到可用视频文件")
	}
	return items, nil
}

func stringsToLower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] = b[i] + 32
		}
	}
	return string(b)
}
