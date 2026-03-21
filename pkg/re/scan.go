package re

import (
	"io/fs"
	"path/filepath"
	"sort"
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
	Movies    []MediaFile
	Subtitles []MediaFile
}

func ScanDirectory(targetPath string) (ScanResult, error) {
	movieExtList := map[string]bool{"avi": true, "mkv": true, "mp4": true}
	subtitleExtList := map[string]bool{"srt": true, "ass": true, "smi": true}

	var result ScanResult

	err := afero.Walk(FileSystem, targetPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Dir(path) != targetPath {
			return nil
		}

		ext := strings.TrimPrefix(filepath.Ext(path), ".")
		name := filepath.Base(path)
		baseName := strings.TrimSuffix(name, filepath.Ext(name))

		if movieExtList[ext] {
			result.Movies = append(result.Movies, MediaFile{
				Path:      path,
				Name:      name,
				BaseName:  baseName,
				Extension: "." + ext,
				Kind:      MovieKind,
			})
		}
		if subtitleExtList[ext] {
			result.Subtitles = append(result.Subtitles, MediaFile{
				Path:      path,
				Name:      name,
				BaseName:  baseName,
				Extension: "." + ext,
				Kind:      SubtitleKind,
			})
		}
		return nil
	})
	if err != nil {
		return ScanResult{}, err
	}

	sort.Slice(result.Movies, func(i, j int) bool {
		return result.Movies[i].Name < result.Movies[j].Name
	})
	sort.Slice(result.Subtitles, func(i, j int) bool {
		return result.Subtitles[i].Path < result.Subtitles[j].Path
	})

	return result, nil
}
