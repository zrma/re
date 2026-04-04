package re

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
)

type MediaKind string

const (
	MovieKind    MediaKind = "movie"
	SubtitleKind MediaKind = "subtitle"
)

type MediaFile struct {
	Path      string
	Name      string
	BaseName  string
	Extension string
	Kind      MediaKind
}

type ScanResult struct {
	Movies             []MediaFile
	Subtitles          []MediaFile
	TemporaryArtifacts []string
}

func ScanDirectory(targetPath string) (ScanResult, error) {
	movieExtList := map[string]bool{"avi": true, "mkv": true, "mp4": true}
	subtitleExtList := map[string]bool{"srt": true, "ass": true, "smi": true}

	var result ScanResult

	entries, err := afero.ReadDir(FileSystem, targetPath)
	if err != nil {
		return ScanResult{}, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(targetPath, entry.Name())
		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		normalizedExt := strings.ToLower(ext)
		name := filepath.Base(path)
		if isInternalAliasProbeFile(name) {
			continue
		}
		if isInternalTemporaryRenameFile(name) {
			result.TemporaryArtifacts = append(result.TemporaryArtifacts, path)
			continue
		}
		baseName := strings.TrimSuffix(name, filepath.Ext(name))

		if movieExtList[normalizedExt] {
			result.Movies = append(result.Movies, MediaFile{
				Path:      path,
				Name:      name,
				BaseName:  baseName,
				Extension: "." + normalizedExt,
				Kind:      MovieKind,
			})
		}
		if subtitleExtList[normalizedExt] {
			result.Subtitles = append(result.Subtitles, MediaFile{
				Path:      path,
				Name:      name,
				BaseName:  baseName,
				Extension: "." + normalizedExt,
				Kind:      SubtitleKind,
			})
		}
	}

	sort.Slice(result.Movies, func(i, j int) bool {
		return result.Movies[i].Name < result.Movies[j].Name
	})
	sort.Slice(result.Subtitles, func(i, j int) bool {
		return result.Subtitles[i].Path < result.Subtitles[j].Path
	})

	return result, nil
}

func isInternalAliasProbeFile(name string) bool {
	return strings.HasPrefix(name, ".re-fs-probe-") && strings.HasSuffix(strings.ToLower(name), ".tmp")
}

func isInternalTemporaryRenameFile(name string) bool {
	dotIndex := strings.LastIndex(name, ".")
	if dotIndex <= 0 {
		return false
	}

	baseName := name[:dotIndex]
	if !strings.HasPrefix(baseName, ".") {
		return false
	}

	markerIndex := strings.LastIndex(baseName, ".re-tmp-")
	if markerIndex <= 0 {
		return false
	}

	suffix := baseName[markerIndex+len(".re-tmp-"):]
	if suffix == "" {
		return false
	}

	_, err := strconv.Atoi(suffix)
	return err == nil
}
