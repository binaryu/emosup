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
			name:     "bracket episode [01]",
			fileName: "[VCB-Studio] Ao Haru Ride [01][Ma10p_1080p][x265_flac].mkv",
			fullPath: "/ccs/[VCB-Studio] Ao Haru Ride [Ma10p_1080p]/[VCB-Studio] Ao Haru Ride [01][Ma10p_1080p][x265_flac].mkv",
			season:   nil,
			episode:  ptr(1),
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
			name:      "special season",
			fileName:  "Show.第1集.mkv",
			fullPath:  "/TV/Show/Specials/Show.第1集.mkv",
			season:    ptr(0),
			episode:   ptr(1),
			isSpecial: true,
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
