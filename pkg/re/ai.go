package re

import (
	"context"
	"fmt"
	"path/filepath"
)

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

func BuildAIInput(targetPath string, scanResult ScanResult, resolution ResolutionResult, rulePlan RenamePlan) AIInput {
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

	unresolvedSubtitles := make([]string, 0, len(resolution.UnresolvedSubtitles))
	for _, subtitle := range resolution.UnresolvedSubtitles {
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

func MergeAIRenamePlan(rulePlan RenamePlan, scanResult ScanResult, unresolvedSubtitles []MediaFile, output AIOutput, minConfidence float64) RenamePlan {
	merged := RenamePlan{
		Operations: append([]RenameOperation{}, rulePlan.Operations...),
		Skips:      nil,
	}

	moviesByPath := map[string]MediaFile{}
	for _, movie := range scanResult.Movies {
		moviesByPath[movie.Path] = movie
	}

	sourceSeen := map[string]bool{}
	destinationSeen := map[string]bool{}
	decisionsBySubtitle := map[string]AIDecision{}
	for _, operation := range merged.Operations {
		sourceSeen[operation.SourcePath] = true
		destinationSeen[operation.DestinationPath] = true
	}

	for _, decision := range output.Decisions {
		decisionsBySubtitle[decision.SubtitlePath] = decision
	}

	for _, subtitle := range unresolvedSubtitles {
		decision, ok := decisionsBySubtitle[subtitle.Path]
		if !ok {
			merged.Skips = append(merged.Skips, SkipOperation{
				SourcePath: subtitle.Path,
				Reason:     ruleSkipReason + "; ai returned no decision",
			})
			continue
		}

		if decision.Outcome != AIDecisionMatch {
			merged.Skips = append(merged.Skips, SkipOperation{
				SourcePath: subtitle.Path,
				Reason:     formatAISkipReason(decision),
			})
			continue
		}

		if decision.Confidence < minConfidence {
			merged.Skips = append(merged.Skips, SkipOperation{
				SourcePath: subtitle.Path,
				Reason:     fmt.Sprintf("ai confidence %.2f below threshold %.2f", decision.Confidence, minConfidence),
			})
			continue
		}

		if sourceSeen[subtitle.Path] {
			merged.Skips = append(merged.Skips, SkipOperation{
				SourcePath: subtitle.Path,
				Reason:     "subtitle already scheduled for rename",
			})
			continue
		}

		movie, ok := moviesByPath[decision.MatchedMoviePath]
		if !ok {
			merged.Skips = append(merged.Skips, SkipOperation{
				SourcePath: subtitle.Path,
				Reason:     "ai returned a movie path that does not exist in scan results",
			})
			continue
		}

		destinationName := movie.BaseName + subtitle.Extension
		destinationPath := filepath.Join(filepath.Dir(subtitle.Path), destinationName)
		if destinationSeen[destinationPath] {
			merged.Skips = append(merged.Skips, SkipOperation{
				SourcePath: subtitle.Path,
				Reason:     "ai destination conflicts with an existing rename target",
			})
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
