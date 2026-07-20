package utils

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"emosup/backend/internal/model"
)

// Episode/season parsing priority (first match wins for episode):
//  1. SxxExx / Sxx.Exx
//  2. NxNN  (1x02)
//  3. 第N集 / 第N话
//  4. EP/E + digits (EP03, E03)
//  5. [NN] bracket (anime releases)
//  6. trailing " - 01.ext" / "_01.ext"
//  7. pure numeric filename 01.mp4
//
// Season: from SxxExx / NxNN first; else nearest path segment (bottom-up).

var (
	reSeasonEpisode  = regexp.MustCompile(`(?i)\bS(\d{1,2})[\s._-]*E(\d{1,3})\b`)
	reXEpisode       = regexp.MustCompile(`(?i)\b(\d{1,2})x(\d{1,3})\b`)
	reCnEpisode      = regexp.MustCompile(`第\s*([0-9]{1,3}|[一二三四五六七八九十百]+)\s*[集话]`)
	reEpisodeToken   = regexp.MustCompile(`(?i)\b(?:EP?|E)(\d{1,3})\b`)
	reBracketEpisode = regexp.MustCompile(`\[(\d{1,3})\]`)
	reEpisodeSuffix  = regexp.MustCompile(`(?i)(?:^|[\s_.\-–—]+)(\d{1,3})\s*\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reNumericFile    = regexp.MustCompile(`(?i)^(\d{1,3})\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reSeasonText     = regexp.MustCompile(`(?i)(?:^|[\s._\-(（\[])season[\s._-]*(\d{1,2})(?:$|[\s._\-)）\]])`)
	reSeasonShort    = regexp.MustCompile(`(?i)(?:^|[\s._\-(（\[])S(\d{1,2})(?:$|[\s._\-)）\]])`)
	reCnSeasonDigit  = regexp.MustCompile(`第\s*(\d{1,2})\s*季`)
	reCnSeasonWord   = regexp.MustCompile(`第([一二三四五六七八九十]+)\s*季`)
	reYearOnly       = regexp.MustCompile(`^\d{4}$`)
	reJunkToken      = regexp.MustCompile(`(?i)^(S\d{1,2}|Season\s*\d+|第.+季|Specials?|特别篇|Complete|全集|全\d+集)$`)
)

func ParseEpisodeInfo(fileName, fullPath string) model.ParsedEpisodeInfo {
	info := model.ParsedEpisodeInfo{}
	name := strings.TrimSpace(fileName)
	full := strings.TrimSpace(fullPath)

	info.IsSpecial = isSpecialPath(full)
	if info.IsSpecial {
		info.Season = intPtr(0)
	}

	// --- Episode + season from filename (highest priority patterns) ---
	if matched := reSeasonEpisode.FindStringSubmatch(name); len(matched) == 3 {
		info.Season = intPtr(mustAtoi(matched[1]))
		info.Episode = intPtr(mustAtoi(matched[2]))
		info.RawText = matched[0]
		return finalizeParsed(info, full)
	}

	if matched := reXEpisode.FindStringSubmatch(name); len(matched) == 3 {
		info.Season = intPtr(mustAtoi(matched[1]))
		info.Episode = intPtr(mustAtoi(matched[2]))
		info.RawText = matched[0]
		return finalizeParsed(info, full)
	}

	// Chinese 第N集 / 第N话
	if matched := reCnEpisode.FindStringSubmatch(name); len(matched) == 2 {
		if ep := parseFlexibleNumber(matched[1]); ep > 0 && ep <= 200 {
			info.Episode = intPtr(ep)
			info.RawText = matched[0]
		}
	}

	// EP03 / E03 (only if episode not set)
	if info.Episode == nil {
		if matched := reEpisodeToken.FindStringSubmatch(name); len(matched) == 2 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
	}

	// [01] anime bracket
	if info.Episode == nil {
		if matched := reBracketEpisode.FindStringSubmatch(name); len(matched) == 2 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
	}

	// Name - 01.mkv
	if info.Episode == nil {
		if matched := reEpisodeSuffix.FindStringSubmatch(name); len(matched) >= 2 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
	}

	// 01.mp4 pure numeric
	if info.Episode == nil {
		if matched := reNumericFile.FindStringSubmatch(name); len(matched) >= 2 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
	}

	return finalizeParsed(info, full)
}

func finalizeParsed(info model.ParsedEpisodeInfo, fullPath string) model.ParsedEpisodeInfo {
	if info.Season == nil {
		if season := parseSeasonFromPath(fullPath); season != nil {
			info.Season = season
		}
	}
	if info.IsSpecial {
		info.Season = intPtr(0)
	}
	return info
}

// parseSeasonFromPath prefers the nearest (rightmost) path segment that looks like a season folder.
func parseSeasonFromPath(fullPath string) *int {
	segments := strings.Split(filepath.ToSlash(fullPath), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment := strings.TrimSpace(segments[i])
		if segment == "" {
			continue
		}
		// Skip the filename itself for season-only folder detection when possible
		if i == len(segments)-1 && strings.Contains(segment, ".") {
			// still try filename for embedded season markers already handled
			continue
		}

		if isSpecialName(segment) {
			return intPtr(0)
		}
		if n := seasonNumberFromSegment(segment); n != nil {
			return n
		}
	}
	// Fallback: check filename segment too
	if len(segments) > 0 {
		if n := seasonNumberFromSegment(segments[len(segments)-1]); n != nil {
			return n
		}
	}
	return nil
}

func seasonNumberFromSegment(segment string) *int {
	if matched := reCnSeasonWord.FindStringSubmatch(segment); len(matched) == 2 {
		if n := cnNumToInt(matched[1]); n > 0 {
			return intPtr(n)
		}
	}
	if matched := reCnSeasonDigit.FindStringSubmatch(segment); len(matched) == 2 {
		return intPtr(mustAtoi(matched[1]))
	}
	if matched := reSeasonText.FindStringSubmatch(segment); len(matched) == 2 {
		return intPtr(mustAtoi(matched[1]))
	}
	if matched := reSeasonShort.FindStringSubmatch(segment); len(matched) == 2 {
		return intPtr(mustAtoi(matched[1]))
	}
	return nil
}

// ExtractShowTitle guesses a show title from a directory path for TMDB search.
func ExtractShowTitle(path string) string {
	path = filepath.ToSlash(strings.TrimSpace(path))
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	// Walk from the end: skip season/special/junk folders, take first meaningful name.
	for i := len(parts) - 1; i >= 0; i-- {
		name := cleanTitleSegment(parts[i])
		if name == "" {
			continue
		}
		if reJunkToken.MatchString(name) || isSpecialName(name) {
			continue
		}
		if reYearOnly.MatchString(name) {
			continue
		}
		// If this segment is pure "Season 1" style after clean, skip
		if seasonNumberFromSegment(parts[i]) != nil && len(name) < 12 {
			// likely just season folder
			if reSeasonText.MatchString(parts[i]) || reSeasonShort.MatchString(parts[i]) ||
				reCnSeasonDigit.MatchString(parts[i]) || reCnSeasonWord.MatchString(parts[i]) {
				continue
			}
		}
		return name
	}
	return ""
}

func cleanTitleSegment(segment string) string {
	name := segment
	// Strip common release tags
	name = regexp.MustCompile(`\[.*?\]`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`【.*?】`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\(.*?\)`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`（.*?）`).ReplaceAllString(name, " ")
	// Remove season / episode markers
	name = reSeasonEpisode.ReplaceAllString(name, " ")
	name = reXEpisode.ReplaceAllString(name, " ")
	name = reCnEpisode.ReplaceAllString(name, " ")
	name = reEpisodeToken.ReplaceAllString(name, " ")
	name = reCnSeasonDigit.ReplaceAllString(name, " ")
	name = reCnSeasonWord.ReplaceAllString(name, " ")
	name = reSeasonText.ReplaceAllString(name, " ")
	name = reSeasonShort.ReplaceAllString(name, " ")
	// Strip resolution / codec noise
	name = regexp.MustCompile(`(?i)\b(1080p|720p|2160p|4k|8k|bluray|web-?dl|webrip|hdtv|x264|x265|h\.?264|h\.?265|hevc|avc|flac|aac|dts|remux|proper|repack)\b`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`[_./\\]+`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	return strings.TrimSpace(name)
}

func cnNumToInt(s string) int {
	cnMap := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
		'百': 100,
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}
	// pure arabic digits
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	if len(runes) == 1 {
		return cnMap[runes[0]]
	}
	// 十一, 十二
	if runes[0] == '十' {
		if len(runes) == 1 {
			return 10
		}
		return 10 + cnMap[runes[1]]
	}
	// 二十, 二十一
	if len(runes) >= 2 && runes[1] == '十' {
		base := cnMap[runes[0]] * 10
		if len(runes) == 2 {
			return base
		}
		return base + cnMap[runes[2]]
	}
	return 0
}

func parseFlexibleNumber(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return cnNumToInt(s)
}

func isSpecialPath(value string) bool {
	for _, segment := range strings.Split(filepath.ToSlash(value), "/") {
		if isSpecialName(segment) {
			return true
		}
	}
	return false
}

func isSpecialName(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "specials") ||
		strings.Contains(lower, "special") ||
		strings.Contains(value, "特别篇") ||
		strings.Contains(value, "特典") ||
		strings.Contains(value, "OVA") ||
		strings.Contains(value, "OAD")
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
