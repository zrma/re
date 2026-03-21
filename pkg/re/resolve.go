package re

import (
	"log"
	"sort"
)

type ResolutionResult struct {
	Episodes            []string
	MoviesByEpisode     map[string]MediaFile
	SubtitlesByEpisode  map[string][]MediaFile
	UnresolvedMovies    []MediaFile
	UnresolvedSubtitles []MediaFile
}

func ResolveByRule(scanResult ScanResult) ResolutionResult {
	result := ResolutionResult{
		MoviesByEpisode:    map[string]MediaFile{},
		SubtitlesByEpisode: map[string][]MediaFile{},
	}

	for _, movie := range scanResult.Movies {
		episode := extractEpisode(movie.BaseName)
		if episode == "" {
			log.Println("[mov] episode is empty", movie.Name)
			result.UnresolvedMovies = append(result.UnresolvedMovies, movie)
			continue
		}
		result.MoviesByEpisode[episode] = movie
	}

	for _, sub := range scanResult.Subtitles {
		episode := extractEpisode(sub.Name)
		if episode == "" {
			log.Println("[sub] episode is empty", sub.Path)
			result.UnresolvedSubtitles = append(result.UnresolvedSubtitles, sub)
			continue
		}
		result.SubtitlesByEpisode[episode] = append(result.SubtitlesByEpisode[episode], sub)
	}

	episodes := make([]string, 0, len(result.MoviesByEpisode))
	for episode := range result.MoviesByEpisode {
		episodes = append(episodes, episode)
	}
	sort.Strings(episodes)
	result.Episodes = episodes

	return result
}
