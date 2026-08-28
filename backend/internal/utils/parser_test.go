package utils

import "testing"

func TestParseEpisodeInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fileName  string
		fullPath  string
		season    *int
		episode   *int
		isSpecial bool
	}{
		{
			name:     "standard sxxexx",
			fileName: "Show.S01E02.mkv",
			fullPath: "/TV/Show/Season 1/Show.S01E02.mkv",
			season:   ptr(1),
			episode:  ptr(2),
		},
		{
			name:     "sxxexx with dot",
			fileName: "Show.S01.E02.mkv",
			fullPath: "/TV/Show/Show.S01.E02.mkv",
			season:   ptr(1),
			episode:  ptr(2),
		},
		{
			name:     "bracket episode [01]",
			fileName: "[VCB-Studio] Ao Haru Ride [01][Ma10p_1080p][x265_flac].mkv",
			fullPath: "/ccs/[VCB-Studio] Ao Haru Ride [Ma10p_1080p]/[VCB-Studio] Ao Haru Ride [01][Ma10p_1080p][x265_flac].mkv",
			season:   ptr(1),
			episode:  ptr(1),
		},
		{
			name:     "bracket episode with parent range",
			fileName: "[Solo Leveling][25][BIG5][1080P].mp4",
			fullPath: "/TV/[Solo Leveling][13-25][BIG5][1080P]/[Solo Leveling][25][BIG5][1080P].mp4",
			season:   ptr(1),
			episode:  ptr(25),
		},
		{
			name:     "bracket episode season from show name",
			fileName: "[Tate no Yuusha no Nariagari S4][08][BIG5][720P].mp4",
			fullPath: "/TV/[Tate no Yuusha no Nariagari S4][01-12][BIG5]/[Tate no Yuusha no Nariagari S4][08][BIG5][720P].mp4",
			season:   ptr(4),
			episode:  ptr(8),
		},
		{
			name:     "lolihouse style anime dash episode S1",
			fileName: "[LoliHouse] Hell Mode - 01 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			fullPath: "/anime/[LoliHouse] Hell Mode - 01 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			season:   ptr(1),
			episode:  ptr(1),
		},
		{
			name:     "lolihouse style anime dash episode S2",
			fileName: "[LoliHouse] Hell Mode S2 - 01 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			fullPath: "/anime/[LoliHouse] Hell Mode S2 - 01 [WebRip 1080p HEVC-10bit AAC SRTx2].mkv",
			season:   ptr(2),
			episode:  ptr(1),
		},
		{
			name:     "bracket episode keeps path season",
			fileName: "[Show][25][BIG5].mkv",
			fullPath: "/TV/Show/Season 2/[Show][25][BIG5].mkv",
			season:   ptr(2),
			episode:  ptr(25),
		},
		{
			name:     "compact chinese episode",
			fileName: "入青云01.mp4",
			fullPath: "/TV/入青云/入青云01.mp4",
			season:   ptr(1),
			episode:  ptr(1),
		},
		{
			name:     "compact episode",
			fileName: "TonikakuKawaii08.mkv",
			fullPath: "/TV/TonikakuKawaii/TonikakuKawaii08.mkv",
			season:   ptr(1),
			episode:  ptr(8),
		},
		{
			name:     "audio fraction is not compact episode",
			fileName: "Movie.5.1.mkv",
			fullPath: "/TV/Movie/Movie.5.1.mkv",
			episode:  nil,
		},
		{
			name:     "ac3 suffix is not compact episode",
			fileName: "Movie.AC3.mkv",
			fullPath: "/TV/Movie/Movie.AC3.mkv",
			episode:  nil,
		},
		{
			name:     "x264 suffix is not compact episode",
			fileName: "Movie.x264.mkv",
			fullPath: "/TV/Movie/Movie.x264.mkv",
			episode:  nil,
		},
		{
			name:     "traditional chinese season",
			fileName: "02.mp4",
			fullPath: "/TV/Show/第贰季/02.mp4",
			season:   ptr(2),
			episode:  ptr(2),
		},
		{
			name:     "chinese short season",
			fileName: "02.mp4",
			fullPath: "/TV/Show/S2季/02.mp4",
			season:   ptr(2),
			episode:  ptr(2),
		},
		{
			name:     "roman numeral season",
			fileName: "02.mp4",
			fullPath: "/TV/Show/Season II/02.mp4",
			season:   ptr(2),
			episode:  ptr(2),
		},
		{
			name:     "twenty one chinese season",
			fileName: "02.mp4",
			fullPath: "/TV/Show/第二十一季/02.mp4",
			season:   ptr(21),
			episode:  ptr(2),
		},
		{
			name:     "suffix episode Name - 05.mkv",
			fileName: "Show Name - 05.mkv",
			fullPath: "/TV/Show/Show Name - 05.mkv",
			season:   nil,
			episode:  ptr(5),
		},
		{
			name:     "pure numeric file 01.mp4 with cn season dir",
			fileName: "01.mp4",
			fullPath: "/quark/D 盾牌/第一季（2002）全13集/01.mp4",
			season:   ptr(1),
			episode:  ptr(1),
		},
		{
			name:     "pure numeric file 02.mp4",
			fileName: "02.mp4",
			fullPath: "/TV/Show/Season 2/02.mp4",
			season:   ptr(2),
			episode:  ptr(2),
		},
		{
			name:     "episode from path season",
			fileName: "Show.EP03.mkv",
			fullPath: "/TV/Show/Season 2/Show.EP03.mkv",
			season:   ptr(2),
			episode:  ptr(3),
		},
		{
			name:     "E-only token",
			fileName: "Show.E07.1080p.mkv",
			fullPath: "/TV/Show/Show.E07.1080p.mkv",
			season:   nil,
			episode:  ptr(7),
		},
		{
			name:     "cn episode 第12集",
			fileName: "剧名 第12集.mkv",
			fullPath: "/TV/剧名/剧名 第12集.mkv",
			season:   nil,
			episode:  ptr(12),
		},
		{
			name:     "cn episode 话",
			fileName: "动画 第3话.mkv",
			fullPath: "/anime/动画/第1季/动画 第3话.mkv",
			season:   ptr(1),
			episode:  ptr(3),
		},
		{
			name:      "special season",
			fileName:  "Show.第1集.mkv",
			fullPath:  "/TV/Show/Specials/Show.第1集.mkv",
			season:    ptr(0),
			episode:   ptr(1),
			isSpecial: true,
		},
		{
			name:     "nearest season folder wins",
			fileName: "03.mkv",
			fullPath: "/TV/Show/Season 1/Season 3/03.mkv",
			season:   ptr(3),
			episode:  ptr(3),
		},
		{
			name:     "sxxexx not overwritten by later bracket",
			fileName: "Show.S02E04.[1080p].mkv",
			fullPath: "/TV/Show/Show.S02E04.[1080p].mkv",
			season:   ptr(2),
			episode:  ptr(4),
		},
		{
			name:     "leading episode number with cn title",
			fileName: "001 狼来了(上).mp4",
			fullPath: "/TV/动画/001 狼来了(上).mp4",
			season:   ptr(1),
			episode:  ptr(1),
		},
		{
			name:     "leading episode number with trailing digit title",
			fileName: "530 神秘大三角 6.mp4",
			fullPath: "/TV/动画/530 神秘大三角 6.mp4",
			season:   ptr(1),
			episode:  ptr(530),
		},
		{
			name:     "leading episode number dot separator",
			fileName: "002.狼来了(下).mp4",
			fullPath: "/TV/动画/002.狼来了(下).mp4",
			season:   ptr(1),
			episode:  ptr(2),
		},
		{
			name:     "leading audio channel is not episode",
			fileName: "5.1.mkv",
			fullPath: "/TV/Show/5.1.mkv",
			episode:  nil,
		},
		{
			name:     "leading audio channel dot variant is not episode",
			fileName: "7.1.mkv",
			fullPath: "/TV/Show/7.1.mkv",
			episode:  nil,
		},
		{
			name:     "four digit year is not leading episode",
			fileName: "2001 太空漫游.mkv",
			fullPath: "/TV/电影/2001 太空漫游.mkv",
			episode:  nil,
		},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			result := ParseEpisodeInfo(testCase.fileName, testCase.fullPath)
			assertIntPtrEqual(t, testCase.season, result.Season)
			assertIntPtrEqual(t, testCase.episode, result.Episode)
			if result.IsSpecial != testCase.isSpecial {
				t.Fatalf("expected special=%v, got %v", testCase.isSpecial, result.IsSpecial)
			}
		})
	}
}

func TestExtractShowTitle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want string
	}{
		{"/TV/Ao Haru Ride/Season 1", "Ao Haru Ride"},
		{"/ccs/[VCB-Studio] Ao Haru Ride [Ma10p_1080p]", "Ao Haru Ride"},
		{"/quark/盾牌勇者/第一季（2002）全13集", "盾牌勇者"},
		{"/anime/Show Name/S01", "Show Name"},
		{"/", ""},
	}
	for _, c := range cases {
		got := ExtractShowTitle(c.path)
		if got != c.want {
			t.Fatalf("ExtractShowTitle(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func assertIntPtrEqual(t *testing.T, expected, actual *int) {
	t.Helper()

	if expected == nil && actual == nil {
		return
	}
	if expected == nil || actual == nil {
		t.Fatalf("expected %v, got %v", expected, actual)
	}
	if *expected != *actual {
		t.Fatalf("expected %d, got %d", *expected, *actual)
	}
}

func ptr(value int) *int {
	return &value
}
