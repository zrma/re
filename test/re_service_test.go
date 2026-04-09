package test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

type failingAIResolver struct {
	err error
}

func (r failingAIResolver) Resolve(_ context.Context, _ re.AIInput) (re.AIOutput, error) {
	return re.AIOutput{}, r.err
}

func TestBuildPreviewReturnsRulePlanAndReport(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "1화.srt"), []byte("subtitle"), 0644))

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath + string(filepath.Separator),
		Options:    re.DefaultRunOptions(),
	})
	require.NoError(t, err)

	require.Len(t, preview.Plan.Operations, 1)
	assert.Equal(t, filepath.Clean(basePath), preview.TargetPath)
	assert.Equal(t, "rule", preview.Plan.Operations[0].MatchSource)
	assert.Equal(t, "canceled", preview.Report.Status)
	assert.True(t, preview.Report.RequiresConfirmation)
	assert.Equal(t, 1, preview.Report.Summary.PlannedRenames)
	assert.Equal(t, 1, preview.Report.Summary.RuleRenames)
	assert.Equal(t, 0, preview.Report.Summary.AIRenames)
}

func TestBuildPreviewUsesAIResolverWhenEnabled(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Special OAD.mkv")
	subtitlePath := filepath.Join(basePath, "special-kor.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     subtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: moviePath,
					Confidence:       0.95,
					Reason:           "same OAD marker",
				},
			},
		},
	}

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath,
		Options:    options,
	})
	require.NoError(t, err)

	require.Len(t, preview.Plan.Operations, 1)
	assert.Equal(t, "ai", preview.Plan.Operations[0].MatchSource)
	assert.Equal(t, 1, preview.Report.Summary.PlannedRenames)
	assert.Equal(t, 0, preview.Report.Summary.RuleRenames)
	assert.Equal(t, 1, preview.Report.Summary.AIRenames)
}

func TestBuildPreviewKeepsRuleOnlyPreviewWhenAIResolverFails(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Bonus Feature.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "bonus-kor.srt"), []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AI.Enabled = true
	options.AI.Resolver = failingAIResolver{err: errors.New("resolver offline")}

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath,
		Options:    options,
	})
	require.NoError(t, err)

	assert.Empty(t, preview.Plan.Operations)
	assert.Len(t, preview.Warnings, 1)
	assert.Contains(t, preview.Warnings[0], "ai fallback failed")
	assert.Equal(t, 1, preview.Report.Summary.UnresolvedMovies)
	assert.Equal(t, 1, preview.Report.Summary.UnresolvedSubtitles)
}

func TestBuildPreviewReturnsNoopWhenNothingNeedsRename(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01.srt"), []byte("subtitle"), 0644))

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath,
		Options:    re.DefaultRunOptions(),
	})
	require.NoError(t, err)

	assert.Empty(t, preview.Plan.Operations)
	assert.Empty(t, preview.Plan.Skips)
	assert.Equal(t, "noop", preview.Report.Status)
	assert.False(t, preview.Report.RequiresConfirmation)
}

func TestBuildPreviewReturnsNeedsReviewWhenOnlySkipsRemain(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "[BD] Example - 01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "special-kor.srt"), []byte("subtitle"), 0644))

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath,
		Options:    re.DefaultRunOptions(),
	})
	require.NoError(t, err)

	assert.Empty(t, preview.Plan.Operations)
	assert.Len(t, preview.Plan.Skips, 1)
	assert.Equal(t, "needs_review", preview.Report.Status)
	assert.Equal(t, 1, preview.Report.Summary.UnresolvedSubtitles)
}

func TestApplyPreviewRejectsStaleDestinationState(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Example - 01.mkv")
	subtitlePath := filepath.Join(basePath, "1화.srt")
	targetPath := filepath.Join(basePath, "[BD] Example - 01.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath,
		Options:    re.DefaultRunOptions(),
	})
	require.NoError(t, err)

	require.NoError(t, afero.WriteFile(re.FileSystem, targetPath, []byte("new-conflict"), 0644))

	_, err = re.ApplyPreview(preview)
	require.Error(t, err)
	assert.True(t, errors.Is(err, re.ErrPreviewExpired))

	_, statErr := re.FileSystem.Stat(subtitlePath)
	assert.NoError(t, statErr)
}

func TestApplyPreviewAppliesPlannedRename(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Example - 01.mkv")
	subtitlePath := filepath.Join(basePath, "1화.srt")
	targetPath := filepath.Join(basePath, "[BD] Example - 01.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
		TargetPath: basePath,
		Options:    re.DefaultRunOptions(),
	})
	require.NoError(t, err)

	report, err := re.ApplyPreview(preview)
	require.NoError(t, err)

	assert.True(t, report.Applied)
	assert.Equal(t, "applied", report.Status)
	_, err = re.FileSystem.Stat(targetPath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(subtitlePath)
	assert.Error(t, err)
}
