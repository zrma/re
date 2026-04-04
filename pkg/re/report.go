package re

import (
	"encoding/json"
	"fmt"
	"io"
)

type RunSummary struct {
	MoviesTotal         int `json:"movies_total"`
	SubtitlesTotal      int `json:"subtitles_total"`
	PlannedRenames      int `json:"planned_renames"`
	RuleRenames         int `json:"rule_renames"`
	AIRenames           int `json:"ai_renames"`
	Skips               int `json:"skips"`
	UnresolvedMovies    int `json:"unresolved_movies"`
	UnresolvedSubtitles int `json:"unresolved_subtitles"`
}

type RunReport struct {
	TargetPath           string            `json:"target_path"`
	Status               string            `json:"status"`
	Applied              bool              `json:"applied"`
	RequiresConfirmation bool              `json:"requires_confirmation"`
	Summary              RunSummary        `json:"summary"`
	Operations           []RenameOperation `json:"operations"`
	Skips                []SkipOperation   `json:"skips"`
}

func BuildRunReport(targetPath string, scanResult ScanResult, resolution ResolutionResult, plan RenamePlan, applied bool, requiresConfirmation bool) RunReport {
	summary := RunSummary{
		MoviesTotal:         len(scanResult.Movies),
		SubtitlesTotal:      len(scanResult.Subtitles),
		PlannedRenames:      len(plan.Operations),
		Skips:               len(plan.Skips),
		UnresolvedMovies:    countRemainingUnresolvedMovies(resolution, plan),
		UnresolvedSubtitles: countRemainingUnresolvedSubtitles(scanResult, plan),
	}

	for _, operation := range plan.Operations {
		if operation.MatchSource == "ai" {
			summary.AIRenames++
			continue
		}
		summary.RuleRenames++
	}

	status := "canceled"
	if applied {
		status = "applied"
	} else if len(plan.Operations) == 0 {
		if len(plan.Skips) > 0 {
			status = "needs_review"
		} else {
			status = "noop"
		}
	}

	return RunReport{
		TargetPath:           targetPath,
		Status:               status,
		Applied:              applied,
		RequiresConfirmation: requiresConfirmation,
		Summary:              summary,
		Operations:           append([]RenameOperation{}, plan.Operations...),
		Skips:                append([]SkipOperation{}, plan.Skips...),
	}
}

func BuildCanceledRunReport(targetPath string, scanResult ScanResult, resolution ResolutionResult, plan RenamePlan) RunReport {
	report := BuildRunReport(targetPath, scanResult, resolution, plan, false, false)
	report.Summary.UnresolvedMovies += countDeferredResolvedMovies(resolution, plan)
	report.Summary.UnresolvedSubtitles += countDeferredUnresolvedOperations(plan)
	return report
}

func countRemainingUnresolvedSubtitles(scanResult ScanResult, plan RenamePlan) int {
	subtitleSources := map[string]bool{}
	for _, subtitle := range scanResult.Subtitles {
		subtitleSources[subtitle.Path] = true
	}

	remaining := map[string]bool{}
	for _, skip := range plan.Skips {
		if subtitleSources[skip.SourcePath] {
			remaining[skip.SourcePath] = true
		}
	}

	return len(remaining)
}

func countRemainingUnresolvedMovies(resolution ResolutionResult, plan RenamePlan) int {
	resolvedMoviePaths := map[string]bool{}
	for _, moviePath := range plan.ResolvedMoviePaths {
		if moviePath == "" {
			continue
		}
		resolvedMoviePaths[moviePath] = true
	}
	for _, operation := range plan.Operations {
		if operation.MoviePath == "" {
			continue
		}
		resolvedMoviePaths[operation.MoviePath] = true
	}

	remaining := 0
	for _, movie := range resolution.UnresolvedMovies {
		if resolvedMoviePaths[movie.Path] {
			continue
		}
		remaining++
	}

	return remaining
}

func countDeferredResolvedMovies(resolution ResolutionResult, plan RenamePlan) int {
	unresolvedMoviePaths := map[string]bool{}
	for _, movie := range resolution.UnresolvedMovies {
		unresolvedMoviePaths[movie.Path] = true
	}

	deferred := map[string]bool{}
	for _, operation := range plan.Operations {
		if operation.MoviePath == "" || !unresolvedMoviePaths[operation.MoviePath] {
			continue
		}
		if isSameExistingPath(operation.SourcePath, operation.DestinationPath) {
			continue
		}
		deferred[operation.MoviePath] = true
	}

	return len(deferred)
}

func countDeferredUnresolvedOperations(plan RenamePlan) int {
	deferred := 0
	for _, operation := range plan.Operations {
		if isSameExistingPath(operation.SourcePath, operation.DestinationPath) {
			continue
		}
		deferred++
	}
	return deferred
}

func PrintTextSummary(writer io.Writer, report RunReport) {
	fmt.Fprintf(
		writer,
		"Summary: %d renames (rule %d, ai %d), %d skips, unresolved movies %d, unresolved subtitles %d\n",
		report.Summary.PlannedRenames,
		report.Summary.RuleRenames,
		report.Summary.AIRenames,
		report.Summary.Skips,
		report.Summary.UnresolvedMovies,
		report.Summary.UnresolvedSubtitles,
	)
}

func WriteJSONReport(writer io.Writer, report RunReport) error {
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if _, err := writer.Write(encoded); err != nil {
		return err
	}
	_, err = writer.Write([]byte("\n"))
	return err
}
