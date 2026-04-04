package test

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
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

func TestRunWithOptionsAIReducesUnresolvedMovieCountWhenMatchingUnparseableMovie(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "Bonus Feature.mkv")
	subtitlePath := filepath.Join(basePath, "bonus-kor.srt")

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
					Reason:           "subtitle belongs to the bonus feature release",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("Y\n"), &output, options)

	_, err := re.FileSystem.Stat(filepath.Join(basePath, "Bonus Feature.srt"))
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(subtitlePath)
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[ai:0.95] "+subtitlePath+" -> Bonus Feature.srt")
	assert.Contains(t, output.String(), "unresolved movies 0")
	assert.Contains(t, output.String(), "unresolved subtitles 0")
}

func TestRunWithOptionsAIReducesUnresolvedMovieCountWhenSubtitleAlreadyMatchesTarget(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	mainMoviePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).mkv")
	specialMoviePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).mkv")
	subtitlePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, mainMoviePath, []byte("main"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialMoviePath, []byte("special"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, subtitlePath, []byte("subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.OutputFormat = re.OutputFormatJSON
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     subtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: specialMoviePath,
					Confidence:       0.96,
					Reason:           "subtitle already matches the special release basename",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader("ignored\n"), &output, options)

	var report struct {
		Status               string `json:"status"`
		RequiresConfirmation bool   `json:"requires_confirmation"`
		Summary              struct {
			PlannedRenames      int `json:"planned_renames"`
			UnresolvedMovies    int `json:"unresolved_movies"`
			UnresolvedSubtitles int `json:"unresolved_subtitles"`
		} `json:"summary"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &report))

	assert.Equal(t, "noop", report.Status)
	assert.False(t, report.RequiresConfirmation)
	assert.Equal(t, 0, report.Summary.PlannedRenames)
	assert.Equal(t, 1, report.Summary.UnresolvedMovies)
	assert.Equal(t, 0, report.Summary.UnresolvedSubtitles)
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

func TestRunWithOptionsSkipsDuplicateAISubtitleDecisions(t *testing.T) {
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
				{
					SubtitlePath:     subtitlePath,
					Outcome:          re.AIDecisionSkip,
					MatchedMoviePath: "",
					Confidence:       0.10,
					Reason:           "conflicting duplicate decision",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	_, err := re.FileSystem.Stat(subtitlePath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(filepath.Join(basePath, "[BD] Special OAD.srt"))
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[skip] "+subtitlePath+" (ai returned multiple decisions for the same subtitle)")
	assert.Contains(t, output.String(), "Summary: 0 renames (rule 0, ai 0), 1 skips, unresolved movies 0, unresolved subtitles 1")
}

func TestRunWithOptionsDoesNotDuplicateTemporaryArtifactSkipsWhenAIEnabled(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Example - 01 (BD).mkv")
	unmatchedSubtitlePath := filepath.Join(basePath, "special-kor.srt")
	artifactPath := filepath.Join(basePath, ".special-kor.re-tmp-0.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, unmatchedSubtitlePath, []byte("subtitle"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, artifactPath, []byte("temp-subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	assert.Equal(t, 1, strings.Count(output.String(), "[skip] "+artifactPath+" (leftover internal temporary rename artifact detected)"))
	assert.Contains(t, output.String(), "[skip] "+unmatchedSubtitlePath+" (episode pattern not recognized by rule matcher)")
	assert.Contains(t, output.String(), "Summary: 0 renames (rule 0, ai 0), 2 skips, unresolved movies 0, unresolved subtitles 1")
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

func TestRunWithOptionsAppliesAIChainRenameIntoFreedSubtitlePath(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	mainMoviePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).mkv")
	specialMoviePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).mkv")
	occupiedTargetPath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).srt")
	unresolvedSubtitlePath := filepath.Join(basePath, "commentary-kor.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, mainMoviePath, []byte("main-movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialMoviePath, []byte("special-movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, occupiedTargetPath, []byte("subtitle-for-main"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, unresolvedSubtitlePath, []byte("subtitle-for-special"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     occupiedTargetPath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: mainMoviePath,
					Confidence:       0.97,
					Reason:           "subtitle content matches main episode",
				},
				{
					SubtitlePath:     unresolvedSubtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: specialMoviePath,
					Confidence:       0.96,
					Reason:           "subtitle content matches special episode",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	mainSubtitlePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).srt")
	specialSubtitlePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).srt")

	mainContent, err := afero.ReadFile(re.FileSystem, mainSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-for-main", string(mainContent))

	specialContent, err := afero.ReadFile(re.FileSystem, specialSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-for-special", string(specialContent))

	_, err = re.FileSystem.Stat(unresolvedSubtitlePath)
	assert.Error(t, err)
	assert.NotContains(t, output.String(), "rename target already exists as another subtitle")
}

func TestRunWithOptionsAppliesAICyclicSubtitleSwap(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	mainMoviePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).mkv")
	specialMoviePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).mkv")
	mainSubtitlePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).srt")
	specialSubtitlePath := filepath.Join(basePath, "[BD] Special Story - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, mainMoviePath, []byte("main-movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialMoviePath, []byte("special-movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, mainSubtitlePath, []byte("subtitle-for-main"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialSubtitlePath, []byte("subtitle-for-special"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     mainSubtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: specialMoviePath,
					Confidence:       0.98,
					Reason:           "subtitle belongs to special release",
				},
				{
					SubtitlePath:     specialSubtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: mainMoviePath,
					Confidence:       0.98,
					Reason:           "subtitle belongs to main release",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	mainContent, err := afero.ReadFile(re.FileSystem, mainSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-for-special", string(mainContent))

	specialContent, err := afero.ReadFile(re.FileSystem, specialSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-for-main", string(specialContent))

	assert.NotContains(t, output.String(), "rename target already exists as another subtitle")
}

func TestRunWithOptionsAllowsAIRenameAfterUnsafeRuleTargetsAreFiltered(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	moviePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).mkv")
	duplicateSubtitleAPath := filepath.Join(basePath, "1화.srt")
	duplicateSubtitleBPath := filepath.Join(basePath, "01.srt")
	aiSubtitlePath := filepath.Join(basePath, "commentary-kor.srt")
	targetSubtitlePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, moviePath, []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, duplicateSubtitleAPath, []byte("subtitle-a"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, duplicateSubtitleBPath, []byte("subtitle-b"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, aiSubtitlePath, []byte("subtitle-ai"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     aiSubtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: moviePath,
					Confidence:       0.97,
					Reason:           "commentary track matches the main episode release",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	content, err := afero.ReadFile(re.FileSystem, targetSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "subtitle-ai", string(content))

	_, err = re.FileSystem.Stat(aiSubtitlePath)
	assert.Error(t, err)
	_, err = re.FileSystem.Stat(duplicateSubtitleAPath)
	assert.NoError(t, err)
	_, err = re.FileSystem.Stat(duplicateSubtitleBPath)
	assert.NoError(t, err)

	assert.Contains(t, output.String(), "[ai:0.97] "+aiSubtitlePath+" -> [BD] Main Story - 01 (BD).srt")
	assert.Contains(t, output.String(), "[skip] "+duplicateSubtitleAPath+" (rename target conflicts with another planned rename)")
	assert.Contains(t, output.String(), "[skip] "+duplicateSubtitleBPath+" (rename target conflicts with another planned rename)")
	assert.NotContains(t, output.String(), "[skip] "+aiSubtitlePath+" (ai destination conflicts with an existing rename target)")
}

func TestRunWithOptionsUsesAIForSubtitleSkippedBySafeRulePlan(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	mainMoviePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).mkv")
	specialMoviePath := filepath.Join(basePath, "[BD] Special OAD.mkv")
	existingMainSubtitlePath := filepath.Join(basePath, "[BD] Main Story - 01 (BD).srt")
	conflictingRuleSubtitlePath := filepath.Join(basePath, "1화.srt")
	specialSubtitlePath := filepath.Join(basePath, "[BD] Special OAD.srt")

	require.NoError(t, afero.WriteFile(re.FileSystem, mainMoviePath, []byte("main-movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, specialMoviePath, []byte("special-movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, existingMainSubtitlePath, []byte("main-subtitle"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, conflictingRuleSubtitlePath, []byte("special-subtitle"), 0644))

	options := re.DefaultRunOptions()
	options.AssumeYes = true
	options.AI.Enabled = true
	options.AI.Resolver = fakeAIResolver{
		output: re.AIOutput{
			Decisions: []re.AIDecision{
				{
					SubtitlePath:     conflictingRuleSubtitlePath,
					Outcome:          re.AIDecisionMatch,
					MatchedMoviePath: specialMoviePath,
					Confidence:       0.97,
					Reason:           "subtitle matches the OAD release rather than the main episode",
				},
			},
		},
	}

	var output bytes.Buffer
	re.RunWithOptions(basePath, strings.NewReader(""), &output, options)

	mainContent, err := afero.ReadFile(re.FileSystem, existingMainSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "main-subtitle", string(mainContent))

	specialContent, err := afero.ReadFile(re.FileSystem, specialSubtitlePath)
	require.NoError(t, err)
	assert.Equal(t, "special-subtitle", string(specialContent))

	_, err = re.FileSystem.Stat(conflictingRuleSubtitlePath)
	assert.Error(t, err)
	assert.Contains(t, output.String(), "[ai:0.97] "+conflictingRuleSubtitlePath+" -> [BD] Special OAD.srt")
	assert.NotContains(t, output.String(), "[skip] "+conflictingRuleSubtitlePath+" (rename target already exists as another subtitle)")
}

func TestMergeAIRenamePlanSkipsInvalidConfidenceValues(t *testing.T) {
	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	movie := re.MediaFile{
		Path:      filepath.Join(basePath, "[BD] Example - 01 (BD).mkv"),
		BaseName:  "[BD] Example - 01 (BD)",
		Extension: ".mkv",
		Kind:      re.MovieKind,
	}
	subtitle := re.MediaFile{
		Path:      filepath.Join(basePath, "1화.srt"),
		Name:      "1화.srt",
		BaseName:  "1화",
		Extension: ".srt",
		Kind:      re.SubtitleKind,
	}
	scanResult := re.ScanResult{
		Movies:    []re.MediaFile{movie},
		Subtitles: []re.MediaFile{subtitle},
	}

	testCases := []struct {
		name       string
		confidence float64
		reason     string
	}{
		{
			name:       "nan",
			confidence: math.NaN(),
			reason:     "ai returned invalid confidence value: NaN",
		},
		{
			name:       "above_one",
			confidence: 1.01,
			reason:     "ai returned invalid confidence value: 1.01",
		},
		{
			name:       "negative_infinity",
			confidence: math.Inf(-1),
			reason:     "ai returned invalid confidence value: -Inf",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			merged := re.MergeAIRenamePlan(
				re.RenamePlan{},
				scanResult,
				[]re.MediaFile{subtitle},
				re.AIOutput{
					Decisions: []re.AIDecision{
						{
							SubtitlePath:     subtitle.Path,
							Outcome:          re.AIDecisionMatch,
							MatchedMoviePath: movie.Path,
							Confidence:       tt.confidence,
							Reason:           "invalid confidence from resolver",
						},
					},
				},
				0.90,
			)

			assert.Empty(t, merged.Operations)
			require.Len(t, merged.Skips, 1)
			assert.Equal(t, subtitle.Path, merged.Skips[0].SourcePath)
			assert.Equal(t, tt.reason, merged.Skips[0].Reason)
		})
	}
}
