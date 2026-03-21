package test

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

type fakeAIResolver struct {
	output re.AIOutput
}

func (r fakeAIResolver) Resolve(_ context.Context, _ re.AIInput) (re.AIOutput, error) {
	return r.output, nil
}

func TestRunWithOptionsUsesAIResolverForUnresolvedSubtitle(t *testing.T) {
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

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("Y\n"), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "[BD] Special OAD.srt"))
	assert.NoError(t, err)

	_, err = re.FileSystem.Stat(subtitlePath)
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[ai:0.95] "+subtitlePath+" -> [BD] Special OAD.srt")
}

func TestRunWithOptionsSkipsLowConfidenceAIMatch(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Special OAD.mkv")
	subtitlePath := filepath.Join(basePath, "special-kor.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AI.Enabled = true
	options.AI.MinConfidence = 0.99
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

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("Y\n"), &output, options)

	_, err := re.FileSystem.Stat(subtitlePath)
	assert.NoError(t, err)

	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[BD] Special OAD.srt"))
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[skip] "+subtitlePath+" (ai confidence 0.95 below threshold 0.99)")
}

func TestRunWithOptionsShowsRuleSkipReason(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Example OAD.mkv")
	subtitlePath := filepath.Join(basePath, "special-kor.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("n\n"), &output, re.DefaultRunOptions())

	assert.Contains(t, output.String(), "[skip] "+subtitlePath+" (episode pattern not recognized by rule matcher)")
}

func TestRunWithOptionsAssumeYesSkipsPromptAndAppliesRename(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Example - 01 (BD).mkv")
	subtitlePath := filepath.Join(basePath, "1화.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "[BD] Example - 01 (BD).srt"))
	assert.NoError(t, err)

	_, err = re.FileSystem.Stat(subtitlePath)
	assert.Error(t, err)

	assert.NotContains(t, output.String(), "Do you want to rename?")
	assert.Contains(t, output.String(), "Done!")
}
