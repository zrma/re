package re

import (
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChangeExtToLowerPreservesBaseName(t *testing.T) {
	FileSystem = afero.NewMemMapFs()
	defer func() { FileSystem = newOSFileSystem() }()

	basePath := filepath.Join("home", "folder", "JohnDoe", "Downloads")
	originalPath := filepath.Join(basePath, "BEST.SRT")

	err := afero.WriteFile(FileSystem, originalPath, []byte("episode"), 0644)
	require.NoError(t, err)

	changeExtToLower(basePath)

	_, err = FileSystem.Stat(filepath.Join(basePath, "BEST.srt"))
	assert.NoError(t, err)

	_, err = FileSystem.Stat(filepath.Join(basePath, "BE.srt"))
	assert.Error(t, err)
}
