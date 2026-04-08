package re

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var FileSystem = newOSFileSystem()

func Run(targetPath string, reader io.Reader) {
	RunWithOptions(targetPath, reader, os.Stdout, DefaultRunOptions())
}

func RunWithOptions(targetPath string, reader io.Reader, writer io.Writer, options RunOptions) {
	if writer == nil {
		writer = os.Stdout
	}
	if !options.OutputFormat.Valid() {
		log.Fatalf("invalid output format: %s", options.OutputFormat)
	}

	targetPath = normalizeTargetPath(targetPath)

	scanResult, err := ScanDirectory(targetPath)
	if err != nil {
		log.Fatalln(err)
	}

	resolution := ResolveByRule(scanResult)
	plan := EnforceSafeRenamePlan(BuildRenamePlan(resolution), scanResult)
	unresolvedCandidates := CollectAICandidateSubtitles(scanResult, resolution, plan)

	if options.AI.Enabled && len(unresolvedCandidates) > 0 {
		resolver := options.AI.Resolver
		if resolver == nil {
			resolver = CodexExecResolver{
				Model:           options.AI.Model,
				DebugOutputPath: options.AI.DebugOutputPath,
			}
		}

		ctx := context.Background()
		cancel := func() {}
		if options.AI.Timeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, options.AI.Timeout)
		}
		defer cancel()

		aiInput := BuildAIInput(targetPath, scanResult, plan, unresolvedCandidates)
		aiOutput, err := resolver.Resolve(ctx, aiInput)
		if err != nil {
			log.Printf("[ai] fallback failed: %v", err)
		} else {
			plan = MergeAIRenamePlan(plan, scanResult, unresolvedCandidates, aiOutput, options.AI.MinConfidence)
			plan = EnforceSafeRenamePlan(plan, scanResult)
		}
	}

	requiresConfirmation := !options.AssumeYes && len(plan.Operations) > 0
	report := BuildRunReport(targetPath, scanResult, resolution, plan, false, requiresConfirmation)

	if options.OutputFormat == OutputFormatText {
		PreviewRenamePlan(writer, plan)
		PrintTextSummary(writer, report)
	}

	if len(plan.Operations) == 0 {
		if options.OutputFormat == OutputFormatJSON {
			if err := WriteJSONReport(writer, report); err != nil {
				log.Fatalln(err)
			}
			return
		}
		if options.AssumeYes && len(plan.Skips) == 0 {
			_, _ = fmt.Fprintln(writer, "Done!")
		}
		return
	}

	if options.AssumeYes {
		if err := ApplyRenamePlan(plan); err != nil {
			log.Fatalln(err)
		}
		report = BuildRunReport(targetPath, scanResult, resolution, plan, true, false)
		if options.OutputFormat == OutputFormatJSON {
			if err := WriteJSONReport(writer, report); err != nil {
				log.Fatalln(err)
			}
			return
		}
		_, _ = fmt.Fprintln(writer, "Done!")
		return
	}

	promptWriter := writer
	if options.OutputFormat == OutputFormatJSON {
		promptWriter = os.Stderr
	}
	_, _ = fmt.Fprintln(promptWriter, "Do you want to rename? (y/n)")

	var input string
	_, _ = fmt.Fscanln(reader, &input)
	if strings.ToLower(input) != "y" {
		report = BuildCanceledRunReport(targetPath, scanResult, resolution, plan)
		if options.OutputFormat == OutputFormatJSON {
			if err := WriteJSONReport(writer, report); err != nil {
				log.Fatalln(err)
			}
			return
		}
		_, _ = fmt.Fprintln(writer, "Canceled")
		PrintTextSummary(writer, report)
		return
	}

	if err := ApplyRenamePlan(plan); err != nil {
		log.Fatalln(err)
	}
	report = BuildRunReport(targetPath, scanResult, resolution, plan, true, false)
	if options.OutputFormat == OutputFormatJSON {
		if err := WriteJSONReport(writer, report); err != nil {
			log.Fatalln(err)
		}
		return
	}
	_, _ = fmt.Fprintln(writer, "Done!")
}

func normalizeTargetPath(targetPath string) string {
	if targetPath == "" {
		return "."
	}
	return filepath.Clean(targetPath)
}
