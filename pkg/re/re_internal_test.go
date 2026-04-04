package re

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanDirectoryNormalizesSubtitleExtensionToLowercase(t *testing.T) {
	FileSystem = afero.NewMemMapFs()
	defer func() { FileSystem = newOSFileSystem() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	originalPath := filepath.Join(basePath, "BEST.SRT")

	err := afero.WriteFile(FileSystem, originalPath, []byte("episode"), 0644)
	require.NoError(t, err)

	scanResult, err := ScanDirectory(basePath)
	require.NoError(t, err)

	require.Len(t, scanResult.Subtitles, 1)
	assert.Equal(t, "BEST", scanResult.Subtitles[0].BaseName)
	assert.Equal(t, ".srt", scanResult.Subtitles[0].Extension)
}
