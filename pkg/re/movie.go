package re

import (
	"regexp"
	"strings"
)

func episodeFromMovie(fileName string) string {
	if strings.Contains(fileName, "OAD") {
		return "OAD"
	}

	episode := parseMovieTypeDash(fileName)
	if episode != "" {
		return episode
	}

	return parseMovieTypeSeasonUnderscore(fileName)
}

func parseMovieTypeDash(fileName string) string {
	regex := regexp.MustCompile(`- \d{2} (END |)\(`)
	episode := regex.FindString(fileName)
	episode = strings.TrimLeft(episode, "- ")
	episode = strings.TrimRight(episode, " (")
	episode = strings.TrimRight(episode, " END")
	episode = strings.Trim(episode, " ")
	return episode
}

func parseMovieTypeSeasonUnderscore(fileName string) string {
	regex := regexp.MustCompile(`S\d{2}_E\d{2}`)
	episode := regex.FindString(fileName)
	if episode == "" {
		return ""
	}

	return episode[5:]
}
