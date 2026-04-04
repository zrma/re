package re

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var oadRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])oad(?:[._ -]?(\d{1,2}))?(?:v\d+)?(?:[^a-z0-9]|$)`)
var dashBraceRegex = regexp.MustCompile(`- \d{2} (END |)?\(`)
var seasonUnderscoreRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s\d{1,2}_e(\d{2})(?:v\d+)?(?:[^a-z0-9]|$)`)
var seasonXRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])\d{1,2}x(\d{2})(?:v\d+)?(?:[^a-z0-9]|$)`)
var endWithRegex = regexp.MustCompile(`(?:^|[^0-9.]|[^0-9.]\.)(\d{2})\.[^.]+$`)
var koreanRegex = regexp.MustCompile(`(?:^|[^0-9])(\d{1,2})화`)
var rawRegex = regexp.MustCompile(`- \d{2} RAW`)
var episodeRegex = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])e(\d{2})(?:v\d+)?(?:[^a-z0-9]|$)`)
var kanjiRegex = regexp.MustCompile(`第\d{2}話`)

func parseOAD(name string) string {
	matches := oadRegex.FindStringSubmatch(name)
	if len(matches) == 0 {
		return ""
	}
	if len(matches) > 1 && matches[1] != "" {
		number, err := strconv.Atoi(matches[1])
		if err != nil {
			return ""
		}
		return fmt.Sprintf("OAD%02d", number)
	}
	return "OAD"
}

func parseDashBrace(name string) string {
	episode := dashBraceRegex.FindString(name)
	episode = strings.TrimLeft(episode, "- ")
	episode = strings.TrimRight(episode, " (")
	episode = strings.TrimRight(episode, " END")
	episode = strings.Trim(episode, " ")
	return episode
}

func parseSeasonUnderscore(name string) string {
	matches := seasonUnderscoreRegex.FindStringSubmatch(name)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func parseSeasonX(name string) string {
	matches := seasonXRegex.FindStringSubmatch(name)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func parseEndWith(name string) string {
	matches := endWithRegex.FindStringSubmatch(name)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func parseKorean(name string) string {
	matches := koreanRegex.FindStringSubmatch(name)
	if len(matches) < 2 {
		return ""
	}
	episode := strings.TrimSpace(matches[1])
	if len(episode) == 1 {
		episode = "0" + episode
	}
	return episode
}

func parseRAW(name string) string {
	episode := rawRegex.FindString(name)
	episode = strings.TrimLeft(episode, "- ")
	episode = strings.TrimRight(episode, " RAW")
	episode = strings.Trim(episode, " ")
	return episode
}

func parseEpisode(name string) string {
	matches := episodeRegex.FindStringSubmatch(name)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func parseKanji(name string) string {
	episode := kanjiRegex.FindString(name)
	episode = strings.TrimLeft(episode, "第")
	episode = strings.TrimRight(episode, "話")
	return episode
}
