package test

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

func TestResolveByRuleKeepsNumberedOADEntriesDistinct(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD01.mkv"), []byte("movie-1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD02.mkv"), []byte("movie-2"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub OAD01.srt"), []byte("subtitle-1"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub OAD02.srt"), []byte("subtitle-2"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, plan.Operations, 2)
	assert.Equal(t, filepath.Join(basePath, "fansub OAD01.srt"), plan.Operations[0].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "Example OAD01.srt"), plan.Operations[0].DestinationPath)
	assert.Equal(t, filepath.Join(basePath, "fansub OAD02.srt"), plan.Operations[1].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "Example OAD02.srt"), plan.Operations[1].DestinationPath)
	assert.Empty(t, plan.Skips)
}

func TestResolveByRuleDoesNotGuessBareOADAgainstNumberedOADMovie(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub OAD.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	assert.Empty(t, plan.Operations)
	require.Len(t, plan.Skips, 1)
	assert.Equal(t, filepath.Join(basePath, "fansub OAD.srt"), plan.Skips[0].SourcePath)
	assert.Equal(t, "no unique movie matched extracted episode", plan.Skips[0].Reason)
}

func TestResolveByRuleMatchesVersionedNumberedOADMarker(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub OAD01v2.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, plan.Operations, 1)
	assert.Equal(t, filepath.Join(basePath, "fansub OAD01v2.srt"), plan.Operations[0].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "Example OAD01.srt"), plan.Operations[0].DestinationPath)
	assert.Empty(t, plan.Skips)
}

func TestResolveByRuleMatchesNumberedOADAcrossCommonSeparators(t *testing.T) {
	re.FileSystem = afero.NewMemMapFs()
	defer func() { re.FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "Example OAD-01.mkv"), []byte("movie"), 0644))
	require.NoError(t, afero.WriteFile(re.FileSystem, filepath.Join(basePath, "fansub OAD 01v2.srt"), []byte("subtitle"), 0644))

	scanResult, err := re.ScanDirectory(basePath)
	require.NoError(t, err)

	resolution := re.ResolveByRule(scanResult)
	plan := re.BuildRenamePlan(resolution)

	require.Len(t, plan.Operations, 1)
	assert.Equal(t, filepath.Join(basePath, "fansub OAD 01v2.srt"), plan.Operations[0].SourcePath)
	assert.Equal(t, filepath.Join(basePath, "Example OAD-01.srt"), plan.Operations[0].DestinationPath)
	assert.Empty(t, plan.Skips)
}
