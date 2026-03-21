package main

import (
	"flag"
	"log"
	"os"

	"github.com/zrma/re/pkg/re"
)

func main() {
	options := re.DefaultRunOptions()
	outputFormat := string(options.OutputFormat)
	targetPath := ""
	flag.StringVar(&targetPath, "t", "", "target path")
	flag.BoolVar(&options.AssumeYes, "yes", false, "apply renames without interactive confirmation")
	flag.StringVar(&outputFormat, "output", outputFormat, "output format: text or json")
	flag.BoolVar(&options.AI.Enabled, "ai-fallback", false, "enable AI fallback for unresolved subtitles")
	flag.StringVar(&options.AI.Model, "ai-model", options.AI.Model, "AI model for codex exec")
	flag.Float64Var(&options.AI.MinConfidence, "ai-min-confidence", options.AI.MinConfidence, "minimum confidence for AI matches")
	flag.DurationVar(&options.AI.Timeout, "ai-timeout", options.AI.Timeout, "timeout for AI fallback")
	flag.StringVar(&options.AI.DebugOutputPath, "ai-debug-output", "", "directory to store AI input/output payloads")

	flag.Parse()

	if targetPath == "" {
		targetPath = "."
	}
	options.OutputFormat = re.OutputFormat(outputFormat)
	if !options.OutputFormat.Valid() {
		log.Fatalf("invalid output format %q (expected text or json)", outputFormat)
	}

	re.RunWithOptions(targetPath, os.Stdin, os.Stdout, options)
}
