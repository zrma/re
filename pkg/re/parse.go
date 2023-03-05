package re

import (
	"regexp"
	"strings"
)

func parseOAD(name string) string {
	regex := regexp.MustCompile(`OAD`)
	episode := regex.FindString(name)
	return episode
}

func parseDashBrace(name string) string {
	regex := regexp.MustCompile(`- \d{2} (END |)\(`)
	episode := regex.FindString(name)
	episode = strings.TrimLeft(episode, "- ")
	episode = strings.TrimRight(episode, " (")
	episode = strings.TrimRight(episode, " END")
	episode = strings.Trim(episode, " ")
	return episode
}

func parseSeasonUnderscore(name string) string {
	regex := regexp.MustCompile(`S\d{2}_E\d{2}`)
	episode := regex.FindString(name)
	if episode == "" {
		return ""
	}
	return episode[5:]
}

func parseEndWith(name string) string {
	regex := regexp.MustCompile(`\d{2}\.`)
	episode := regex.FindString(name)
	episode = strings.TrimRight(episode, ".")
	episode = strings.Trim(episode, " ")
	return episode
}

func parseKorean(name string) string {
	regex := regexp.MustCompile(`\d{1,2}화`)
	episode := regex.FindString(name)
	episode = strings.TrimRight(episode, "화")
	episode = strings.Trim(episode, " ")
	if len(episode) == 1 {
		episode = "0" + episode
	}
	return episode
}
