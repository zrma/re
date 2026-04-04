package re

import "testing"

func TestExtractEpisodeMatchesExpandedEpisodePatterns(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
		want     string
	}{
		{
			name:     "numberedOADWithVersionSuffix",
			fileName: "fansub OAD01v2.srt",
			want:     "OAD01",
		},
		{
			name:     "numberedOADWithSpaceSeparator",
			fileName: "fansub OAD 01v2.srt",
			want:     "OAD01",
		},
		{
			name:     "numberedOADWithHyphenSeparator",
			fileName: "fansub OAD-01.ass",
			want:     "OAD01",
		},
		{
			name:     "episodeTokenAtStart",
			fileName: "E01.srt",
			want:     "01",
		},
		{
			name:     "seasonXAtStart",
			fileName: "10x01.mkv",
			want:     "01",
		},
		{
			name:     "lowercaseSeasonUnderscore",
			fileName: "show s01_e01.ass",
			want:     "01",
		},
		{
			name:     "episodeTokenWithVersionSuffix",
			fileName: "E01v2.srt",
			want:     "01",
		},
		{
			name:     "seasonXWithVersionSuffix",
			fileName: "10x01v2.mkv",
			want:     "01",
		},
		{
			name:     "seasonUnderscoreWithVersionSuffix",
			fileName: "show s01_e01v2.ass",
			want:     "01",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractEpisode(tt.fileName); got != tt.want {
				t.Fatalf("extractEpisode(%q) = %q, want %q", tt.fileName, got, tt.want)
			}
		})
	}
}

func TestExtractEpisodeRejectsEmbeddedAlphanumericTokens(t *testing.T) {
	testCases := []struct {
		name     string
		fileName string
	}{
		{
			name:     "episodeTokenInsideWord",
			fileName: "foo_e01bit.ass",
		},
		{
			name:     "seasonXInsideWord",
			fileName: "10x01bit.mkv",
		},
		{
			name:     "seasonUnderscoreInsideWord",
			fileName: "show_s01_e01bit.ass",
		},
		{
			name:     "nonEpisodeBonusTrackWithVersionSuffix",
			fileName: "Example OAD NCOP01v2.mkv",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractEpisode(tt.fileName); got != "" {
				t.Fatalf("extractEpisode(%q) = %q, want empty", tt.fileName, got)
			}
		})
	}
}
