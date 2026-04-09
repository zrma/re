package re

import (
	"fmt"
	"io"
	"log"
	"os"
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
	preview, err := BuildPreview(nil, PreviewRequest{
		TargetPath: targetPath,
		Options:    options,
	})
	if err != nil {
		log.Fatalln(err)
	}
	for _, warning := range preview.Warnings {
		log.Printf("[ai] %s", warning)
	}
	report := preview.Report

	if options.OutputFormat == OutputFormatText {
		PreviewRenamePlan(writer, preview.Plan)
		PrintTextSummary(writer, report)
	}

	if len(preview.Plan.Operations) == 0 {
		if options.OutputFormat == OutputFormatJSON {
			if err := WriteJSONReport(writer, report); err != nil {
				log.Fatalln(err)
			}
			return
		}
		if options.AssumeYes && len(preview.Plan.Skips) == 0 {
			_, _ = fmt.Fprintln(writer, "Done!")
		}
		return
	}

	if options.AssumeYes {
		report, err = ApplyPreview(preview)
		if err != nil {
			log.Fatalln(err)
		}
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
		report = BuildCanceledRunReport(preview.TargetPath, preview.ScanResult, preview.Resolution, preview.Plan)
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

	report, err = ApplyPreview(preview)
	if err != nil {
		log.Fatalln(err)
	}
	if options.OutputFormat == OutputFormatJSON {
		if err := WriteJSONReport(writer, report); err != nil {
			log.Fatalln(err)
		}
		return
	}
	_, _ = fmt.Fprintln(writer, "Done!")
}
