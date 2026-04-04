package re

import "regexp"

type extractFunc func(fileName string) string

var nonEpisodeMarkerRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(ncop|nced|op|ed)(?:\d{0,2})?(?:v\d+)?(?:[^a-z0-9]|$)`)

func extractEpisode(fileName string) string {
	if hasNonEpisodeMarker(fileName) {
		return ""
	}
	if episode := parseOAD(fileName); episode != "" {
		return episode
	}
	return extractChain(
		fileName,
		parseEpisode,
		parseSeasonX,
		parseKanji,
		parseDashBrace,
		parseSeasonUnderscore,
		parseKorean,
		parseEndWith,
		parseRAW,
	)
}

func extractChain(fileName string, extractFuncs ...extractFunc) string {
	for _, extractFunc := range extractFuncs {
		episode := extractFunc(fileName)
		if episode != "" {
			return episode
		}
	}
	return ""
}

func hasNonEpisodeMarker(fileName string) bool {
	return nonEpisodeMarkerRegex.MatchString(fileName)
}
