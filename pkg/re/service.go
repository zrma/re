package re

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/spf13/afero"
)

var ErrPreviewExpired = errors.New("preview is stale")

type PreviewRequest struct {
	TargetPath string
	Options    RunOptions
}

type PreviewResult struct {
	TargetPath string
	Options    RunOptions
	ScanResult ScanResult
	Resolution ResolutionResult
	Plan       RenamePlan
	Report     RunReport
	Snapshot   DirectorySnapshot
	Warnings   []string
}

type FileSnapshot struct {
	Path    string
	Exists  bool
	Mode    os.FileMode
	Size    int64
	ModTime time.Time
}

type DirectorySnapshot struct {
	Files []FileSnapshot
}

type PreviewExpiredError struct {
	Path   string
	Reason string
}

func (e PreviewExpiredError) Error() string {
	if e.Path == "" {
		return ErrPreviewExpired.Error()
	}
	if e.Reason == "" {
		return fmt.Sprintf("%s: %s", ErrPreviewExpired, e.Path)
	}
	return fmt.Sprintf("%s: %s (%s)", ErrPreviewExpired, e.Path, e.Reason)
}

func (e PreviewExpiredError) Unwrap() error {
	return ErrPreviewExpired
}

func BuildPreview(ctx context.Context, req PreviewRequest) (PreviewResult, error) {
	options := req.Options
	if !options.OutputFormat.Valid() {
		return PreviewResult{}, fmt.Errorf("invalid output format: %s", options.OutputFormat)
	}

	targetPath := normalizeTargetPath(req.TargetPath)

	scanResult, err := ScanDirectory(targetPath)
	if err != nil {
		return PreviewResult{}, err
	}

	resolution := ResolveByRule(scanResult)
	plan := BuildRenamePlan(resolution)
	plan = EnforceSafeRenamePlan(plan, scanResult)
	warnings := []string{}

	unresolvedCandidates := CollectAICandidateSubtitles(scanResult, resolution, plan)
	if options.AI.Enabled && len(unresolvedCandidates) > 0 {
		aiOutput, err := resolveAIOutput(ctx, targetPath, scanResult, plan, unresolvedCandidates, options)
		if err != nil {
			warnings = append(warnings, err.Error())
		} else if aiOutput != nil {
			plan = MergeAIRenamePlan(plan, scanResult, unresolvedCandidates, *aiOutput, options.AI.MinConfidence)
			plan = EnforceSafeRenamePlan(plan, scanResult)
		}
	}

	snapshot, err := buildDirectorySnapshot(scanResult, plan)
	if err != nil {
		return PreviewResult{}, err
	}

	requiresConfirmation := !options.AssumeYes && len(plan.Operations) > 0
	report := BuildRunReport(targetPath, scanResult, resolution, plan, false, requiresConfirmation)

	return PreviewResult{
		TargetPath: targetPath,
		Options:    options,
		ScanResult: scanResult,
		Resolution: resolution,
		Plan:       plan,
		Report:     report,
		Snapshot:   snapshot,
		Warnings:   warnings,
	}, nil
}

func ApplyPreview(preview PreviewResult) (RunReport, error) {
	if err := ValidatePreviewSnapshot(preview); err != nil {
		return RunReport{}, err
	}

	if err := ApplyRenamePlan(preview.Plan); err != nil {
		return RunReport{}, err
	}

	return BuildRunReport(preview.TargetPath, preview.ScanResult, preview.Resolution, preview.Plan, true, false), nil
}

func ValidatePreviewSnapshot(preview PreviewResult) error {
	for _, expected := range preview.Snapshot.Files {
		current, err := captureFileSnapshot(expected.Path)
		if err != nil {
			return fmt.Errorf("capture current snapshot for %s: %w", expected.Path, err)
		}
		if expected.matches(current) {
			continue
		}
		return PreviewExpiredError{
			Path:   expected.Path,
			Reason: describeSnapshotMismatch(expected, current),
		}
	}

	return nil
}

func resolveAIOutput(
	ctx context.Context,
	targetPath string,
	scanResult ScanResult,
	plan RenamePlan,
	unresolvedCandidates []MediaFile,
	options RunOptions,
) (*AIOutput, error) {
	resolver := options.AI.Resolver
	if resolver == nil {
		resolver = CodexExecResolver{
			Model:           options.AI.Model,
			DebugOutputPath: options.AI.DebugOutputPath,
		}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	cancel := func() {}
	if options.AI.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, options.AI.Timeout)
	}
	defer cancel()

	aiInput := BuildAIInput(targetPath, scanResult, plan, unresolvedCandidates)
	aiOutput, err := resolver.Resolve(ctx, aiInput)
	if err != nil {
		return nil, fmt.Errorf("ai fallback failed: %w", err)
	}

	return &aiOutput, nil
}

func buildDirectorySnapshot(scanResult ScanResult, plan RenamePlan) (DirectorySnapshot, error) {
	paths := make(map[string]struct{}, len(scanResult.Movies)+len(scanResult.Subtitles)+len(scanResult.TemporaryArtifacts)+len(plan.Operations)*2)

	for _, movie := range scanResult.Movies {
		paths[movie.Path] = struct{}{}
	}
	for _, subtitle := range scanResult.Subtitles {
		paths[subtitle.Path] = struct{}{}
	}
	for _, artifactPath := range scanResult.TemporaryArtifacts {
		paths[artifactPath] = struct{}{}
	}
	for _, operation := range plan.Operations {
		paths[operation.SourcePath] = struct{}{}
		paths[operation.DestinationPath] = struct{}{}
	}

	sortedPaths := make([]string, 0, len(paths))
	for path := range paths {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	files := make([]FileSnapshot, 0, len(sortedPaths))
	for _, path := range sortedPaths {
		snapshot, err := captureFileSnapshot(path)
		if err != nil {
			return DirectorySnapshot{}, fmt.Errorf("capture preview snapshot for %s: %w", path, err)
		}
		files = append(files, snapshot)
	}

	return DirectorySnapshot{Files: files}, nil
}

func captureFileSnapshot(path string) (FileSnapshot, error) {
	snapshot := FileSnapshot{Path: path}

	if lstater, ok := FileSystem.(afero.Lstater); ok {
		info, _, err := lstater.LstatIfPossible(path)
		if err != nil {
			if os.IsNotExist(err) {
				return snapshot, nil
			}
			return FileSnapshot{}, err
		}
		return snapshotFromFileInfo(path, info), nil
	}

	info, err := FileSystem.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return FileSnapshot{}, err
	}

	return snapshotFromFileInfo(path, info), nil
}

func snapshotFromFileInfo(path string, info os.FileInfo) FileSnapshot {
	return FileSnapshot{
		Path:    path,
		Exists:  true,
		Mode:    info.Mode(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}
}

func (snapshot FileSnapshot) matches(other FileSnapshot) bool {
	if snapshot.Exists != other.Exists {
		return false
	}
	if !snapshot.Exists {
		return true
	}

	return snapshot.Mode == other.Mode &&
		snapshot.Size == other.Size &&
		snapshot.ModTime.Equal(other.ModTime)
}

func describeSnapshotMismatch(expected FileSnapshot, current FileSnapshot) string {
	switch {
	case expected.Exists && !current.Exists:
		return "path disappeared after preview"
	case !expected.Exists && current.Exists:
		return "path appeared after preview"
	case expected.Mode != current.Mode:
		return "path type changed after preview"
	default:
		return "path contents changed after preview"
	}
}

func normalizeTargetPath(targetPath string) string {
	if targetPath == "" {
		return "."
	}
	return filepath.Clean(targetPath)
}
