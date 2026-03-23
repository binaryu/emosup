package utils

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"emosup/backend/internal/model"
)

var (
	reSeasonEpisode = regexp.MustCompile(`(?i)\bS(\d{1,2})\s*E(\d{1,3})\b`)
	reXEpisode      = regexp.MustCompile(`(?i)\b(\d{1,2})x(\d{1,3})\b`)
	reEpisodeOnly   = regexp.MustCompile(`(?i)\bEP?(\d{1,3})\b`)
	reCnEpisode     = regexp.MustCompile(`第\s*(\d{1,3})\s*集`)
	reSeasonText    = regexp.MustCompile(`(?i)season[\s._-]*(\d{1,2})`)
	reSeasonShort   = regexp.MustCompile(`(?i)\bS(\d{1,2})\b`)
	reCnSeason      = regexp.MustCompile(`第\s*(\d{1,2})\s*季`)
)

func ParseEpisodeInfo(fileName, fullPath string) model.ParsedEpisodeInfo {
	info := model.ParsedEpisodeInfo{}
	name := strings.TrimSpace(fileName)
	full := strings.TrimSpace(fullPath)

	if matched := reSeasonEpisode.FindStringSubmatch(name); len(matched) == 3 {
		info.Season = intPtr(mustAtoi(matched[1]))
		info.Episode = intPtr(mustAtoi(matched[2]))
		info.RawText = matched[0]
		return info
	}

	if matched := reXEpisode.FindStringSubmatch(name); len(matched) == 3 {
		info.Season = intPtr(mustAtoi(matched[1]))
		info.Episode = intPtr(mustAtoi(matched[2]))
		info.RawText = matched[0]
		return info
	}

	info.IsSpecial = isSpecialPath(full)
	if info.IsSpecial {
		info.Season = intPtr(0)
	}

	if matched := reEpisodeOnly.FindStringSubmatch(name); len(matched) == 2 {
		info.Episode = intPtr(mustAtoi(matched[1]))
		info.RawText = matched[0]
	}

	if matched := reCnEpisode.FindStringSubmatch(name); len(matched) == 2 {
		info.Episode = intPtr(mustAtoi(matched[1]))
		info.RawText = matched[0]
	}

	if info.Season == nil {
		if season := parseSeasonFromPath(full); season != nil {
			info.Season = season
		}
	}

	if info.IsSpecial && info.Season == nil {
		info.Season = intPtr(0)
	}

	return info
}

func parseSeasonFromPath(fullPath string) *int {
	segments := strings.Split(filepath.ToSlash(fullPath), "/")
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		if isSpecialPath(segment) {
			return intPtr(0)
		}

		if matched := reSeasonText.FindStringSubmatch(segment); len(matched) == 2 {
			return intPtr(mustAtoi(matched[1]))
		}

		if matched := reCnSeason.FindStringSubmatch(segment); len(matched) == 2 {
			return intPtr(mustAtoi(matched[1]))
		}

		if matched := reSeasonShort.FindStringSubmatch(segment); len(matched) == 2 {
			return intPtr(mustAtoi(matched[1]))
		}
	}

	return nil
}

func isSpecialPath(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "specials") ||
		strings.Contains(lower, "special") ||
		strings.Contains(value, "特别篇")
}

func mustAtoi(value string) int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}

	return number
}

func intPtr(value int) *int {
	return &value
}
