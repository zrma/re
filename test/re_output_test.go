package test

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

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
