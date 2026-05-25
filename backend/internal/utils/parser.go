package utils

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"emosup/backend/internal/model"
)

var (
	reSeasonEpisode   = regexp.MustCompile(`(?i)\bS(\d{1,2})\s*E(\d{1,3})\b`)
	reXEpisode        = regexp.MustCompile(`(?i)\b(\d{1,2})x(\d{1,3})\b`)
	reBracketEpisode  = regexp.MustCompile(`\[(\d{1,3})\]`)
	reNumericFile     = regexp.MustCompile(`(?i)^(\d{1,3})\s*\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reEpisodeSuffix   = regexp.MustCompile(`(?i)[\s_.\-]+(\d{1,3})\s*\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reEpisodeOnly     = regexp.MustCompile(`(?i)\bEP?(\d{1,3})\b`)
	reCnEpisode       = regexp.MustCompile(`第\s*(\d{1,3})\s*集`)
	reSeasonText      = regexp.MustCompile(`(?i)season[\s._-]*(\d{1,2})`)
	reSeasonShort     = regexp.MustCompile(`(?i)\bS(\d{1,2})\b`)
	reCnSeasonDigit   = regexp.MustCompile(`第\s*(\d{1,2})\s*季`)
	reCnSeasonWord    = regexp.MustCompile(`第([一二三四五六七八九十]+)季`)
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

	// Pure numeric filename: 01.mp4, 02.mp4
	if info.Episode == nil {
		if matched := reNumericFile.FindStringSubmatch(name); len(matched) >= 3 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
	}

	// [01] square bracket episode (most common in anime releases)
	if info.Episode == nil {
		if matched := reBracketEpisode.FindStringSubmatch(name); len(matched) == 2 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
	}

	// Name - 01.mkv or Name_01.mkv (episode just before extension)
	if info.Episode == nil {
		if matched := reEpisodeSuffix.FindStringSubmatch(name); len(matched) >= 3 {
			ep := mustAtoi(matched[1])
			if ep > 0 && ep <= 200 {
				info.Episode = intPtr(ep)
				info.RawText = matched[0]
			}
		}
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

		// Chinese number word: 第一季, 第二季...
		if matched := reCnSeasonWord.FindStringSubmatch(segment); len(matched) == 2 {
			if n := cnNumToInt(matched[1]); n > 0 {
				return intPtr(n)
			}
		}

		// Digit season: 第1季, Season 1, S1
		if matched := reCnSeasonDigit.FindStringSubmatch(segment); len(matched) == 2 {
			return intPtr(mustAtoi(matched[1]))
		}

		if matched := reSeasonText.FindStringSubmatch(segment); len(matched) == 2 {
			return intPtr(mustAtoi(matched[1]))
		}

		if matched := reSeasonShort.FindStringSubmatch(segment); len(matched) == 2 {
			return intPtr(mustAtoi(matched[1]))
		}
	}

	return nil
}

func cnNumToInt(s string) int {
	cnMap := map[rune]int{
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
	}
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}
	if len(runes) == 1 {
		return cnMap[runes[0]]
	}
	// Handle 十一, 十二, 二十 etc.
	if runes[0] == '十' {
		return 10 + cnMap[runes[1]]
	}
	if len(runes) == 2 && runes[1] == '十' {
		return cnMap[runes[0]] * 10
	}
	return 0
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
