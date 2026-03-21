package re

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
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
	changeExtToLower(targetPath)

	scanResult, err := ScanDirectory(targetPath)
	if err != nil {
		log.Fatalln(err)
	}

	resolution := ResolveByRule(scanResult)
	plan := BuildRenamePlan(resolution)

	if options.AI.Enabled && len(resolution.UnresolvedSubtitles) > 0 {
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

		aiInput := BuildAIInput(targetPath, scanResult, resolution, plan)
		aiOutput, err := resolver.Resolve(ctx, aiInput)
		if err != nil {
			log.Printf("[ai] fallback failed: %v", err)
		} else {
			plan = MergeAIRenamePlan(plan, scanResult, resolution.UnresolvedSubtitles, aiOutput, options.AI.MinConfidence)
		}
	}

	report := BuildRunReport(targetPath, scanResult, resolution, plan, false, !options.AssumeYes)

	if options.OutputFormat == OutputFormatText {
		PreviewRenamePlan(writer, plan)
		PrintTextSummary(writer, report)
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
		fmt.Fprintln(writer, "Done!")
		return
	}

	promptWriter := writer
	if options.OutputFormat == OutputFormatJSON {
		promptWriter = os.Stderr
	}
	fmt.Fprintln(promptWriter, "Do you want to rename? (y/n)")

	var input string
	_, _ = fmt.Fscanln(reader, &input)
	if strings.ToLower(input) != "y" {
		if options.OutputFormat == OutputFormatJSON {
			if err := WriteJSONReport(writer, report); err != nil {
				log.Fatalln(err)
			}
			return
		}
		fmt.Fprint(writer, "Canceled")
		return
	}

	if err := ApplyRenamePlan(plan); err != nil {
		log.Fatalln(err)
	}
	report = BuildRunReport(targetPath, scanResult, resolution, plan, true, true)
	if options.OutputFormat == OutputFormatJSON {
		if err := WriteJSONReport(writer, report); err != nil {
			log.Fatalln(err)
		}
		return
	}
	fmt.Fprintln(writer, "Done!")
}

func normalizeTargetPath(targetPath string) string {
	return strings.TrimSuffix(targetPath, "\\")
}

func changeExtToLower(targetPath string) {
	movieExtList := map[string]bool{
		"avi": true, "mkv": true, "mp4": true,
		"AVI": true, "MKV": true, "MP4": true,
	}
	subtitleExtList := map[string]bool{
		"srt": true, "ass": true, "smi": true,
		"SRT": true, "ASS": true, "SMI": true,
	}
	err := afero.Walk(FileSystem, targetPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Dir(path) != targetPath {
			return nil
		}

		ext := filepath.Ext(path)
		ext = ext[1:]
		if movieExtList[ext] {
			if ext != strings.ToLower(ext) {
				ext = strings.ToLower(ext)
				newPath := strings.TrimSuffix(path, filepath.Ext(path)) + "." + ext
				err := FileSystem.Rename(path, newPath)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
		if subtitleExtList[ext] {
			if ext != strings.ToLower(ext) {
				ext = strings.ToLower(ext)
				newPath := strings.TrimSuffix(path, filepath.Ext(path)) + "." + ext
				err := FileSystem.Rename(path, newPath)
				if err != nil {
					log.Fatalln(err)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalln(err)
	}
}
