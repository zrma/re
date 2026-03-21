package re

import "time"

type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
)

type RunOptions struct {
	AssumeYes    bool
	OutputFormat OutputFormat
	AI           AIOptions
}

type AIOptions struct {
	Enabled         bool
	Model           string
	MinConfidence   float64
	Timeout         time.Duration
	DebugOutputPath string
	Resolver        AIResolver
}

func DefaultRunOptions() RunOptions {
	return RunOptions{
		OutputFormat: OutputFormatText,
		AI: AIOptions{
			Model:         "gpt-5.4-mini",
			MinConfidence: 0.90,
			Timeout:       30 * time.Second,
		},
	}
}

func (f OutputFormat) Valid() bool {
	return f == OutputFormatText || f == OutputFormatJSON
}
