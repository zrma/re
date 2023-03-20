package re

type extractFunc func(fileName string) string

func extractEpisode(fileName string) string {
	return extractChain(
		fileName,
		parseOAD,
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
