package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

type statFailFS struct {
	afero.Fs
	blockedPath string
	err         error
}

func (fs statFailFS) Stat(name string) (os.FileInfo, error) {
	if name == fs.blockedPath {
		return nil, fs.err
	}
	return fs.Fs.Stat(name)
}

type renameFailFS struct {
	afero.Fs
	blockedDestination string
	err                error
	failOnce           bool
}

type recordedRename struct {
	oldPath string
	newPath string
}

type renameRecorderFS struct {
	afero.Fs
	renames []recordedRename
}

func (fs *renameFailFS) Rename(oldname string, newname string) error {
	if fs.failOnce && newname == fs.blockedDestination {
		fs.failOnce = false
		return fs.err
	}
	return fs.Fs.Rename(oldname, newname)
}

func (fs *renameRecorderFS) Rename(oldname string, newname string) error {
	fs.renames = append(fs.renames, recordedRename{
		oldPath: oldname,
		newPath: newname,
	})
	return fs.Fs.Rename(oldname, newname)
}

func TestRunWithOptionsPrintsTextSummary(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, options)

	assert.Contains(t, output.String(), "Summary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0")
}

func TestRunWithOptionsPrintsJSONReport(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true
	options.OutputFormat = re.OutputFormatJSON

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	var report struct {
		Status  string `json:"status"`
		Applied bool   `json:"applied"`
		Summary struct {
			PlannedRenames int `json:"planned_renames"`
			RuleRenames    int `json:"rule_renames"`
			AIRenames      int `json:"ai_renames"`
			Skips          int `json:"skips"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.Equal(t, "applied", report.Status)
	assert.True(t, report.Applied)
	assert.Equal(t, 1, report.Summary.PlannedRenames)
	assert.Equal(t, 1, report.Summary.RuleRenames)
	assert.Equal(t, 0, report.Summary.AIRenames)
	assert.Equal(t, 0, report.Summary.Skips)
}

func TestRunWithOptionsJSONReportMarksConfirmationHandledAfterApply(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.OutputFormat = re.OutputFormatJSON

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("y\n"), &output, options)

	var report struct {
		Applied              bool `json:"applied"`
		RequiresConfirmation bool `json:"requires_confirmation"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.True(t, report.Applied)
	assert.False(t, report.RequiresConfirmation)
}

func TestRunWithOptionsJSONReportMarksConfirmationHandledAfterCancel(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.OutputFormat = re.OutputFormatJSON

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, options)

	var report struct {
		Status               string `json:"status"`
		Applied              bool   `json:"applied"`
		RequiresConfirmation bool   `json:"requires_confirmation"`
		Summary              struct {
			PlannedRenames      int `json:"planned_renames"`
			UnresolvedSubtitles int `json:"unresolved_subtitles"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.Equal(t, "canceled", report.Status)
	assert.False(t, report.Applied)
	assert.False(t, report.RequiresConfirmation)
	assert.Equal(t, 1, report.Summary.PlannedRenames)
	assert.Equal(t, 1, report.Summary.UnresolvedSubtitles)
}

func TestRunWithOptionsPrintsFinalTextSummaryAfterCancel(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, re.DefaultRunOptions())

	assert.Contains(t, output.String(), "Canceled\nSummary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 1")
}

func TestRunWithOptionsCountsCaseOnlyRenameAsUnresolvedAfterCancelOnCaseSensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).MKV"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).SRT"), []byte("subtitle"), 0644))

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, re.DefaultRunOptions())

	assert.Contains(t, output.String(), "Canceled\nSummary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 1")
}

func TestRunWithOptionsCancelRestoresUnresolvedMovieCountAfterAIMatch(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "Special Feature.mkv")
	subtitlePath := filepath.Join(basePath, "special-kor.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AI.Enabled = true
	options.OutputFormat = re.OutputFormatJSON
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     subtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: moviePath,
					Confidence:       0.96,
					Reason:           "subtitle belongs to the unresolved special feature",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, options)

	var report struct {
		Status  string `json:"status"`
		Summary struct {
			UnresolvedMovies    int `json:"unresolved_movies"`
			UnresolvedSubtitles int `json:"unresolved_subtitles"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.Equal(t, "canceled", report.Status)
	assert.Equal(t, 1, report.Summary.UnresolvedMovies)
	assert.Equal(t, 1, report.Summary.UnresolvedSubtitles)
}

func TestRunWithOptionsNormalizesTrailingSlashTargetPath(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath+string(os.PathSeparator), strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.NoError(t, err)
}

func TestRunWithOptionsSkipsConflictingRenameTargets(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "01.srt"), []byte("subtitle2"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "1화.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "01.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.Error(t, err)
	assert.Contains(t, output.String(), "rename target conflicts with another planned rename")
	assert.Contains(t, output.String(), "unresolved subtitles 2")
}

func TestRunWithOptionsSkipsRenameWhenDestinationInspectionFails(t *testing.T) {
	baseFS := afero.NewMemMapFs()
	re.FileSystem = statFailFS{
		Fs:          baseFS,
		blockedPath: filepath.Join("home", "folder", "JohnDoe", "Downloads", "[BD] Example - 01 (BD).srt"),
		err:         errors.New("permission denied"),
	}
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	sourceSubtitlePath := filepath.Join(basePath, "1화.srt")
	destinationSubtitlePath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(baseFS, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(baseFS, sourceSubtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := baseFS.Stat(sourceSubtitlePath)
	assert.NoError(t, err)
	_, err = baseFS.Stat(destinationSubtitlePath)
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[skip] "+sourceSubtitlePath+" (rename target could not be inspected safely)")
	assert.Contains(t, output.String(), "unresolved subtitles 1")
}

func TestApplyRenamePlanRollsBackCompletedRenamesOnFailure(t *testing.T) {
	baseFS := afero.NewMemMapFs()
	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	failingDestinationPath := filepath.Join(basePath, "[BD] Example - 02 (BD).srt")
	re.FileSystem = &renameFailFS{
		Fs:                 baseFS,
		blockedDestination: failingDestinationPath,
		err:                errors.New("simulated rename failure"),
		failOnce:           true,
	}
	defer func() { re.FileSystem = afero.NewOsFs() }()

	firstSourcePath := filepath.Join(basePath, "1화.srt")
	secondSourcePath := filepath.Join(basePath, "2화.srt")
	firstDestinationPath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(baseFS, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie-1"), 0644))
	require.NoError(t, afero.WriteFile(baseFS, filepath.Join(basePath, "[BD] Example - 02 (BD).mkv"), []byte("movie-2"), 0644))
	require.NoError(t, afero.WriteFile(baseFS, firstSourcePath, []byte("subtitle-1"), 0644))
	require.NoError(t, afero.WriteFile(baseFS, secondSourcePath, []byte("subtitle-2"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.EnforceSafeRenamePlan(re.BuildRenamePlan(resolution), scanResult)

	err = re.ApplyRenamePlan(plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), secondSourcePath)
	assert.Contains(t, err.Error(), failingDestinationPath)

	firstContent, err := afero.ReadFile(baseFS, firstSourcePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-1", string(firstContent))

	secondContent, err := afero.ReadFile(baseFS, secondSourcePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-2", string(secondContent))

	_, err = baseFS.Stat(firstDestinationPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
	_, err = baseFS.Stat(failingDestinationPath)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestApplyRenamePlanRollsBackTemporaryCycleBreakOnFailure(t *testing.T) {
	baseFS := afero.NewMemMapFs()
	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	aPath := filepath.Join(basePath, "a.srt")
	bPath := filepath.Join(basePath, "b.srt")
	re.FileSystem = &renameFailFS{
		Fs:                 baseFS,
		blockedDestination: bPath,
		err:                errors.New("simulated cycle rename failure"),
		failOnce:           true,
	}
	defer func() { re.FileSystem = afero.NewOsFs() }()

	require.NoError(t, afero.WriteFile(baseFS, aPath, []byte("subtitle-a"), 0644))
	require.NoError(t, afero.WriteFile(baseFS, bPath, []byte("subtitle-b"), 0644))

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      aPath,
				DestinationPath: bPath,
				DestinationName: filepath.Base(bPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      bPath,
				DestinationPath: aPath,
				DestinationName: filepath.Base(aPath),
				MatchSource:     "ai",
			},
		},
	}

	err := re.ApplyRenamePlan(plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), bPath)

	aContent, err := afero.ReadFile(baseFS, aPath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-a", string(aContent))

	bContent, err := afero.ReadFile(baseFS, bPath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-b", string(bContent))

	_, err = baseFS.Stat(filepath.Join(basePath, ".a.re-tmp-0.srt"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestApplyRenamePlanHandlesThreeWayCycle(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	aPath := filepath.Join(basePath, "a.srt")
	bPath := filepath.Join(basePath, "b.srt")
	cPath := filepath.Join(basePath, "c.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, aPath, []byte("subtitle-a"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, bPath, []byte("subtitle-b"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, cPath, []byte("subtitle-c"), 0644))

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      aPath,
				DestinationPath: bPath,
				DestinationName: filepath.Base(bPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      bPath,
				DestinationPath: cPath,
				DestinationName: filepath.Base(cPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      cPath,
				DestinationPath: aPath,
				DestinationName: filepath.Base(aPath),
				MatchSource:     "ai",
			},
		},
	}

	require.NoError(t, re.ApplyRenamePlan(plan))

	aContent, err := afero.ReadFile(re.FileSystem, aPath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-c", string(aContent))

	bContent, err := afero.ReadFile(re.FileSystem, bPath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-a", string(bContent))

	cContent, err := afero.ReadFile(re.FileSystem, cPath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-b", string(cContent))

	_, err = re.FileSystem.Stat(filepath.Join(basePath, ".a.re-tmp-0.srt"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestApplyRenamePlanDoesNotOverwriteDestinationCreatedAfterPlanning(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "1화.srt")
	destinationPath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, sourcePath, []byte("planned-source"), 0644))

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      sourcePath,
				DestinationPath: destinationPath,
				DestinationName: filepath.Base(destinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
		},
	}

	require.NoError(t, afero.WriteFile(re.FileSystem, destinationPath, []byte("late-destination"), 0644))

	err := re.ApplyRenamePlan(plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), sourcePath)
	assert.Contains(t, err.Error(), destinationPath)
	assert.Contains(t, err.Error(), "destination already exists at apply time")

	sourceContent, err := afero.ReadFile(re.FileSystem, sourcePath)
	require.NoError(t, err)
	assert.Equal(t, "planned-source", string(sourceContent))

	destinationContent, err := afero.ReadFile(re.FileSystem, destinationPath)
	require.NoError(t, err)
	assert.Equal(t, "late-destination", string(destinationContent))
}

func TestApplyRenamePlanDoesNotOverwriteBrokenSymlinkDestinationCreatedAfterPlanning(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "1화.srt")
	destinationPath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, sourcePath, []byte("planned-source"), 0644))

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      sourcePath,
				DestinationPath: destinationPath,
				DestinationName: filepath.Base(destinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
		},
	}

	require.NoError(t, os.Symlink("missing-target", destinationPath))

	err := re.ApplyRenamePlan(plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), sourcePath)
	assert.Contains(t, err.Error(), destinationPath)
	assert.Contains(t, err.Error(), "destination already exists at apply time")

	sourceContent, err := afero.ReadFile(re.FileSystem, sourcePath)
	require.NoError(t, err)
	assert.Equal(t, "planned-source", string(sourceContent))

	linkTarget, err := os.Readlink(destinationPath)
	require.NoError(t, err)
	assert.Equal(t, "missing-target", linkTarget)
}

func TestApplyRenamePlanDirectlyRenamesCaseOnlyPathWithoutTemporaryStep(t *testing.T) {
	baseFS := afero.NewOsFs()
	recordingFS := &renameRecorderFS{Fs: baseFS}
	re.FileSystem = recordingFS
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "[BD] Example - 01 (BD).SRT")
	destinationPath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(baseFS, sourcePath, []byte("subtitle"), 0644))

	sourceInfo, err := baseFS.Stat(sourcePath)
	require.NoError(t, err)
	destinationInfo, err := baseFS.Stat(destinationPath)
	if err != nil || !os.SameFile(sourceInfo, destinationInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase subtitle paths")
	}

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      sourcePath,
				DestinationPath: destinationPath,
				DestinationName: filepath.Base(destinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
		},
	}

	require.NoError(t, re.ApplyRenamePlan(plan))
	require.Len(t, recordingFS.renames, 1)
	assert.Equal(t, sourcePath, recordingFS.renames[0].oldPath)
	assert.Equal(t, destinationPath, recordingFS.renames[0].newPath)
}

func TestRunWithOptionsReportsLeftoverTemporaryArtifacts(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	artifactPath := filepath.Join(basePath, ".1화.re-tmp-0.srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, artifactPath, []byte("temp-subtitle"), 0644))

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, re.DefaultRunOptions())

	assert.Contains(t, output.String(), "[skip] "+artifactPath+" (leftover internal temporary rename artifact detected)")
	assert.Contains(t, output.String(), "Summary: 0 renames (rule 0, ai 0), 1 skips, unresolved movies 0, unresolved subtitles 0")
}

func TestRunWithOptionsIgnoresInternalAliasProbeArtifacts(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	oldProbePath := filepath.Join(basePath, ".re-fs-probe-a.re-tmp-0.tmp")
	newProbePath := filepath.Join(basePath, ".re-fs-probe-a.re-probe-0.tmp")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, oldProbePath, []byte("old-probe"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, newProbePath, []byte("new-probe"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "1화.srt"))
	assert.Error(t, err)
	assert.NotContains(t, output.String(), "[skip] "+oldProbePath)
	assert.NotContains(t, output.String(), "[skip] "+newProbePath)
	assert.Contains(t, output.String(), "Summary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0")
}

func TestRunWithOptionsJSONReportUsesNeedsReviewStatusWhenOnlySkipsRemain(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	artifactPath := filepath.Join(basePath, ".1화.re-tmp-0.srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, artifactPath, []byte("temp-subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.OutputFormat = re.OutputFormatJSON

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	var report struct {
		Status  string `json:"status"`
		Summary struct {
			PlannedRenames int `json:"planned_renames"`
			Skips          int `json:"skips"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.Equal(t, "needs_review", report.Status)
	assert.Equal(t, 0, report.Summary.PlannedRenames)
	assert.Equal(t, 1, report.Summary.Skips)
}

func TestRunWithOptionsAssumeYesDoesNotPrintDoneWhenOnlySkipsRemain(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	artifactPath := filepath.Join(basePath, ".1화.re-tmp-0.srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, artifactPath, []byte("temp-subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	assert.Contains(t, output.String(), "[skip] "+artifactPath+" (leftover internal temporary rename artifact detected)")
	assert.NotContains(t, output.String(), "Done!")
}

func TestRunWithOptionsSkipsDuplicateTargetWhenCaseOnlyPathIsDistinctFile(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	normalizedSubtitlePath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")
	uppercaseSubtitlePath := filepath.Join(basePath, "[BD] Example - 01 (BD).SRT")
	conflictingSubtitlePath := filepath.Join(basePath, "1화.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, uppercaseSubtitlePath, []byte("subtitle1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, conflictingSubtitlePath, []byte("subtitle2"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	content, err := afero.ReadFile(re.FileSystem, normalizedSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle1", string(content))
	_, err = re.FileSystem.Stat(uppercaseSubtitlePath)
	assert.Error(t, err)
	_, err = re.FileSystem.Stat(conflictingSubtitlePath)
	assert.NoError(t, err)
	assert.NotContains(t, output.String(), "[skip] "+uppercaseSubtitlePath)
	assert.Contains(t, output.String(), uppercaseSubtitlePath+" -> [BD] Example - 01 (BD).srt")
	assert.Contains(t, output.String(), "[skip] "+conflictingSubtitlePath+" (rename target conflicts with another planned rename)")
	assert.Contains(t, output.String(), "Summary: 1 renames (rule 1, ai 0), 1 skips, unresolved movies 0, unresolved subtitles 1")
}

func TestRunWithOptionsSkipsDuplicateTargetWhenMultipleCanonicalCaseVariantsExist(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	firstCanonicalPath := filepath.Join(basePath, "[BD] Example - 01 (BD).SRT")
	secondCanonicalPath := filepath.Join(basePath, "[bd] example - 01 (bd).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, firstCanonicalPath, []byte("subtitle1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, secondCanonicalPath, []byte("subtitle2"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(firstCanonicalPath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(secondCanonicalPath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[skip] "+firstCanonicalPath+" (rename target conflicts with another planned rename)")
	assert.Contains(t, output.String(), "[skip] "+secondCanonicalPath+" (rename target conflicts with another planned rename)")
	assert.Contains(t, output.String(), "Summary: 0 renames (rule 0, ai 0), 2 skips, unresolved movies 0, unresolved subtitles 2")
}

func TestRunWithOptionsSkipsDuplicateTargetWhenUnicodeCaseFoldCanonicalSubtitleExists(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	moviePath := filepath.Join(basePath, "[BD] ΟΣ - 01 (BD).mkv")
	canonicalSourcePath := filepath.Join(basePath, "[BD] ος - 01 (BD).srt")
	destinationPath := filepath.Join(basePath, "[BD] ΟΣ - 01 (BD).srt")
	conflictingSubtitlePath := filepath.Join(basePath, "1화.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, canonicalSourcePath, []byte("canonical"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, conflictingSubtitlePath, []byte("candidate"), 0644))

	sourceInfo, err := re.FileSystem.Stat(canonicalSourcePath)
	require.NoError(t, err)
	destinationInfo, err := re.FileSystem.Stat(destinationPath)
	if err != nil || !os.SameFile(sourceInfo, destinationInfo) {
		t.Skip("filesystem does not alias unicode case-fold subtitle paths")
	}

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	content, err := afero.ReadFile(re.FileSystem, destinationPath)
	require.NoError(t, err)
	assert.Equal(t, "canonical", string(content))
	_, err = re.FileSystem.Stat(conflictingSubtitlePath)
	assert.NoError(t, err)
	assert.Contains(t, output.String(), canonicalSourcePath+" -> [BD] ΟΣ - 01 (BD).srt")
	assert.Contains(t, output.String(), "[skip] "+conflictingSubtitlePath+" (rename target conflicts with another planned rename)")
}

func TestRunWithOptionsSkipsDuplicateTargetWhenNormalizationEquivalentCanonicalSubtitleExists(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	moviePath := filepath.Join(basePath, "[BD] Café - 01 (BD).mkv")
	canonicalSourcePath := filepath.Join(basePath, "[BD] Cafe\u0301 - 01 (BD).srt")
	destinationPath := filepath.Join(basePath, "[BD] Café - 01 (BD).srt")
	conflictingSubtitlePath := filepath.Join(basePath, "1화.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, canonicalSourcePath, []byte("canonical"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, conflictingSubtitlePath, []byte("candidate"), 0644))

	sourceInfo, err := re.FileSystem.Stat(canonicalSourcePath)
	require.NoError(t, err)
	destinationInfo, err := re.FileSystem.Stat(destinationPath)
	if err != nil || !os.SameFile(sourceInfo, destinationInfo) {
		t.Skip("filesystem does not alias normalization-equivalent subtitle paths")
	}

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	content, err := afero.ReadFile(re.FileSystem, destinationPath)
	require.NoError(t, err)
	assert.Equal(t, "canonical", string(content))
	_, err = re.FileSystem.Stat(conflictingSubtitlePath)
	assert.NoError(t, err)
	assert.Contains(t, output.String(), canonicalSourcePath+" -> [BD] Café - 01 (BD).srt")
	assert.Contains(t, output.String(), "[skip] "+conflictingSubtitlePath+" (rename target conflicts with another planned rename)")
}

func TestRunWithOptionsSkipsSubtitleWithoutMatchingMovie(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "2화.srt"), []byte("subtitle2"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "2화.srt"))
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "no unique movie matched extracted episode")
	assert.Contains(t, output.String(), "unresolved subtitles 1")
}

func TestRunWithOptionsSkipsAmbiguousMovieEpisode(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[WEB] Example - 01 (WEB).mkv"), []byte("movie2"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "1화.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.Error(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[WEB] Example - 01 (WEB).srt"))
	assert.Error(t, err)
	assert.Contains(t, output.String(), "multiple movies matched same episode")
	assert.Contains(t, output.String(), "unresolved subtitles 1")
}

func TestRunWithOptionsUsesAIResolverForAmbiguousMovieEpisode(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	mainMoviePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).mkv")
	specialMoviePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).mkv")
	subtitlePath := filepath.Join(basePath, "Special 1화.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, mainMoviePath, []byte("main"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialMoviePath, []byte("special"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     subtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: specialMoviePath,
					Confidence:       0.96,
					Reason:           "subtitle name matches special release",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "[BD] Special Story - 01 (BD).srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(subtitlePath)
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[ai:0.96] "+subtitlePath+" -> [BD] Special Story - 01 (BD).srt")
	assert.NotContains(t, output.String(), "[skip] "+subtitlePath)
	assert.Contains(t, output.String(), "unresolved movies 1")
	assert.Contains(t, output.String(), "unresolved subtitles 0")
}

func TestEnforceSafeRenamePlanSkipsExistingDestinationDirectory(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))
	require.NoError(t, re.FileSystem.MkdirAll(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"), 0755))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)
	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	require.Len(t, safePlan.Operations, 0)
	require.Len(t, safePlan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "1화.srt"), safePlan.Skips[0].SourcePath)
	assert.Equal(t, "rename target already exists on disk", safePlan.Skips[0].Reason)
}

func TestRunWithOptionsSkipsBrokenSymlinkDestination(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "1화.srt")
	destinationPath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, sourcePath, []byte("subtitle"), 0644))
	require.NoError(t, os.Symlink("missing-target", destinationPath))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(sourcePath)
	assert.NoError(t, err)

	linkTarget, err := os.Readlink(destinationPath)
	require.NoError(t, err)
	assert.Equal(t, "missing-target", linkTarget)

	assert.Contains(t, output.String(), "[skip] "+sourcePath+" (rename target already exists as another subtitle)")
	assert.Contains(t, output.String(), "Summary: 0 renames (rule 0, ai 0), 1 skips, unresolved movies 0, unresolved subtitles 1")
}

func TestRunWithOptionsKeepsRuleSkipsWhenAIHandlesOtherSubtitle(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	exampleMoviePath := filepath.Join(basePath, "[BD] Example - 01 (BD).mkv")
	specialMoviePath := filepath.Join(basePath, "[BD] Special OAD.mkv")
	unmatchedSubtitlePath := filepath.Join(basePath, "2화.srt")
	aiSubtitlePath := filepath.Join(basePath, "special-kor.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, exampleMoviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialMoviePath, []byte("special"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, unmatchedSubtitlePath, []byte("subtitle1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, aiSubtitlePath, []byte("subtitle2"), 0644))

	options := re.DefaultRunOptions()
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     aiSubtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: specialMoviePath,
					Confidence:       0.95,
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, options)

	assert.Contains(t, output.String(), "[skip] "+unmatchedSubtitlePath+" (no unique movie matched extracted episode)")
	assert.Contains(t, output.String(), "[ai:0.95] "+aiSubtitlePath+" -> [BD] Special OAD.srt")
}

func TestRunWithOptionsCancelDoesNotMutateUppercaseExtensions(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	originalMoviePath := filepath.Join(basePath, "[BD] Example - 01 (BD).MKV")
	originalSubtitlePath := filepath.Join(basePath, "1화.SRT")

	require.NoError(t, afero.WriteFile(re.FileSystem, originalMoviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, originalSubtitlePath, []byte("subtitle"), 0644))

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, re.DefaultRunOptions())

	_, err := re.FileSystem.Stat(originalMoviePath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(originalSubtitlePath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.Error(t, err)
	assert.Contains(t, output.String(), "Summary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0")
	assert.Contains(t, output.String(), "Canceled\nSummary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 1")
}

func TestRunWithOptionsAppliesCaseOnlyExtensionNormalizationOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	movieName := "[BD] Example - 01 (BD).mkv"
	originalSubtitleName := "[BD] Example - 01 (BD).SRT"
	normalizedSubtitleName := "[BD] Example - 01 (BD).srt"

	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, movieName), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, originalSubtitleName), []byte("subtitle"), 0644))

	sourceInfo, err := re.FileSystem.Stat(filepath.Join(basePath, originalSubtitleName))
	require.NoError(t, err)
	destinationInfo, err := re.FileSystem.Stat(filepath.Join(basePath, normalizedSubtitleName))
	if err != nil || !os.SameFile(sourceInfo, destinationInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase extension paths")
	}

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	entries, err := os.ReadDir(basePath)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}

	assert.Contains(t, names, movieName)
	assert.Contains(t, names, normalizedSubtitleName)
	assert.NotContains(t, names, originalSubtitleName)
	assert.Contains(t, output.String(), "Summary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0")
}

func TestRunWithOptionsCancelKeepsCaseOnlyNormalizationOutOfUnresolvedCountOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	movieName := "[BD] Example - 01 (BD).mkv"
	originalSubtitleName := "[BD] Example - 01 (BD).SRT"
	normalizedSubtitleName := "[BD] Example - 01 (BD).srt"

	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, movieName), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, originalSubtitleName), []byte("subtitle"), 0644))

	sourceInfo, err := re.FileSystem.Stat(filepath.Join(basePath, originalSubtitleName))
	require.NoError(t, err)
	destinationInfo, err := re.FileSystem.Stat(filepath.Join(basePath, normalizedSubtitleName))
	if err != nil || !os.SameFile(sourceInfo, destinationInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase extension paths")
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, re.DefaultRunOptions())

	assert.Contains(t, output.String(), "Canceled\nSummary: 1 renames (rule 1, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0")
}

func TestRunWithOptionsDoesNotPlanRenameForAlreadyMatchingSubtitle(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	subtitlePath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, re.DefaultRunOptions())

	assert.NotContains(t, output.String(), subtitlePath+" -> [BD] Example - 01 (BD).srt")
	assert.Contains(t, output.String(), "Summary: 0 renames (rule 0, ai 0), 0 skips, unresolved movies 0, unresolved subtitles 0")
	assert.NotContains(t, output.String(), "Do you want to rename?")
	assert.NotContains(t, output.String(), "Canceled")
}

func TestRunWithOptionsJSONReportDoesNotRequireConfirmationWhenNothingToRename(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	subtitlePath := filepath.Join(basePath, "[BD] Example - 01 (BD).srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.OutputFormat = re.OutputFormatJSON

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("ignored\n"), &output, options)

	var report struct {
		Status               string `json:"status"`
		Applied              bool   `json:"applied"`
		RequiresConfirmation bool   `json:"requires_confirmation"`
		Summary              struct {
			PlannedRenames int `json:"planned_renames"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.Equal(t, "noop", report.Status)
	assert.False(t, report.Applied)
	assert.False(t, report.RequiresConfirmation)
	assert.Equal(t, 0, report.Summary.PlannedRenames)
}
