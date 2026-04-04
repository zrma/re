package re

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/spf13/afero"
	"golang.org/x/text/unicode/norm"
)

const ruleSkipReason = "episode pattern not recognized by rule matcher"
const duplicateRenameTargetSkipReason = "rename target conflicts with another planned rename"
const duplicateRenameSourceSkipReason = "rename source is referenced by multiple planned renames"
const existingSubtitleTargetSkipReason = "rename target already exists as another subtitle"
const existingPathTargetSkipReason = "rename target already exists on disk"
const inspectTargetSkipReason = "rename target could not be inspected safely"
const noMatchingMovieSkipReason = "no unique movie matched extracted episode"
const ambiguousMovieMatchSkipReason = "multiple movies matched same episode"
const temporaryArtifactSkipReason = "leftover internal temporary rename artifact detected"

type RenameOperation struct {
	Episode         string  `json:"episode,omitempty"`
	SourcePath      string  `json:"source_path"`
	DestinationPath string  `json:"destination_path"`
	DestinationName string  `json:"destination_name"`
	MoviePath       string  `json:"movie_path,omitempty"`
	MatchSource     string  `json:"match_source"`
	Confidence      float64 `json:"confidence,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}

type SkipOperation struct {
	SourcePath string `json:"source_path"`
	Reason     string `json:"reason"`
}

type RenamePlan struct {
	Operations         []RenameOperation
	Skips              []SkipOperation
	ResolvedMoviePaths []string
}

func BuildRenamePlan(resolution ResolutionResult) RenamePlan {
	var plan RenamePlan

	for _, episode := range resolution.Episodes {
		movie := resolution.MoviesByEpisode[episode]
		subs := resolution.SubtitlesByEpisode[episode]

		for _, sub := range subs {
			newName := movie.BaseName + sub.Extension
			destinationPath := filepath.Join(filepath.Dir(sub.Path), newName)
			if destinationPath == sub.Path {
				continue
			}
			plan.Operations = append(plan.Operations, RenameOperation{
				Episode:         episode,
				SourcePath:      sub.Path,
				DestinationPath: destinationPath,
				DestinationName: newName,
				MoviePath:       movie.Path,
				MatchSource:     "rule",
				Confidence:      1,
			})
		}
	}

	for _, episode := range unmatchedSubtitleEpisodes(resolution) {
		reason := noMatchingMovieSkipReason
		if resolution.AmbiguousMovieEpisodes[episode] {
			reason = ambiguousMovieMatchSkipReason
		}
		for _, sub := range resolution.SubtitlesByEpisode[episode] {
			plan.Skips = append(plan.Skips, SkipOperation{
				SourcePath: sub.Path,
				Reason:     reason,
			})
		}
	}

	for _, sub := range resolution.UnresolvedSubtitles {
		plan.Skips = append(plan.Skips, SkipOperation{
			SourcePath: sub.Path,
			Reason:     ruleSkipReason,
		})
	}

	return plan
}

func EnforceSafeRenamePlan(plan RenamePlan, scanResult ScanResult) RenamePlan {
	filtered := RenamePlan{
		Skips:              append([]SkipOperation{}, plan.Skips...),
		ResolvedMoviePaths: append([]string{}, plan.ResolvedMoviePaths...),
	}

	for _, artifactPath := range scanResult.TemporaryArtifacts {
		if hasSkipForSource(filtered.Skips, artifactPath, temporaryArtifactSkipReason) {
			continue
		}
		filtered.Skips = append(filtered.Skips, SkipOperation{
			SourcePath: artifactPath,
			Reason:     temporaryArtifactSkipReason,
		})
	}

	operationsBySource := map[string][]RenameOperation{}
	for _, operation := range plan.Operations {
		operationsBySource[operation.SourcePath] = append(operationsBySource[operation.SourcePath], operation)
	}

	subtitleSourcePaths := make([]string, 0, len(scanResult.Subtitles))
	for _, subtitle := range scanResult.Subtitles {
		subtitleSourcePaths = append(subtitleSourcePaths, subtitle.Path)
	}
	subtitleSources := newExistingPathSet(subtitleSourcePaths)

	sourceUniqueOperations := make([]RenameOperation, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		if conflictingOperations := operationsBySource[operation.SourcePath]; len(conflictingOperations) > 1 {
			if !hasSkipForSource(filtered.Skips, operation.SourcePath, duplicateRenameSourceSkipReason) {
				filtered.Skips = append(filtered.Skips, SkipOperation{
					SourcePath: operation.SourcePath,
					Reason:     duplicateRenameSourceSkipReason,
				})
			}
			continue
		}
		sourceUniqueOperations = append(sourceUniqueOperations, operation)
	}

	operationsByDestination := map[string][]RenameOperation{}
	for _, operation := range sourceUniqueOperations {
		operationsByDestination[operation.DestinationPath] = append(operationsByDestination[operation.DestinationPath], operation)
	}
	operationsByEquivalentDestination := buildEquivalentDestinationBuckets(sourceUniqueOperations)

	activeOperations := make([]RenameOperation, 0, len(sourceUniqueOperations))
	for _, operation := range sourceUniqueOperations {
		if conflictingOperations := conflictingDestinationOperations(operation, operationsByDestination, operationsByEquivalentDestination); len(conflictingOperations) > 1 {
			preferredSourcePath, hasPreferredSource := preferredDuplicateSourcePath(conflictingOperations)
			if !hasPreferredSource || operation.SourcePath != preferredSourcePath {
				filtered.Skips = append(filtered.Skips, SkipOperation{
					SourcePath: operation.SourcePath,
					Reason:     duplicateRenameTargetSkipReason,
				})
				continue
			}
		}
		activeOperations = append(activeOperations, operation)
	}

	for {
		// 다른 rename이 먼저 비워줄 대상 경로는 허용하고, 끝까지 비워지지 않는 경로만 제거한다.
		nextOperations := make([]RenameOperation, 0, len(activeOperations))
		removed := false
		for _, operation := range activeOperations {
			reason, blocked := blockedDestinationReason(operation, activeOperations, subtitleSources)
			if blocked {
				filtered.Skips = append(filtered.Skips, SkipOperation{
					SourcePath: operation.SourcePath,
					Reason:     reason,
				})
				removed = true
				continue
			}
			nextOperations = append(nextOperations, operation)
		}

		activeOperations = nextOperations
		if !removed {
			break
		}
	}

	filtered.Operations = activeOperations

	return filtered
}

func hasSkipForSource(skips []SkipOperation, sourcePath string, reason string) bool {
	for _, skip := range skips {
		if skip.SourcePath == sourcePath && skip.Reason == reason {
			return true
		}
	}
	return false
}

func conflictingDestinationOperations(
	operation RenameOperation,
	operationsByDestination map[string][]RenameOperation,
	operationsByEquivalentDestination []equivalentDestinationBucket,
) []RenameOperation {
	if conflictingOperations := operationsByDestination[operation.DestinationPath]; len(conflictingOperations) > 1 {
		return conflictingOperations
	}

	conflictingOperations := equivalentDestinationOperations(operation.DestinationPath, operationsByEquivalentDestination)
	if len(conflictingOperations) <= 1 {
		return nil
	}
	if !destinationPathsAliasOnCurrentFileSystem(conflictingOperations) {
		return nil
	}

	return conflictingOperations
}

type equivalentDestinationBucket struct {
	Representative string
	Operations     []RenameOperation
}

func buildEquivalentDestinationBuckets(operations []RenameOperation) []equivalentDestinationBucket {
	buckets := make([]equivalentDestinationBucket, 0, len(operations))

	for _, operation := range operations {
		matched := false
		for index := range buckets {
			if !pathNamesPotentiallyAlias(buckets[index].Representative, operation.DestinationPath) {
				continue
			}
			buckets[index].Operations = append(buckets[index].Operations, operation)
			matched = true
			break
		}
		if matched {
			continue
		}
		buckets = append(buckets, equivalentDestinationBucket{
			Representative: operation.DestinationPath,
			Operations:     []RenameOperation{operation},
		})
	}

	return buckets
}

func equivalentDestinationOperations(path string, buckets []equivalentDestinationBucket) []RenameOperation {
	for _, bucket := range buckets {
		if pathNamesPotentiallyAlias(bucket.Representative, path) {
			return bucket.Operations
		}
	}

	return nil
}

func destinationPathsAliasOnCurrentFileSystem(operations []RenameOperation) bool {
	requiresCaseInsensitiveAliasHandling := false
	requiresNormalizationInsensitiveAliasHandling := false

	for firstIndex := 0; firstIndex < len(operations); firstIndex++ {
		for secondIndex := firstIndex + 1; secondIndex < len(operations); secondIndex++ {
			switch {
			case strings.EqualFold(operations[firstIndex].DestinationPath, operations[secondIndex].DestinationPath):
				requiresCaseInsensitiveAliasHandling = true
			case pathNamesRequireNormalizationAliasHandling(operations[firstIndex].DestinationPath, operations[secondIndex].DestinationPath):
				requiresNormalizationInsensitiveAliasHandling = true
			}
		}
	}

	if !requiresCaseInsensitiveAliasHandling && !requiresNormalizationInsensitiveAliasHandling {
		return false
	}

	supportByDirectory := make(map[string]directoryAliasSupport, len(operations))
	for _, operation := range operations {
		directoryPath := filepath.Dir(operation.DestinationPath)
		support, ok := supportByDirectory[directoryPath]
		if !ok {
			support = detectDirectoryAliasSupport(directoryPath)
			supportByDirectory[directoryPath] = support
		}
		if requiresCaseInsensitiveAliasHandling && support.CaseInsensitive {
			return true
		}
		if requiresNormalizationInsensitiveAliasHandling && support.NormalizationInsensitive {
			return true
		}
	}

	return false
}

type directoryAliasSupport struct {
	CaseInsensitive          bool
	NormalizationInsensitive bool
}

func detectDirectoryAliasSupport(directoryPath string) directoryAliasSupport {
	return directoryAliasSupport{
		CaseInsensitive:          directoryHasCaseInsensitiveAlias(directoryPath),
		NormalizationInsensitive: directoryHasNormalizationInsensitiveAlias(directoryPath),
	}
}

func directoryHasCaseInsensitiveAlias(directoryPath string) bool {
	probePath, aliasPath, ok := directoryAliasProbePaths(directoryPath, "a", "A")
	if !ok {
		return false
	}

	return directoryProbeAliases(probePath, aliasPath)
}

func directoryHasNormalizationInsensitiveAlias(directoryPath string) bool {
	probePath, aliasPath, ok := directoryAliasProbePaths(directoryPath, "é", "e\u0301")
	if !ok {
		return false
	}

	return directoryProbeAliases(probePath, aliasPath)
}

func directoryAliasProbePaths(directoryPath string, baseName string, aliasBaseName string) (string, string, bool) {
	if baseName == aliasBaseName {
		return "", "", false
	}

	for index := 0; ; index++ {
		probeName := fmt.Sprintf(".re-fs-probe-%s.re-probe-%d.tmp", baseName, index)
		aliasName := fmt.Sprintf(".re-fs-probe-%s.re-probe-%d.tmp", aliasBaseName, index)
		probePath := filepath.Join(directoryPath, probeName)
		aliasPath := filepath.Join(directoryPath, aliasName)

		probeExists, err := pathEntryExists(probePath)
		if err != nil {
			return "", "", false
		}
		if probeExists {
			continue
		}

		aliasExists, err := pathEntryExists(aliasPath)
		if err != nil {
			return "", "", false
		}
		if aliasExists {
			continue
		}

		return probePath, aliasPath, true
	}
}

func directoryProbeAliases(probePath string, aliasPath string) bool {
	if err := afero.WriteFile(FileSystem, probePath, []byte("probe"), 0600); err != nil {
		return false
	}
	defer func() {
		_ = FileSystem.Remove(probePath)
	}()

	return isAliasedExistingPath(probePath, aliasPath)
}

func blockedDestinationReason(operation RenameOperation, activeOperations []RenameOperation, subtitleSources existingPathSet) (string, bool) {
	if operation.SourcePath == operation.DestinationPath {
		return "", false
	}

	exists, err := pathEntryExists(operation.DestinationPath)
	if err != nil {
		return inspectTargetSkipReason, true
	}
	if !exists {
		return "", false
	}
	if isSameExistingPath(operation.SourcePath, operation.DestinationPath) {
		return "", false
	}
	if pathFreedByPendingRename(operation.DestinationPath, activeOperations) {
		return "", false
	}
	if subtitleSources.Contains(operation.DestinationPath) {
		return existingSubtitleTargetSkipReason, true
	}
	return existingPathTargetSkipReason, true
}

type existingPathSet struct {
	exact map[string]struct{}
	paths []string
}

func newExistingPathSet(paths []string) existingPathSet {
	set := existingPathSet{
		exact: make(map[string]struct{}, len(paths)),
		paths: make([]string, 0, len(paths)),
	}

	for _, path := range paths {
		if _, ok := set.exact[path]; ok {
			continue
		}
		set.exact[path] = struct{}{}
		set.paths = append(set.paths, path)
	}

	return set
}

func pathEntryExists(path string) (bool, error) {
	if lstater, ok := FileSystem.(afero.Lstater); ok {
		if _, _, err := lstater.LstatIfPossible(path); err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	}

	return afero.Exists(FileSystem, path)
}

func (set existingPathSet) Contains(path string) bool {
	if _, ok := set.exact[path]; ok {
		return true
	}

	for _, existingPath := range set.paths {
		if !pathNamesPotentiallyAlias(existingPath, path) {
			continue
		}
		if isSameExistingPath(existingPath, path) {
			return true
		}
	}

	return false
}

func pathFreedByPendingRename(path string, operations []RenameOperation) bool {
	for _, operation := range operations {
		if !isSameExistingPath(operation.SourcePath, path) {
			continue
		}
		if isSameExistingPath(operation.DestinationPath, path) {
			continue
		}
		return true
	}

	return false
}

func pathNamesPotentiallyAlias(firstPath string, secondPath string) bool {
	return strings.EqualFold(norm.NFC.String(firstPath), norm.NFC.String(secondPath))
}

func pathNamesRequireNormalizationAliasHandling(firstPath string, secondPath string) bool {
	if strings.EqualFold(firstPath, secondPath) {
		return false
	}

	return pathNamesPotentiallyAlias(firstPath, secondPath)
}

func preferredDuplicateSourcePath(operations []RenameOperation) (string, bool) {
	preferredSourcePath, ok := uniquePreferredSourcePath(operations, func(operation RenameOperation) bool {
		return isSameExistingPath(operation.SourcePath, operation.DestinationPath)
	})
	if ok {
		return preferredSourcePath, true
	}

	return uniquePreferredSourcePath(operations, func(operation RenameOperation) bool {
		return pathNamesPotentiallyAlias(filepath.Base(operation.SourcePath), operation.DestinationName)
	})
}

func uniquePreferredSourcePath(operations []RenameOperation, predicate func(RenameOperation) bool) (string, bool) {
	preferredSourcePath := ""
	for _, operation := range operations {
		if !predicate(operation) {
			continue
		}
		if preferredSourcePath != "" {
			return "", false
		}
		preferredSourcePath = operation.SourcePath
	}

	if preferredSourcePath == "" {
		return "", false
	}

	return preferredSourcePath, true
}

func pathHasCaseInsensitiveAlias(path string) bool {
	aliasPath, ok := alternateCasePath(path)
	if !ok {
		return false
	}

	return isAliasedExistingPath(path, aliasPath)
}

func pathHasNormalizationInsensitiveAlias(path string) bool {
	for _, aliasPath := range normalizationVariants(path) {
		if aliasPath == path {
			continue
		}
		if isAliasedExistingPath(path, aliasPath) {
			return true
		}
	}

	return false
}

func normalizationVariants(path string) []string {
	variants := []string{
		norm.NFC.String(path),
		norm.NFD.String(path),
	}
	if variants[0] == variants[1] {
		return variants[:1]
	}
	return variants
}

func hasDistinctDirectoryEntries(path string, aliasPath string) bool {
	entries, err := afero.ReadDir(FileSystem, filepath.Dir(path))
	if err != nil {
		return false
	}

	baseName := filepath.Base(path)
	aliasBaseName := filepath.Base(aliasPath)
	foundBaseName := false
	foundAliasBaseName := false

	for _, entry := range entries {
		switch entry.Name() {
		case baseName:
			foundBaseName = true
		case aliasBaseName:
			foundAliasBaseName = true
		}
		if foundBaseName && foundAliasBaseName {
			return true
		}
	}

	return false
}

func alternateCasePath(path string) (string, bool) {
	baseName, ok := alternateCaseString(filepath.Base(path))
	if !ok {
		return "", false
	}

	return filepath.Join(filepath.Dir(path), baseName), true
}

func alternateCaseString(value string) (string, bool) {
	runes := []rune(value)
	for index, r := range runes {
		alternate := unicode.SimpleFold(r)
		if alternate == r {
			continue
		}
		runes[index] = alternate
		return string(runes), true
	}

	return "", false
}

func isSameExistingPath(sourcePath string, destinationPath string) bool {
	if sourcePath == destinationPath {
		return true
	}
	if !pathNamesPotentiallyAlias(sourcePath, destinationPath) {
		return false
	}

	return isAliasedExistingPath(sourcePath, destinationPath)
}

func isAliasedExistingPath(sourcePath string, destinationPath string) bool {
	sourceInfo, err := FileSystem.Stat(sourcePath)
	if err != nil {
		return false
	}
	destinationInfo, err := FileSystem.Stat(destinationPath)
	if err != nil {
		return false
	}

	if !os.SameFile(sourceInfo, destinationInfo) {
		return false
	}

	// case-sensitive 파일시스템에서 대소문자만 다른 별도 hard link 엔트리는 안전한 alias가 아니다.
	return !hasDistinctDirectoryEntries(sourcePath, destinationPath)
}

func PreviewRenamePlan(writer io.Writer, plan RenamePlan) {
	for _, operation := range plan.Operations {
		if operation.MatchSource == "ai" {
			fmt.Fprintf(writer, "[ai:%.2f] %s -> %s\n", operation.Confidence, operation.SourcePath, operation.DestinationName)
			continue
		}
		fmt.Fprintf(writer, "%s -> %s\n", operation.SourcePath, operation.DestinationName)
	}

	for _, skip := range plan.Skips {
		fmt.Fprintf(writer, "[skip] %s (%s)\n", skip.SourcePath, skip.Reason)
	}
}

func ApplyRenamePlan(plan RenamePlan) error {
	pending := make([]RenameOperation, 0, len(plan.Operations))
	for _, operation := range plan.Operations {
		if operation.SourcePath == operation.DestinationPath {
			continue
		}
		pending = append(pending, operation)
	}

	appliedSteps := make([]renameStep, 0, len(pending))
	applyRename := func(sourcePath string, destinationPath string) error {
		if err := ensureRenameDestinationIsAvailable(sourcePath, destinationPath); err != nil {
			return wrapRenameFailure(sourcePath, destinationPath, err, rollbackRenameSteps(appliedSteps))
		}
		if err := FileSystem.Rename(sourcePath, destinationPath); err != nil {
			return wrapRenameFailure(sourcePath, destinationPath, err, rollbackRenameSteps(appliedSteps))
		}
		appliedSteps = append(appliedSteps, renameStep{
			SourcePath:      sourcePath,
			DestinationPath: destinationPath,
		})
		return nil
	}

	for len(pending) > 0 {
		nextPending := make([]RenameOperation, 0, len(pending))
		progress := false
		for _, operation := range pending {
			if isSameExistingPath(operation.SourcePath, operation.DestinationPath) {
				if err := applyRename(operation.SourcePath, operation.DestinationPath); err != nil {
					return err
				}
				progress = true
				continue
			}
			if pathFreedByPendingRename(operation.DestinationPath, pending) {
				nextPending = append(nextPending, operation)
				continue
			}
			if err := applyRename(operation.SourcePath, operation.DestinationPath); err != nil {
				return err
			}
			progress = true
		}
		if progress {
			pending = nextPending
			continue
		}

		// 남은 작업이 순환 그래프면 첫 source를 임시 이름으로 옮겨 사이클을 끊는다.
		tempPath, err := temporaryRenamePath(pending[0].SourcePath)
		if err != nil {
			return err
		}
		if err := applyRename(pending[0].SourcePath, tempPath); err != nil {
			return err
		}
		pending[0].SourcePath = tempPath
	}

	return nil
}

func ensureRenameDestinationIsAvailable(sourcePath string, destinationPath string) error {
	if sourcePath == destinationPath || isSameExistingPath(sourcePath, destinationPath) {
		return nil
	}

	exists, err := pathEntryExists(destinationPath)
	if err != nil {
		return fmt.Errorf("inspect destination %s: %w", destinationPath, err)
	}
	if exists {
		return fmt.Errorf("destination already exists at apply time")
	}

	return nil
}

type renameStep struct {
	SourcePath      string
	DestinationPath string
}

func rollbackRenameSteps(appliedSteps []renameStep) error {
	var rollbackErrors []error

	for index := len(appliedSteps) - 1; index >= 0; index-- {
		step := appliedSteps[index]
		if err := FileSystem.Rename(step.DestinationPath, step.SourcePath); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("rollback rename %s -> %s: %w", step.DestinationPath, step.SourcePath, err))
		}
	}

	return errors.Join(rollbackErrors...)
}

func wrapRenameFailure(sourcePath string, destinationPath string, renameErr error, rollbackErr error) error {
	renameFailure := fmt.Errorf("rename %s -> %s: %w", sourcePath, destinationPath, renameErr)
	if rollbackErr == nil {
		return renameFailure
	}
	return errors.Join(renameFailure, fmt.Errorf("rollback failed: %w", rollbackErr))
}

func temporaryRenamePath(sourcePath string) (string, error) {
	dir := filepath.Dir(sourcePath)
	ext := filepath.Ext(sourcePath)
	base := strings.TrimSuffix(filepath.Base(sourcePath), ext)

	for index := 0; ; index++ {
		candidate := filepath.Join(dir, "."+base+".re-tmp-"+strconv.Itoa(index)+ext)
		exists, err := pathEntryExists(candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
}
