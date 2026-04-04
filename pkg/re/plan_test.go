package re

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
)

func TestPathHasCaseInsensitiveAliasIgnoresDistinctHardLinkVariant(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "alpha.srt")
	aliasPath := filepath.Join(basePath, "Alpha.srt")

	if err := afero.WriteFile(FileSystem, sourcePath, []byte("subtitle"), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.Link(sourcePath, aliasPath); err != nil {
		t.Skipf("filesystem does not allow distinct hard-link case variants: %v", err)
	}

	sourceInfo, err := FileSystem.Stat(sourcePath)
	if err != nil {
		t.Fatalf("Stat(source) error = %v", err)
	}
	aliasInfo, err := FileSystem.Stat(aliasPath)
	if err != nil {
		t.Fatalf("Stat(alias) error = %v", err)
	}
	if !os.SameFile(sourceInfo, aliasInfo) {
		t.Fatalf("hard-link alias must reference the same file")
	}

	if !hasDistinctDirectoryEntries(sourcePath, aliasPath) {
		t.Fatalf("directory entries must remain distinct on a case-sensitive filesystem")
	}
	if pathHasCaseInsensitiveAlias(sourcePath) {
		t.Fatalf("distinct hard-link case variants must not be treated as a case-insensitive alias")
	}
}

func TestIsSameExistingPathIgnoresDistinctHardLinkVariant(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "alpha.srt")
	aliasPath := filepath.Join(basePath, "Alpha.srt")

	if err := afero.WriteFile(FileSystem, sourcePath, []byte("subtitle"), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.Link(sourcePath, aliasPath); err != nil {
		t.Skipf("filesystem does not allow distinct hard-link case variants: %v", err)
	}

	if isSameExistingPath(sourcePath, aliasPath) {
		t.Fatalf("distinct hard-link case variants must not be treated as the same existing path")
	}
}

func TestIsSameExistingPathRecognizesNormalizationEquivalentAlias(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "Café.srt")
	aliasPath := filepath.Join(basePath, "Cafe\u0301.srt")

	if err := afero.WriteFile(FileSystem, sourcePath, []byte("subtitle"), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}

	sourceInfo, err := FileSystem.Stat(sourcePath)
	if err != nil {
		t.Fatalf("Stat(source) error = %v", err)
	}
	aliasInfo, err := FileSystem.Stat(aliasPath)
	if err != nil || !os.SameFile(sourceInfo, aliasInfo) {
		t.Skipf("filesystem does not alias normalization-equivalent paths: %v", err)
	}

	if !isSameExistingPath(sourcePath, aliasPath) {
		t.Fatalf("normalization-equivalent alias paths must be treated as the same existing path")
	}
}

func TestAlternateCaseStringUsesUnicodeSimpleFold(t *testing.T) {
	alternate, ok := alternateCaseString("ΟΣ.srt")
	if !ok {
		t.Fatalf("alternateCaseString() ok = false, want true")
	}
	if alternate == "ΟΣ.srt" {
		t.Fatalf("alternateCaseString() returned original value")
	}
	if !pathNamesPotentiallyAlias("ΟΣ.srt", alternate) {
		t.Fatalf("alternateCaseString() = %q, want unicode case-fold alias", alternate)
	}
}

func TestDirectoryAliasProbePathsFindsSlotBeyondPreexistingProbeArtifacts(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	for index := 0; index < 32; index++ {
		probePath := filepath.Join(basePath, fmt.Sprintf(".re-fs-probe-a.re-probe-%d.tmp", index))
		if err := afero.WriteFile(FileSystem, probePath, []byte("probe"), 0644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", probePath, err)
		}
	}

	probePath, aliasPath, ok := directoryAliasProbePaths(basePath, "a", "A")
	if !ok {
		t.Fatalf("directoryAliasProbePaths() ok = false, want true")
	}
	if got, want := filepath.Base(probePath), ".re-fs-probe-a.re-probe-32.tmp"; got != want {
		t.Fatalf("directoryAliasProbePaths() probe = %q, want %q", got, want)
	}
	if got, want := filepath.Base(aliasPath), ".re-fs-probe-A.re-probe-32.tmp"; got != want {
		t.Fatalf("directoryAliasProbePaths() alias = %q, want %q", got, want)
	}
}

func TestEnforceSafeRenamePlanUsesMoviePathToDetectUnicodeAliasDestinationConflict(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	firstMoviePath := filepath.Join(basePath, "ΟΣ.mkv")
	secondMoviePath := filepath.Join(basePath, "ος.mkv")
	firstSourcePath := filepath.Join(basePath, "자막1.srt")
	secondSourcePath := filepath.Join(basePath, "자막2.srt")
	firstDestinationPath := filepath.Join(basePath, "ΟΣ.srt")
	secondDestinationPath := filepath.Join(basePath, "ος.srt")

	if err := afero.WriteFile(FileSystem, firstMoviePath, []byte("movie"), 0644); err != nil {
		t.Fatalf("WriteFile(movie) error = %v", err)
	}
	if err := afero.WriteFile(FileSystem, firstSourcePath, []byte("subtitle-1"), 0644); err != nil {
		t.Fatalf("WriteFile(source1) error = %v", err)
	}
	if err := afero.WriteFile(FileSystem, secondSourcePath, []byte("subtitle-2"), 0644); err != nil {
		t.Fatalf("WriteFile(source2) error = %v", err)
	}

	firstMovieInfo, err := FileSystem.Stat(firstMoviePath)
	if err != nil {
		t.Fatalf("Stat(movie) error = %v", err)
	}
	secondMovieInfo, err := FileSystem.Stat(secondMoviePath)
	if err != nil || !os.SameFile(firstMovieInfo, secondMovieInfo) {
		t.Skipf("filesystem does not alias unicode case-fold movie paths: %v", err)
	}

	plan := RenamePlan{
		Operations: []RenameOperation{
			{
				SourcePath:      firstSourcePath,
				DestinationPath: firstDestinationPath,
				DestinationName: filepath.Base(firstDestinationPath),
				MoviePath:       firstMoviePath,
				MatchSource:     "ai",
			},
			{
				SourcePath:      secondSourcePath,
				DestinationPath: secondDestinationPath,
				DestinationName: filepath.Base(secondDestinationPath),
				MoviePath:       secondMoviePath,
				MatchSource:     "ai",
			},
		},
	}
	scanResult := ScanResult{
		Subtitles: []MediaFile{
			{Path: firstSourcePath},
			{Path: secondSourcePath},
		},
	}

	safePlan := EnforceSafeRenamePlan(plan, scanResult)
	if len(safePlan.Operations) != 0 {
		t.Fatalf("EnforceSafeRenamePlan() operations = %d, want 0", len(safePlan.Operations))
	}
	if len(safePlan.Skips) != 2 {
		t.Fatalf("EnforceSafeRenamePlan() skips = %d, want 2", len(safePlan.Skips))
	}

	for _, skip := range safePlan.Skips {
		if skip.Reason != duplicateRenameTargetSkipReason {
			t.Fatalf("EnforceSafeRenamePlan() skip reason = %q, want %q", skip.Reason, duplicateRenameTargetSkipReason)
		}
	}
}

func TestEnforceSafeRenamePlanDetectsNormalizationAliasDestinationConflictWithASCIISources(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	firstSourcePath := filepath.Join(basePath, "first.srt")
	secondSourcePath := filepath.Join(basePath, "second.srt")
	firstDestinationPath := filepath.Join(basePath, "Café.srt")
	secondDestinationPath := filepath.Join(basePath, "Cafe\u0301.srt")

	if err := afero.WriteFile(FileSystem, firstSourcePath, []byte("subtitle-1"), 0644); err != nil {
		t.Fatalf("WriteFile(source1) error = %v", err)
	}
	if err := afero.WriteFile(FileSystem, secondSourcePath, []byte("subtitle-2"), 0644); err != nil {
		t.Fatalf("WriteFile(source2) error = %v", err)
	}

	firstDestinationInfo, err := FileSystem.Stat(firstDestinationPath)
	if err != nil {
		t.Skipf("filesystem does not alias normalization-equivalent destination paths: %v", err)
	}
	secondDestinationInfo, err := FileSystem.Stat(secondDestinationPath)
	if err != nil || !os.SameFile(firstDestinationInfo, secondDestinationInfo) {
		t.Skipf("filesystem does not alias normalization-equivalent destination paths: %v", err)
	}

	plan := RenamePlan{
		Operations: []RenameOperation{
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
	scanResult := ScanResult{
		Subtitles: []MediaFile{
			{Path: firstSourcePath},
			{Path: secondSourcePath},
		},
	}

	safePlan := EnforceSafeRenamePlan(plan, scanResult)
	if len(safePlan.Operations) != 0 {
		t.Fatalf("EnforceSafeRenamePlan() operations = %d, want 0", len(safePlan.Operations))
	}
	if len(safePlan.Skips) != 2 {
		t.Fatalf("EnforceSafeRenamePlan() skips = %d, want 2", len(safePlan.Skips))
	}

	for _, skip := range safePlan.Skips {
		if skip.Reason != duplicateRenameTargetSkipReason {
			t.Fatalf("EnforceSafeRenamePlan() skip reason = %q, want %q", skip.Reason, duplicateRenameTargetSkipReason)
		}
	}
}

func TestApplyRenamePlanRejectsDistinctHardLinkCaseVariantDestination(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "alpha.srt")
	destinationPath := filepath.Join(basePath, "Alpha.srt")

	if err := afero.WriteFile(FileSystem, sourcePath, []byte("subtitle"), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.Link(sourcePath, destinationPath); err != nil {
		t.Skipf("filesystem does not allow distinct hard-link case variants: %v", err)
	}

	err := ApplyRenamePlan(RenamePlan{
		Operations: []RenameOperation{
			{
				SourcePath:      sourcePath,
				DestinationPath: destinationPath,
				DestinationName: filepath.Base(destinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
		},
	})
	if err == nil {
		t.Fatalf("ApplyRenamePlan() error = nil, want destination conflict")
	}

	if _, statErr := FileSystem.Stat(sourcePath); statErr != nil {
		t.Fatalf("Stat(source) error = %v", statErr)
	}
	if _, statErr := FileSystem.Stat(destinationPath); statErr != nil {
		t.Fatalf("Stat(destination) error = %v", statErr)
	}
}

func TestEnforceSafeRenamePlanSkipsDistinctHardLinkCaseVariantDestination(t *testing.T) {
	FileSystem = afero.NewOsFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := t.TempDir()
	sourcePath := filepath.Join(basePath, "alpha.srt")
	destinationPath := filepath.Join(basePath, "Alpha.srt")

	if err := afero.WriteFile(FileSystem, sourcePath, []byte("subtitle"), 0644); err != nil {
		t.Fatalf("WriteFile(source) error = %v", err)
	}
	if err := os.Link(sourcePath, destinationPath); err != nil {
		t.Skipf("filesystem does not allow distinct hard-link case variants: %v", err)
	}

	plan := RenamePlan{
		Operations: []RenameOperation{
			{
				SourcePath:      sourcePath,
				DestinationPath: destinationPath,
				DestinationName: filepath.Base(destinationPath),
				MatchSource:     "rule",
				Confidence:      1,
			},
		},
	}
	scanResult := ScanResult{
		Subtitles: []MediaFile{
			{Path: sourcePath},
			{Path: destinationPath},
		},
	}

	safePlan := EnforceSafeRenamePlan(plan, scanResult)
	if len(safePlan.Operations) != 0 {
		t.Fatalf("EnforceSafeRenamePlan() operations = %d, want 0", len(safePlan.Operations))
	}
	if len(safePlan.Skips) != 1 {
		t.Fatalf("EnforceSafeRenamePlan() skips = %d, want 1", len(safePlan.Skips))
	}
	if safePlan.Skips[0].Reason != existingSubtitleTargetSkipReason {
		t.Fatalf("EnforceSafeRenamePlan() skip reason = %q, want %q", safePlan.Skips[0].Reason, existingSubtitleTargetSkipReason)
	}
}

func TestDestinationPathsAliasOnCurrentFileSystemDoesNotAssumeNormalizationAlias(t *testing.T) {
	FileSystem = afero.NewMemMapFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder")
	firstSourcePath := filepath.Join(basePath, "first.srt")
	secondSourcePath := filepath.Join(basePath, "second.srt")

	if err := afero.WriteFile(FileSystem, firstSourcePath, []byte("first"), 0644); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}
	if err := afero.WriteFile(FileSystem, secondSourcePath, []byte("second"), 0644); err != nil {
		t.Fatalf("WriteFile(second) error = %v", err)
	}

	operations := []RenameOperation{
		{
			SourcePath:      firstSourcePath,
			DestinationPath: filepath.Join(basePath, "Café.srt"),
			DestinationName: "Café.srt",
			MatchSource:     "ai",
		},
		{
			SourcePath:      secondSourcePath,
			DestinationPath: filepath.Join(basePath, "Cafe\u0301.srt"),
			DestinationName: "Cafe\u0301.srt",
			MatchSource:     "ai",
		},
	}

	if destinationPathsAliasOnCurrentFileSystem(operations) {
		t.Fatalf("destinationPathsAliasOnCurrentFileSystem() = true, want false without filesystem alias evidence")
	}
}

func TestEnforceSafeRenamePlanAllowsNormalizationDistinctTargetsOnNormalizationSensitiveFS(t *testing.T) {
	FileSystem = afero.NewMemMapFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder")
	firstSourcePath := filepath.Join(basePath, "first.srt")
	secondSourcePath := filepath.Join(basePath, "second.srt")
	firstDestinationPath := filepath.Join(basePath, "Café.srt")
	secondDestinationPath := filepath.Join(basePath, "Cafe\u0301.srt")

	if err := afero.WriteFile(FileSystem, firstSourcePath, []byte("first"), 0644); err != nil {
		t.Fatalf("WriteFile(first) error = %v", err)
	}
	if err := afero.WriteFile(FileSystem, secondSourcePath, []byte("second"), 0644); err != nil {
		t.Fatalf("WriteFile(second) error = %v", err)
	}

	plan := RenamePlan{
		Operations: []RenameOperation{
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
	scanResult := ScanResult{
		Subtitles: []MediaFile{
			{Path: firstSourcePath},
			{Path: secondSourcePath},
		},
	}

	safePlan := EnforceSafeRenamePlan(plan, scanResult)
	if len(safePlan.Operations) != 2 {
		t.Fatalf("EnforceSafeRenamePlan() operations = %d, want 2", len(safePlan.Operations))
	}
	if len(safePlan.Skips) != 0 {
		t.Fatalf("EnforceSafeRenamePlan() skips = %d, want 0", len(safePlan.Skips))
	}
}

func TestEnforceSafeRenamePlanDuplicateSourceDoesNotBlockValidTarget(t *testing.T) {
	FileSystem = afero.NewMemMapFs()
	defer func() { FileSystem = afero.NewOsFs() }()

	basePath := filepath.Join("home", "folder")
	duplicateSourcePath := filepath.Join(basePath, "duplicate.srt")
	validSourcePath := filepath.Join(basePath, "valid.srt")
	validDestinationPath := filepath.Join(basePath, "target-a.srt")

	if err := afero.WriteFile(FileSystem, duplicateSourcePath, []byte("duplicate"), 0644); err != nil {
		t.Fatalf("WriteFile(duplicate) error = %v", err)
	}
	if err := afero.WriteFile(FileSystem, validSourcePath, []byte("valid"), 0644); err != nil {
		t.Fatalf("WriteFile(valid) error = %v", err)
	}

	plan := RenamePlan{
		Operations: []RenameOperation{
			{
				SourcePath:      duplicateSourcePath,
				DestinationPath: validDestinationPath,
				DestinationName: filepath.Base(validDestinationPath),
				MatchSource:     "ai",
			},
			{
				SourcePath:      duplicateSourcePath,
				DestinationPath: filepath.Join(basePath, "target-b.srt"),
				DestinationName: "target-b.srt",
				MatchSource:     "ai",
			},
			{
				SourcePath:      validSourcePath,
				DestinationPath: validDestinationPath,
				DestinationName: filepath.Base(validDestinationPath),
				MatchSource:     "ai",
			},
		},
	}
	scanResult := ScanResult{
		Subtitles: []MediaFile{
			{Path: duplicateSourcePath},
			{Path: validSourcePath},
		},
	}

	safePlan := EnforceSafeRenamePlan(plan, scanResult)
	if len(safePlan.Operations) != 1 {
		t.Fatalf("EnforceSafeRenamePlan() operations = %d, want 1", len(safePlan.Operations))
	}
	if safePlan.Operations[0].SourcePath != validSourcePath {
		t.Fatalf("EnforceSafeRenamePlan() kept %q, want %q", safePlan.Operations[0].SourcePath, validSourcePath)
	}
	if len(safePlan.Skips) != 1 {
		t.Fatalf("EnforceSafeRenamePlan() skips = %d, want 1", len(safePlan.Skips))
	}
	if safePlan.Skips[0].SourcePath != duplicateSourcePath {
		t.Fatalf("EnforceSafeRenamePlan() skipped %q, want %q", safePlan.Skips[0].SourcePath, duplicateSourcePath)
	}
	if safePlan.Skips[0].Reason != duplicateRenameSourceSkipReason {
		t.Fatalf("EnforceSafeRenamePlan() skip reason = %q, want %q", safePlan.Skips[0].Reason, duplicateRenameSourceSkipReason)
	}
}
