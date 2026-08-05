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
	reCnEpisode      = regexp.MustCompile(`第\s*([0-9]{1,3}|[一二三四五六七八九十壹贰叁肆伍陆柒捌玖拾廿百]+)\s*[集话話]`)
	reEpisodeToken   = regexp.MustCompile(`(?i)\b(?:EP?|E)(\d{1,3})\b`)
	reBracketEpisode = regexp.MustCompile(`\[(\d{1,3})\]`)
	reLeadingEpisode = regexp.MustCompile(`(?i)^(\d{1,3})([\s_.\-–—]+)(.+?)\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reEpisodeSuffix  = regexp.MustCompile(`(?i)(?:^|[\s_.\-–—]+)(\d{1,3})\s*\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reNumericFile    = regexp.MustCompile(`(?i)^(\d{1,3})\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reCompactEpisode = regexp.MustCompile(`(?i)^(.+?)(\d{1,3})\.(mkv|mp4|avi|mov|wmv|flv|m4v|ts|mpg|mpeg|webm)$`)
	reCompactE       = regexp.MustCompile(`(?i)\s+E\s*$`)
	reSeasonText     = regexp.MustCompile(`(?i)(?:^|[\s._\-(（\[])season[\s._-]*(\d{1,2})(?:$|[\s._\-)）\]])`)
	reSeasonShort    = regexp.MustCompile(`(?i)(?:^|[\s._\-(（\[])S(\d{1,2})(?:$|[\s._\-)）\]])`)
	reCnSeasonShort  = regexp.MustCompile(`(?i)(?:^|[\s._\-(（\[])S(\d{1,2})\s*季(?:$|[\s._\-)）\]])`)
	reSeasonRoman    = regexp.MustCompile(`(?i)(?:^|[\s._\-(（\[])season[\s._-]*([ivxlcdm]{1,8})(?:$|[\s._\-)）\]])`)
	reCnSeasonDigit  = regexp.MustCompile(`第\s*(\d{1,2})\s*季`)
	reCnSeasonWord   = regexp.MustCompile(`第([一二三四五六七八九十壹贰叁肆伍陆柒捌玖拾廿百]+)\s*季`)
	reYearOnly       = regexp.MustCompile(`^\d{4}$`)
	reJunkToken      = regexp.MustCompile(`(?i)^(S\d{1,2}|Season\s*\d+|第.+季|Specials?|特别篇|Complete|全集|全\d+集)$`)
)

var compactInvalidTitles = map[string]struct{}{
	"BIG5": {}, "CHS": {}, "CHT": {}, "GB": {}, "HEVC": {}, "AAC": {},
	"WEB": {}, "WEB-DL": {}, "WEBRIP": {}, "1080P": {}, "720P": {},
	"2160P": {}, "4K": {}, "MP4": {}, "MKV": {}, "AVI": {},
}

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

	// "001 标题.mkv" / "530 标题 6.mp4": episode number at the start of the
	// filename. Checked before the trailing-number rule so titles that end in
	// digits (e.g. "530 神秘大三角 6.mp4") are not misread as episode 6.
	// Leading numbers are unambiguous, so allow the full 1-3 digit range
	// (long-running animes routinely exceed 200 episodes).
	if info.Episode == nil {
		if matched := reLeadingEpisode.FindStringSubmatch(name); len(matched) == 5 {
			if loc := reLeadingEpisode.FindStringSubmatchIndex(name); len(loc) >= 6 && safeLeadingEpisode(name, loc[4], matched[1]) {
				ep := mustAtoi(matched[1])
				if ep > 0 {
					info.Episode = intPtr(ep)
					info.RawText = matched[0]
				}
			}
		}
	}

	// Name - 01.mkv
	if info.Episode == nil {
		if matched := reEpisodeSuffix.FindStringSubmatch(name); len(matched) >= 2 {
			if loc := reEpisodeSuffix.FindStringSubmatchIndex(name); len(loc) >= 4 && safeEpisodeSuffix(name, loc[2], matched[1]) {
				ep := mustAtoi(matched[1])
				if ep > 0 && ep <= 200 {
					info.Episode = intPtr(ep)
					info.RawText = matched[0]
				}
			}
		}
	}

	// Compact show name + episode: 入青云01.mkv, TonikakuKawaii08.mkv
	if info.Episode == nil {
		if matched := reCompactEpisode.FindStringSubmatch(name); len(matched) == 4 {
			showPart := matched[1]
			showName := cleanCompactShowName(showPart)
			ep := mustAtoi(matched[2])
			if validCompactEpisode(showPart, showName, matched[2], ep) {
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
	} else if info.Episode != nil &&
		(reBracketEpisode.MatchString(info.RawText) ||
			reLeadingEpisode.MatchString(info.RawText) ||
			(info.RawText == filepath.Base(fullPath) && reCompactEpisode.MatchString(info.RawText))) {
			if season := seasonFromBracketShowName(fullPath); season != nil {
				info.Season = season
			} else {
				info.Season = intPtr(1)
			}
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
	if matched := reCnSeasonShort.FindStringSubmatch(segment); len(matched) == 2 {
		return intPtr(mustAtoi(matched[1]))
	}
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
	if matched := reSeasonRoman.FindStringSubmatch(segment); len(matched) == 2 {
		if n := romanToInt(matched[1]); n > 0 {
			return intPtr(n)
		}
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
			name = bracketSegmentTitle(parts[i])
		}
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

func cleanCompactShowName(showPart string) string {
	name := strings.TrimSpace(showPart)
	name = reCompactE.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

func validCompactEpisode(showPart, showName, episodeText string, episode int) bool {
	if episode <= 0 || episode > 200 || showName == "" || len([]rune(showName)) < 2 {
		return false
	}
	if _, invalid := compactInvalidTitles[strings.ToUpper(showName)]; invalid {
		return false
	}
	if isAllASCIIHexDigits(showName) {
		return false
	}

	epStart := len(showPart)
	if len(episodeText) == 1 {
		if epStart >= 2 && showPart[epStart-1] == '.' && showPart[epStart-2] >= '0' && showPart[epStart-2] <= '9' {
			return false // 5.1 / 7.1 audio channel fraction
		}
		if epStart >= 1 && isASCIIAlphaNumeric(showPart[epStart-1]) {
			return false // AC3, x264 and similar tag suffix
		}
	}
	if strings.HasSuffix(showPart, ".") || strings.HasSuffix(showPart, "_") || strings.HasSuffix(showPart, "-") {
		return false
	}
	return true
}

// safeLeadingEpisode rejects version-like leading numbers such as "5.1.mkv"
// (audio channel) where the digits are directly followed by ".digits.".
func safeLeadingEpisode(name string, sepStart int, episodeText string) bool {
	if len(name) <= sepStart || name[sepStart] != '.' {
		return true
	}
	// "5.1.mkv" / "7.1.2.mkv": single digit directly after the dot
	if len(episodeText) == 1 && sepStart+2 < len(name) && name[sepStart+1] >= '0' && name[sepStart+1] <= '9' && name[sepStart+2] == '.' {
		return false
	}
	// "5.12.mkv": digit run directly after the dot
	i := sepStart + 1
	for i < len(name) && name[i] >= '0' && name[i] <= '9' {
		i++
	}
	return !(i < len(name) && name[i] == '.')
}

func safeEpisodeSuffix(name string, epStart int, episodeText string) bool {
	if len(episodeText) != 1 {
		return true
	}
	if epStart >= 2 && name[epStart-1] == '.' && name[epStart-2] >= '0' && name[epStart-2] <= '9' {
		return false // 5.1 / 7.1 audio channel fraction
	}
	return !(epStart >= 1 && isASCIIAlphaNumeric(name[epStart-1]))
}

func isAllASCIIHexDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isASCIIAlphaNumeric(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'z') ||
		(value >= 'A' && value <= 'Z')
}

func seasonFromBracketShowName(fullPath string) *int {
	segments := strings.Split(filepath.ToSlash(fullPath), "/")
	candidates := make([]string, 0, 2)
	if len(segments) > 0 {
		candidates = append(candidates, segments[len(segments)-1])
	}
	if len(segments) > 1 {
		candidates = append(candidates, segments[len(segments)-2])
	}
	for _, segment := range candidates {
		title := bracketSegmentTitle(segment)
		if title == "" {
			continue
		}
		if season := seasonNumberFromSegment(title); season != nil {
			return season
		}
	}
	return nil
}

func bracketSegmentTitle(segment string) string {
	trimmed := strings.TrimSpace(segment)
	if !strings.HasPrefix(trimmed, "[") {
		return ""
	}
	end := strings.Index(trimmed, "]")
	if end <= 1 {
		return ""
	}
	title := strings.TrimSpace(trimmed[1:end])
	if title == "" || isAllASCIIHexDigits(title) || strings.ContainsAny(title, "-") {
		return ""
	}
	if _, quality := compactInvalidTitles[strings.ToUpper(title)]; quality {
		return ""
	}
	return strings.TrimSpace(title)
}

func cnNumToInt(s string) int {
	runes := []rune(s)
	if len(runes) == 0 {
		return 0
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}

	digits := map[rune]int{
		'零': 0, '〇': 0,
		'一': 1, '二': 2, '三': 3, '四': 4, '五': 5,
		'六': 6, '七': 7, '八': 8, '九': 9,
		'壹': 1, '贰': 2, '叁': 3, '肆': 4, '伍': 5,
		'陆': 6, '柒': 7, '捌': 8, '玖': 9, '拾': 10,
		'廿': 20, '卅': 30,
	}
	units := map[rune]int{
		'十': 10, '拾': 10, '百': 100, '佰': 100,
		'千': 1000, '仟': 1000,
	}

	total := 0
	last := 0
	for _, r := range runes {
		if n, ok := digits[r]; ok {
			if r == '廿' || r == '卅' {
				total += n
				last = 0
				continue
			}
			last = n
			continue
		}
		if u, ok := units[r]; ok {
			if last == 0 {
				last = 1
			}
			total += last * u
			last = 0
			continue
		}
		return 0
	}
	return total + last
}

func romanToInt(value string) int {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if upper == "" {
		return 0
	}
	values := map[byte]int{
		'I': 1, 'V': 5, 'X': 10, 'L': 50,
		'C': 100, 'D': 500, 'M': 1000,
	}
	total := 0
	for i := 0; i < len(upper); i++ {
		n := values[upper[i]]
		if n == 0 {
			return 0
		}
		if i+1 < len(upper) && n < values[upper[i+1]] {
			total -= n
		} else {
			total += n
		}
	}
	return total
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
