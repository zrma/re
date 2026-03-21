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
		UnresolvedMovies:    len(resolution.UnresolvedMovies),
		UnresolvedSubtitles: len(resolution.UnresolvedSubtitles),
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
