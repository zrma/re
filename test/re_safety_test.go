package test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

func TestResolveByRuleDoesNotTreatBroadcastAsOADMovie(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "BROADCAST TITLE.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "special OAD.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, resolution.UnresolvedMovies, 1)
	assert.Equal(t, filepath.Join(basePath, "BROADCAST TITLE.mkv"), resolution.UnresolvedMovies[0].Path)
	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "special OAD.srt"), plan.Skips[0].SourcePath)
	assert.Equal(t, "no unique movie matched extracted episode", plan.Skips[0].Reason)
}

func TestScanDirectoryIgnoresInternalTemporaryRenameArtifacts(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	artifactPath := filepath.Join(basePath, ".1화.re-tmp-0.srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, artifactPath, []byte("temp-subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	require.Len(t, scanResult.Movies, 1)
	assert.Empty(t, scanResult.Subtitles)
	assert.Equal(t, []string{artifactPath}, scanResult.TemporaryArtifacts)
}

func TestScanDirectoryIgnoresInternalAliasProbeArtifacts(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	oldProbePath := filepath.Join(basePath, ".re-fs-probe-a.re-tmp-0.tmp")
	newProbePath := filepath.Join(basePath, ".re-fs-probe-a.re-probe-0.tmp")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, oldProbePath, []byte("old-probe"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, newProbePath, []byte("new-probe"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	require.Len(t, scanResult.Movies, 1)
	assert.Empty(t, scanResult.Subtitles)
	assert.Empty(t, scanResult.TemporaryArtifacts)
}

func TestRunWithOptionsDoesNotTreatTrailingResolutionAsEpisodeNumber(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 80 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Unrelated subtitle 1080.ass"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "Unrelated subtitle 1080.ass"), plan.Skips[0].SourcePath)
	assert.Equal(t, "episode pattern not recognized by rule matcher", plan.Skips[0].Reason)
}

func TestRunWithOptionsDoesNotTreatVersionSuffixAsEpisodeNumber(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Commentary v2.01.ass"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "Commentary v2.01.ass"), plan.Skips[0].SourcePath)
	assert.Equal(t, "episode pattern not recognized by rule matcher", plan.Skips[0].Reason)
}

func TestRunWithOptionsStillSupportsDotSeparatedEpisodeSuffix(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example.01.ass"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, plan.Operations, 1)
	assert.Equal(t, filepath.Join(basePath, "Example.01.ass"), plan.Operations[0].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "[BD] Example - 01 (BD).ass"), plan.Operations[0].DestinationPath)
}

func TestRunWithOptionsSupportsDotSeparatedMovieEpisodeSuffix(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example.01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "sub.01.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	re.RunWithOptions(basePath, strings.NewReader(""), io.Discard, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "Example.01.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "sub.01.srt"))
	assert.Error(t, err)
}

func TestRunWithOptionsSupportsSeasonXAtStartOfFileName(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "10x01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	re.RunWithOptions(basePath, strings.NewReader(""), io.Discard, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "10x01.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "1화.srt"))
	assert.Error(t, err)
}

func TestRunWithOptionsSupportsLowercaseSeasonUnderscorePattern(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "show s01_e01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "sub s01_e01.ass"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	re.RunWithOptions(basePath, strings.NewReader(""), io.Discard, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "show s01_e01.ass"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "sub s01_e01.ass"))
	assert.Error(t, err)
}

func TestRunWithOptionsSupportsEpisodeTokenAtStartOfFileName(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "show E01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "E01.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	re.RunWithOptions(basePath, strings.NewReader(""), io.Discard, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "show E01.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "E01.srt"))
	assert.Error(t, err)
}

func TestRunWithOptionsSupportsVersionedEpisodeTokens(t *testing.T) {
	testCases := []struct {
		name         string
		movieName    string
		subtitleName string
		wantName     string
	}{
		{
			name:         "episodeTokenWithVersionSuffix",
			movieName:    "show E01v2.mkv",
			subtitleName: "1화.srt",
			wantName:     "show E01v2.srt",
		},
		{
			name:         "seasonXWithVersionSuffix",
			movieName:    "show 10x01v2.mkv",
			subtitleName: "1화.ass",
			wantName:     "show 10x01v2.ass",
		},
		{
			name:         "seasonUnderscoreWithVersionSuffix",
			movieName:    "show s01_e01v2.mkv",
			subtitleName: "1화.smi",
			wantName:     "show s01_e01v2.smi",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			re.FileSystem = afero.NewMemMapFs()
			defer func() { re.FileSystem = afero.NewOsFs() }()

			basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
			require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, tt.movieName), []byte("movie"), 0644))
			require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, tt.subtitleName), []byte("subtitle"), 0644))

			options := re.DefaultRunOptions()
			options.AssumeYes = true

			re.RunWithOptions(basePath, strings.NewReader(""), io.Discard, options)

			_, err := re.FileSystem.Stat(filepath.Join(basePath, tt.wantName))
			assert.NoError(t, err)
			_, err = re.FileSystem.Stat(filepath.Join(basePath, tt.subtitleName))
			assert.Error(t, err)
		})
	}
}

func TestScanDirectoryRejectsFileTargetPath(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	filePath := filepath.Join(basePath, "1화.srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, filePath, []byte("subtitle"), 0644))

	_, err := re.ScanDirectory(filePath)
	require.Error(t, err)
}

func TestRunWithOptionsDoesNotTreatKoreanQualityLabelAsEpisodeNumber(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 80 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Unrelated subtitle 1080화질.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "Unrelated subtitle 1080화질.srt"), plan.Skips[0].SourcePath)
	assert.Equal(t, "episode pattern not recognized by rule matcher", plan.Skips[0].Reason)
}

func TestRunWithOptionsDoesNotTreatNonEpisodeBonusTracksAsEpisodeSubtitles(t *testing.T) {
	testCases := []struct {
		name         string
		subtitleName string
	}{
		{
			name:         "NCOP",
			subtitleName: "NCOP 01.ass",
		},
		{
			name:         "ED",
			subtitleName: "ED 01.ass",
		},
		{
			name:         "NCOPWithNumericSuffix",
			subtitleName: "NCOP01.ass",
		},
		{
			name:         "OADNCOPWithNumericSuffix",
			subtitleName: "Example OAD NCOP01.ass",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			re.FileSystem = afero.NewMemMapFs()
			defer func() { re.FileSystem = afero.NewOsFs() }()

			basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
			require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
			require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, tt.subtitleName), []byte("subtitle"), 0644))

			scanResult, err := re.ScanDirectory(basePath)
			require.NoError(t, err)

			resolution := re.ResolveByRule(scanResult)
			plan := re.BuildRenamePlan(resolution)

			assert.Empty(t, plan.Operations)
			require.Len(t, plan.Skips, 1)
			assert.Equal(t, filepath.Join(basePath, tt.subtitleName), plan.Skips[0].SourcePath)
			assert.Equal(t, "episode pattern not recognized by rule matcher", plan.Skips[0].Reason)
		})
	}
}

func TestResolveByRuleDoesNotTreatOADBonusVideoAsOADMovie(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD NCOP01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "special OAD.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, resolution.UnresolvedMovies, 1)
	assert.Equal(t, filepath.Join(basePath, "Example OAD NCOP01.mkv"), resolution.UnresolvedMovies[0].Path)
	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "special OAD.srt"), plan.Skips[0].SourcePath)
	assert.Equal(t, "no unique movie matched extracted episode", plan.Skips[0].Reason)
}

func TestResolveByRuleDoesNotTreatVersionedOADBonusVideoAsOADMovie(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD NCOP01v2.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "special OAD.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, resolution.UnresolvedMovies, 1)
	assert.Equal(t, filepath.Join(basePath, "Example OAD NCOP01v2.mkv"), resolution.UnresolvedMovies[0].Path)
	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "special OAD.srt"), plan.Skips[0].SourcePath)
	assert.Equal(t, "no unique movie matched extracted episode", plan.Skips[0].Reason)
}

func TestResolveByRuleMatchesLowercaseOADMarker(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example oad.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub oad.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, plan.Operations, 1)
	assert.Equal(t, filepath.Join(basePath, "fansub oad.srt"), plan.Operations[0].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "Example oad.srt"), plan.Operations[0].DestinationPath)
}

func TestResolveByRuleDoesNotTreatNumberedOADSubtitleAsEpisodeSubtitle(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "special OAD01.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "special OAD01.srt"), plan.Skips[0].SourcePath)
	assert.Equal(t, "no unique movie matched extracted episode", plan.Skips[0].Reason)
}

func TestResolveByRuleMatchesUnderscoreNumberedOADMarker(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example_OAD01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub_OAD01.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, plan.Operations, 1)
	assert.Equal(t, filepath.Join(basePath, "fansub_OAD01.srt"), plan.Operations[0].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "Example_OAD01.srt"), plan.Operations[0].DestinationPath)
}

func TestEnforceSafeRenamePlanSkipsDuplicateSourcePaths(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	sourcePath := filepath.Join(basePath, "candidate.srt")
	require.NoError(t, afero.WriteFile(re.FileSystem, sourcePath, []byte("subtitle"), 0644))

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: sourcePath},
		},
	}
	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      sourcePath,
				DestinationPath: filepath.Join(basePath, "target-a.srt"),
				DestinationName: "target-a.srt",
				MatchSource:     "ai",
			},
			{
				SourcePath:      sourcePath,
				DestinationPath: filepath.Join(basePath, "target-b.srt"),
				DestinationName: "target-b.srt",
				MatchSource:     "ai",
			},
		},
	}

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	assert.Empty(t, safePlan.Operations)
	require.Len(t, safePlan.Skips, 1)
	assert.Equal(t, sourcePath, safePlan.Skips[0].SourcePath)
	assert.Equal(t, "rename source is referenced by multiple planned renames", safePlan.Skips[0].Reason)
}

func TestEnforceSafeRenamePlanSkipsCaseInsensitiveAliasDestinationConflicts(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	firstSourcePath := filepath.Join(basePath, "first.srt")
	secondSourcePath := filepath.Join(basePath, "second.srt")
	firstDestinationPath := filepath.Join(basePath, "Target.srt")
	secondDestinationPath := filepath.Join(basePath, "target.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, firstSourcePath, []byte("first"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, secondSourcePath, []byte("second"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, firstDestinationPath, []byte("probe"), 0644))

	firstDestinationInfo, err := re.FileSystem.Stat(firstDestinationPath)
	require.NoError(t, err)
	secondDestinationInfo, err := re.FileSystem.Stat(secondDestinationPath)
	if err != nil || !os.SameFile(firstDestinationInfo, secondDestinationInfo) {
		t.Skip("case-sensitive filesystem does not alias case-only destination paths")
	}
	require.NoError(t, re.FileSystem.Remove(firstDestinationPath))

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: firstSourcePath},
			{Path: secondSourcePath},
		},
	}
	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      firstSourcePath,
				DestinationPath: firstDestinationPath,
				DestinationName: filepath.Base(firstDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      secondSourcePath,
				DestinationPath: secondDestinationPath,
				DestinationName: filepath.Base(secondDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	assert.Empty(t, safePlan.Operations)
	require.Len(t, safePlan.Skips, 2)

	reasonsBySource := map[string]string{}
	for _, skip := range safePlan.Skips {
		reasonsBySource[skip.SourcePath] = skip.Reason
	}
	assert.Equal(t, "rename target conflicts with another planned rename", reasonsBySource[firstSourcePath])
	assert.Equal(t, "rename target conflicts with another planned rename", reasonsBySource[secondSourcePath])
}

func TestEnforceSafeRenamePlanSkipsUnicodeCaseFoldDestinationConflicts(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	firstSourcePath := filepath.Join(basePath, "first.srt")
	secondSourcePath := filepath.Join(basePath, "second.srt")
	firstDestinationPath := filepath.Join(basePath, "ΟΣ.srt")
	secondDestinationPath := filepath.Join(basePath, "ος.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, firstSourcePath, []byte("first"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, secondSourcePath, []byte("second"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, firstDestinationPath, []byte("probe"), 0644))

	firstDestinationInfo, err := re.FileSystem.Stat(firstDestinationPath)
	require.NoError(t, err)
	secondDestinationInfo, err := re.FileSystem.Stat(secondDestinationPath)
	if err != nil || !os.SameFile(firstDestinationInfo, secondDestinationInfo) {
		t.Skip("filesystem does not alias unicode case-fold destination paths")
	}
	require.NoError(t, re.FileSystem.Remove(firstDestinationPath))

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: firstSourcePath},
			{Path: secondSourcePath},
		},
	}
	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      firstSourcePath,
				DestinationPath: firstDestinationPath,
				DestinationName: filepath.Base(firstDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      secondSourcePath,
				DestinationPath: secondDestinationPath,
				DestinationName: filepath.Base(secondDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	assert.Empty(t, safePlan.Operations)
	require.Len(t, safePlan.Skips, 2)

	reasonsBySource := map[string]string{}
	for _, skip := range safePlan.Skips {
		reasonsBySource[skip.SourcePath] = skip.Reason
	}
	assert.Equal(t, "rename target conflicts with another planned rename", reasonsBySource[firstSourcePath])
	assert.Equal(t, "rename target conflicts with another planned rename", reasonsBySource[secondSourcePath])
}

func TestEnforceSafeRenamePlanSkipsAliasDestinationNotFreedByCaseOnlyRenameOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	normalizingSourcePath := filepath.Join(basePath, "[BD] Example - 01.SRT")
	normalizingDestinationPath := filepath.Join(basePath, "[BD] Example - 01.srt")
	conflictingSourcePath := filepath.Join(basePath, "candidate.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, normalizingSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, conflictingSourcePath, []byte("moving"), 0644))

	sourceInfo, err := re.FileSystem.Stat(normalizingSourcePath)
	require.NoError(t, err)
	destinationInfo, err := re.FileSystem.Stat(normalizingDestinationPath)
	if err != nil || !os.SameFile(sourceInfo, destinationInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase subtitle paths")
	}

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: normalizingSourcePath},
			{Path: conflictingSourcePath},
		},
	}
	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      normalizingSourcePath,
				DestinationPath: normalizingDestinationPath,
				DestinationName: filepath.Base(normalizingDestinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
			{
				SourcePath:      conflictingSourcePath,
				DestinationPath: normalizingSourcePath,
				DestinationName: filepath.Base(normalizingSourcePath),
				MatchSource:     "ai",
			},
		},
	}

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	require.Len(t, safePlan.Operations, 1)
	assert.Equal(t, normalizingSourcePath, safePlan.Operations[0].SourcePath)
	assert.Equal(t, normalizingDestinationPath, safePlan.Operations[0].DestinationPath)
	require.Len(t, safePlan.Skips, 1)
	assert.Equal(t, conflictingSourcePath, safePlan.Skips[0].SourcePath)
	assert.Equal(t, "rename target conflicts with another planned rename", safePlan.Skips[0].Reason)
}

func TestEnforceSafeRenamePlanAllowsAliasDestinationFreedByPendingRenameOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "[BD] Example - 01.SRT")
	aliasDestinationPath := filepath.Join(basePath, "[BD] Example - 01.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")
	freedDestinationPath := filepath.Join(basePath, "[BD] Other - 01.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase subtitle paths")
	}

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: movingSourcePath},
			{Path: activeSourcePath},
		},
	}
	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      movingSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      activeSourcePath,
				DestinationPath: freedDestinationPath,
				DestinationName: filepath.Base(freedDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	require.Len(t, safePlan.Operations, 2)
	assert.Empty(t, safePlan.Skips)
}

func TestEnforceSafeRenamePlanAllowsNormalizationEquivalentAliasDestinationFreedByPendingRename(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "Café.srt")
	aliasDestinationPath := filepath.Join(basePath, "Cafe\u0301.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")
	freedDestinationPath := filepath.Join(basePath, "moved-away.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("filesystem does not alias normalization-equivalent subtitle paths")
	}

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: movingSourcePath},
			{Path: activeSourcePath},
		},
	}
	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      movingSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      activeSourcePath,
				DestinationPath: freedDestinationPath,
				DestinationName: filepath.Base(freedDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	require.Len(t, safePlan.Operations, 2)
	assert.Empty(t, safePlan.Skips)
}

func TestEnforceSafeRenamePlanAllowsThreeWayCycle(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	aPath := filepath.Join(basePath, "a.srt")
	bPath := filepath.Join(basePath, "b.srt")
	cPath := filepath.Join(basePath, "c.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, aPath, []byte("subtitle-a"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, bPath, []byte("subtitle-b"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, cPath, []byte("subtitle-c"), 0644))

	scanResult := re.ScanResult{
		Subtitles: []re.MediaFile{
			{Path: aPath},
			{Path: bPath},
			{Path: cPath},
		},
	}
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

	safePlan := re.EnforceSafeRenamePlan(plan, scanResult)

	require.Len(t, safePlan.Operations, 3)
	assert.Empty(t, safePlan.Skips)
}

func TestApplyRenamePlanHandlesAliasDestinationFreedByLaterRenameOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "[BD] Example - 01.SRT")
	aliasDestinationPath := filepath.Join(basePath, "[BD] Example - 01.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")
	freedDestinationPath := filepath.Join(basePath, "[BD] Other - 01.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase subtitle paths")
	}

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      movingSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      activeSourcePath,
				DestinationPath: freedDestinationPath,
				DestinationName: filepath.Base(freedDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	require.NoError(t, re.ApplyRenamePlan(plan))

	aliasContent, err := afero.ReadFile(re.FileSystem, aliasDestinationPath)
	require.NoError(t, err)
	assert.Equal(t, "moving", string(aliasContent))

	freedContent, err := afero.ReadFile(re.FileSystem, freedDestinationPath)
	require.NoError(t, err)
	assert.Equal(t, "active", string(freedContent))
}

func TestApplyRenamePlanHandlesUnicodeCaseFoldAliasDestinationFreedByLaterRenameOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "ΟΣ.srt")
	aliasDestinationPath := filepath.Join(basePath, "ος.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")
	freedDestinationPath := filepath.Join(basePath, "moved-away.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("filesystem does not alias unicode case-fold subtitle paths")
	}

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      movingSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      activeSourcePath,
				DestinationPath: freedDestinationPath,
				DestinationName: filepath.Base(freedDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	require.NoError(t, re.ApplyRenamePlan(plan))

	aliasContent, err := afero.ReadFile(re.FileSystem, aliasDestinationPath)
	require.NoError(t, err)
	assert.Equal(t, "moving", string(aliasContent))

	freedContent, err := afero.ReadFile(re.FileSystem, freedDestinationPath)
	require.NoError(t, err)
	assert.Equal(t, "active", string(freedContent))
}

func TestApplyRenamePlanHandlesNormalizationEquivalentAliasDestinationFreedByLaterRename(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "Café.srt")
	aliasDestinationPath := filepath.Join(basePath, "Cafe\u0301.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")
	freedDestinationPath := filepath.Join(basePath, "moved-away.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("filesystem does not alias normalization-equivalent subtitle paths")
	}

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      movingSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      activeSourcePath,
				DestinationPath: freedDestinationPath,
				DestinationName: filepath.Base(freedDestinationPath),
				MatchSource:     "ai",
			},
		},
	}

	require.NoError(t, re.ApplyRenamePlan(plan))

	aliasContent, err := afero.ReadFile(re.FileSystem, aliasDestinationPath)
	require.NoError(t, err)
	assert.Equal(t, "moving", string(aliasContent))

	freedContent, err := afero.ReadFile(re.FileSystem, freedDestinationPath)
	require.NoError(t, err)
	assert.Equal(t, "active", string(freedContent))
}

func TestApplyRenamePlanRejectsAliasDestinationNotFreedByCaseOnlyRenameOnCaseInsensitiveFS(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "[BD] Example - 01.SRT")
	aliasDestinationPath := filepath.Join(basePath, "[BD] Example - 01.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("case-sensitive filesystem does not alias uppercase/lowercase subtitle paths")
	}

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      activeSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
			{
				SourcePath:      movingSourcePath,
				DestinationPath: activeSourcePath,
				DestinationName: filepath.Base(activeSourcePath),
				MatchSource:     "ai",
			},
		},
	}

	err = re.ApplyRenamePlan(plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), movingSourcePath)
	assert.Contains(t, err.Error(), activeSourcePath)

	activeContent, err := afero.ReadFile(re.FileSystem, activeSourcePath)
	require.NoError(t, err)
	assert.Equal(t, "active", string(activeContent))

	movingContent, err := afero.ReadFile(re.FileSystem, movingSourcePath)
	require.NoError(t, err)
	assert.Equal(t, "moving", string(movingContent))
}

func TestApplyRenamePlanRejectsNormalizationEquivalentAliasDestinationNotFreedByPendingRename(t *testing.T) {
	re.FileSystem = afero.NewOsFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	activeSourcePath := filepath.Join(basePath, "Café.srt")
	aliasDestinationPath := filepath.Join(basePath, "Cafe\u0301.srt")
	movingSourcePath := filepath.Join(basePath, "candidate.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, activeSourcePath, []byte("active"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, movingSourcePath, []byte("moving"), 0644))

	activeInfo, err := re.FileSystem.Stat(activeSourcePath)
	require.NoError(t, err)
	aliasInfo, err := re.FileSystem.Stat(aliasDestinationPath)
	if err != nil || !os.SameFile(activeInfo, aliasInfo) {
		t.Skip("filesystem does not alias normalization-equivalent subtitle paths")
	}

	plan := re.RenamePlan{
		Operations: []re.RenameOperation{
			{
				SourcePath:      activeSourcePath,
				DestinationPath: aliasDestinationPath,
				DestinationName: filepath.Base(aliasDestinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
			{
				SourcePath:      movingSourcePath,
				DestinationPath: activeSourcePath,
				DestinationName: filepath.Base(activeSourcePath),
				MatchSource:     "ai",
			},
		},
	}

	err = re.ApplyRenamePlan(plan)
	require.Error(t, err)
	assert.Contains(t, err.Error(), movingSourcePath)
	assert.Contains(t, err.Error(), activeSourcePath)

	activeContent, err := afero.ReadFile(re.FileSystem, activeSourcePath)
	require.NoError(t, err)
	assert.Equal(t, "active", string(activeContent))

	movingContent, err := afero.ReadFile(re.FileSystem, movingSourcePath)
	require.NoError(t, err)
	assert.Equal(t, "moving", string(movingContent))
}
