package re

import (
	"fmt"
	"io"
	"path/filepath"
)

const ruleSkipReason = "episode pattern not recognized by rule matcher"

type RenameOperation struct {
	Episode         string  `json:"episode,omitempty"`
	SourcePath      string  `json:"source_path"`
	DestinationPath string  `json:"destination_path"`
	DestinationName string  `json:"destination_name"`
	MoviePath       string  `json:"movie_path,omitempty"`
	MatchSource     string  `json:"match_source"`
	Confidence      float64 `json:"confidence,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}

type SkipOperation struct {
	SourcePath string `json:"source_path"`
	Reason     string `json:"reason"`
}

type RenamePlan struct {
	Operations []RenameOperation
	Skips      []SkipOperation
}

func BuildRenamePlan(resolution ResolutionResult) RenamePlan {
	var plan RenamePlan

	for _, episode := range resolution.Episodes {
		movie := resolution.MoviesByEpisode[episode]
		subs := resolution.SubtitlesByEpisode[episode]

		for _, sub := range subs {
			newName := movie.BaseName + sub.Extension
			plan.Operations = append(plan.Operations, RenameOperation{
				Episode:         episode,
				SourcePath:      sub.Path,
				DestinationPath: filepath.Join(filepath.Dir(sub.Path), newName),
				DestinationName: newName,
				MoviePath:       movie.Path,
				MatchSource:     "rule",
				Confidence:      1,
			})
		}
	}

	for _, sub := range resolution.UnresolvedSubtitles {
		plan.Skips = append(plan.Skips, SkipOperation{
			SourcePath: sub.Path,
			Reason:     ruleSkipReason,
		})
	}

	return plan
}

func PreviewRenamePlan(writer io.Writer, plan RenamePlan) {
	for _, operation := range plan.Operations {
		if operation.MatchSource == "ai" {
			fmt.Fprintf(writer, "[ai:%.2f] %s -> %s\n", operation.Confidence, operation.SourcePath, operation.DestinationName)
			continue
		}
		fmt.Fprintf(writer, "%s -> %s\n", operation.SourcePath, operation.DestinationName)
	}

	for _, skip := range plan.Skips {
		fmt.Fprintf(writer, "[skip] %s (%s)\n", skip.SourcePath, skip.Reason)
	}
}

func ApplyRenamePlan(plan RenamePlan) error {
	for _, operation := range plan.Operations {
		if err := FileSystem.Rename(operation.SourcePath, operation.DestinationPath); err != nil {
			return err
		}
	}
	return nil
}
