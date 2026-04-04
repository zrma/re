package re

import "sort"

type ResolutionResult struct {
	Episodes               []string
	MoviesByEpisode        map[string]MediaFile
	AmbiguousMovieEpisodes map[string]bool
	SubtitlesByEpisode     map[string][]MediaFile
	UnresolvedMovies       []MediaFile
	UnresolvedSubtitles    []MediaFile
}

func ResolveByRule(scanResult ScanResult) ResolutionResult {
	result := ResolutionResult{
		MoviesByEpisode:        map[string]MediaFile{},
		AmbiguousMovieEpisodes: map[string]bool{},
		SubtitlesByEpisode:     map[string][]MediaFile{},
	}
	unresolvedMoviePaths := map[string]bool{}
	appendUnresolvedMovie := func(movie MediaFile) {
		if unresolvedMoviePaths[movie.Path] {
			return
		}
		result.UnresolvedMovies = append(result.UnresolvedMovies, movie)
		unresolvedMoviePaths[movie.Path] = true
	}

	for _, movie := range scanResult.Movies {
		episode := extractEpisode(movie.Name)
		if episode == "" {
			appendUnresolvedMovie(movie)
			continue
		}
		if result.AmbiguousMovieEpisodes[episode] {
			appendUnresolvedMovie(movie)
			continue
		}
		if existing, ok := result.MoviesByEpisode[episode]; ok {
			appendUnresolvedMovie(existing)
			appendUnresolvedMovie(movie)
			delete(result.MoviesByEpisode, episode)
			result.AmbiguousMovieEpisodes[episode] = true
			continue
		}
		result.MoviesByEpisode[episode] = movie
	}

	for _, sub := range scanResult.Subtitles {
		episode := extractEpisode(sub.Name)
		if episode == "" {
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

func unmatchedSubtitleEpisodes(resolution ResolutionResult) []string {
	episodes := make([]string, 0, len(resolution.SubtitlesByEpisode))
	for episode := range resolution.SubtitlesByEpisode {
		if _, ok := resolution.MoviesByEpisode[episode]; ok {
			continue
		}
		episodes = append(episodes, episode)
	}
	sort.Strings(episodes)
	return episodes
}

func CollectUnresolvedSubtitles(resolution ResolutionResult) []MediaFile {
	unresolved := make([]MediaFile, 0, len(resolution.UnresolvedSubtitles))
	unresolved = append(unresolved, resolution.UnresolvedSubtitles...)

	for _, episode := range unmatchedSubtitleEpisodes(resolution) {
		unresolved = append(unresolved, resolution.SubtitlesByEpisode[episode]...)
	}

	return unresolved
}
