package re

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
)

const duplicateAISubtitleDecisionSkipReason = "ai returned multiple decisions for the same subtitle"

type AIResolver interface {
	Resolve(ctx context.Context, input AIInput) (AIOutput, error)
}

type AIInput struct {
	Directory           string         `json:"directory"`
	Movies              []AIFile       `json:"movies"`
	Subtitles           []AIFile       `json:"subtitles"`
	UnresolvedSubtitles []string       `json:"unresolved_subtitles"`
	ResolvedPairs       []ResolvedPair `json:"resolved_pairs"`
	Rules               AIRules        `json:"rules"`
}

type AIFile struct {
	Path      string `json:"path"`
	BaseName  string `json:"basename"`
	Extension string `json:"extension,omitempty"`
}

type ResolvedPair struct {
	Subtitle string `json:"subtitle"`
	Movie    string `json:"movie"`
	Source   string `json:"source"`
}

type AIRules struct {
	MustPreserveExtension        bool `json:"must_preserve_extension"`
	MustUseExistingMovieBaseName bool `json:"must_use_existing_movie_basename"`
	PreferSkipOverGuess          bool `json:"prefer_skip_over_guess"`
}

type AIDecisionOutcome string

const (
	AIDecisionMatch      AIDecisionOutcome = "match"
	AIDecisionSkip       AIDecisionOutcome = "skip"
	AIDecisionNeedsHuman AIDecisionOutcome = "needs_human"
)

type AIDecision struct {
	SubtitlePath     string            `json:"subtitle_path"`
	Outcome          AIDecisionOutcome `json:"outcome"`
	MatchedMoviePath string            `json:"matched_movie_path"`
	Confidence       float64           `json:"confidence"`
	Reason           string            `json:"reason"`
}

type AIOutput struct {
	Decisions []AIDecision `json:"decisions"`
}

func CollectAICandidateSubtitles(scanResult ScanResult, resolution ResolutionResult, plan RenamePlan) []MediaFile {
	candidates := make([]MediaFile, 0, len(scanResult.Subtitles))
	seen := map[string]bool{}
	subtitlesByPath := map[string]MediaFile{}
	for _, subtitle := range scanResult.Subtitles {
		subtitlesByPath[subtitle.Path] = subtitle
	}

	appendCandidate := func(subtitle MediaFile) {
		if seen[subtitle.Path] {
			return
		}
		candidates = append(candidates, subtitle)
		seen[subtitle.Path] = true
	}

	for _, subtitle := range CollectUnresolvedSubtitles(resolution) {
		appendCandidate(subtitle)
	}

	for _, skip := range plan.Skips {
		subtitle, ok := subtitlesByPath[skip.SourcePath]
		if !ok {
			continue
		}
		appendCandidate(subtitle)
	}

	return candidates
}

func BuildAIInput(targetPath string, scanResult ScanResult, rulePlan RenamePlan, unresolvedCandidates []MediaFile) AIInput {
	movies := make([]AIFile, 0, len(scanResult.Movies))
	for _, movie := range scanResult.Movies {
		movies = append(movies, AIFile{
			Path:      movie.Path,
			BaseName:  movie.BaseName,
			Extension: movie.Extension,
		})
	}

	subtitles := make([]AIFile, 0, len(scanResult.Subtitles))
	for _, subtitle := range scanResult.Subtitles {
		subtitles = append(subtitles, AIFile{
			Path:      subtitle.Path,
			BaseName:  subtitle.BaseName,
			Extension: subtitle.Extension,
		})
	}

	unresolvedSubtitles := make([]string, 0, len(unresolvedCandidates))
	for _, subtitle := range unresolvedCandidates {
		unresolvedSubtitles = append(unresolvedSubtitles, subtitle.Path)
	}

	resolvedPairs := make([]ResolvedPair, 0, len(rulePlan.Operations))
	for _, operation := range rulePlan.Operations {
		if operation.MatchSource != "rule" {
			continue
		}
		resolvedPairs = append(resolvedPairs, ResolvedPair{
			Subtitle: operation.SourcePath,
			Movie:    operation.MoviePath,
			Source:   operation.MatchSource,
		})
	}

	return AIInput{
		Directory:           filepath.Base(targetPath),
		Movies:              movies,
		Subtitles:           subtitles,
		UnresolvedSubtitles: unresolvedSubtitles,
		ResolvedPairs:       resolvedPairs,
		Rules: AIRules{
			MustPreserveExtension:        true,
			MustUseExistingMovieBaseName: true,
			PreferSkipOverGuess:          true,
		},
	}
}

func MergeAIRenamePlan(rulePlan RenamePlan, scanResult ScanResult, unresolvedCandidates []MediaFile, output AIOutput, minConfidence float64) RenamePlan {
	merged := RenamePlan{
		Operations:         append([]RenameOperation{}, rulePlan.Operations...),
		ResolvedMoviePaths: append([]string{}, rulePlan.ResolvedMoviePaths...),
	}
	skipBySource := map[string]SkipOperation{}
	skipOrder := make([]string, 0, len(rulePlan.Skips))
	for _, skip := range rulePlan.Skips {
		skipBySource[skip.SourcePath] = skip
		skipOrder = append(skipOrder, skip.SourcePath)
	}
	setSkip := func(sourcePath string, reason string) {
		if _, ok := skipBySource[sourcePath]; !ok {
			skipOrder = append(skipOrder, sourcePath)
		}
		skipBySource[sourcePath] = SkipOperation{
			SourcePath: sourcePath,
			Reason:     reason,
		}
	}
	removeSkip := func(sourcePath string) {
		delete(skipBySource, sourcePath)
	}

	moviesByPath := map[string]MediaFile{}
	for _, movie := range scanResult.Movies {
		moviesByPath[movie.Path] = movie
	}
	resolvedMovieSeen := map[string]bool{}
	for _, moviePath := range merged.ResolvedMoviePaths {
		resolvedMovieSeen[moviePath] = true
	}
	recordResolvedMovie := func(moviePath string) {
		if moviePath == "" || resolvedMovieSeen[moviePath] {
			return
		}
		merged.ResolvedMoviePaths = append(merged.ResolvedMoviePaths, moviePath)
		resolvedMovieSeen[moviePath] = true
	}

	sourceSeen := map[string]bool{}
	destinationSeen := map[string]bool{}
	decisionsBySubtitle := map[string]AIDecision{}
	duplicateDecisionSubtitles := map[string]bool{}
	for _, operation := range merged.Operations {
		sourceSeen[operation.SourcePath] = true
		destinationSeen[operation.DestinationPath] = true
	}

	for _, decision := range output.Decisions {
		if _, ok := decisionsBySubtitle[decision.SubtitlePath]; ok {
			duplicateDecisionSubtitles[decision.SubtitlePath] = true
			delete(decisionsBySubtitle, decision.SubtitlePath)
			continue
		}
		decisionsBySubtitle[decision.SubtitlePath] = decision
	}

	for _, subtitle := range unresolvedCandidates {
		if duplicateDecisionSubtitles[subtitle.Path] {
			setSkip(subtitle.Path, duplicateAISubtitleDecisionSkipReason)
			continue
		}

		decision, ok := decisionsBySubtitle[subtitle.Path]
		if !ok {
			continue
		}

		if decision.Outcome != AIDecisionMatch {
			setSkip(subtitle.Path, formatAISkipReason(decision))
			continue
		}

		if !isValidAIConfidence(decision.Confidence) {
			setSkip(subtitle.Path, formatInvalidAIConfidenceReason(decision.Confidence))
			continue
		}

		if decision.Confidence < minConfidence {
			setSkip(subtitle.Path, fmt.Sprintf("ai confidence %.2f below threshold %.2f", decision.Confidence, minConfidence))
			continue
		}

		if sourceSeen[subtitle.Path] {
			setSkip(subtitle.Path, "subtitle already scheduled for rename")
			continue
		}

		movie, ok := moviesByPath[decision.MatchedMoviePath]
		if !ok {
			setSkip(subtitle.Path, "ai returned a movie path that does not exist in scan results")
			continue
		}

		destinationName := movie.BaseName + subtitle.Extension
		destinationPath := filepath.Join(filepath.Dir(subtitle.Path), destinationName)
		if destinationPath == subtitle.Path {
			recordResolvedMovie(movie.Path)
			removeSkip(subtitle.Path)
			continue
		}
		if destinationSeen[destinationPath] {
			setSkip(subtitle.Path, "ai destination conflicts with an existing rename target")
			continue
		}

		merged.Operations = append(merged.Operations, RenameOperation{
			SourcePath:      subtitle.Path,
			DestinationPath: destinationPath,
			DestinationName: destinationName,
			MoviePath:       movie.Path,
			MatchSource:     "ai",
			Confidence:      decision.Confidence,
			Reason:          decision.Reason,
		})

		sourceSeen[subtitle.Path] = true
		destinationSeen[destinationPath] = true
		removeSkip(subtitle.Path)
	}

	merged.Skips = make([]SkipOperation, 0, len(skipBySource))
	for _, sourcePath := range skipOrder {
		skip, ok := skipBySource[sourcePath]
		if !ok {
			continue
		}
		merged.Skips = append(merged.Skips, skip)
	}

	return merged
}

func formatAISkipReason(decision AIDecision) string {
	if decision.Outcome == AIDecisionNeedsHuman {
		if decision.Reason == "" {
			return "ai requested human review"
		}
		return "ai requested human review: " + decision.Reason
	}
	if decision.Reason == "" {
		return "ai skipped this subtitle"
	}
	return "ai skipped: " + decision.Reason
}

func isValidAIConfidence(confidence float64) bool {
	if math.IsNaN(confidence) || math.IsInf(confidence, 0) {
		return false
	}
	return confidence >= 0 && confidence <= 1
}

func formatInvalidAIConfidenceReason(confidence float64) string {
	return fmt.Sprintf("ai returned invalid confidence value: %v", confidence)
}
