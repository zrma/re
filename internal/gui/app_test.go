package gui

import (
	"path/filepath"
	"testing"

	fynetest "fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/zrma/re/pkg/re"
)

func newTestApplication(t *testing.T) *application {
	t.Helper()

	a := fynetest.NewApp()
	t.Cleanup(func() {
		a.Quit()
	})

	window := a.NewWindow("test")
	ui := &application{window: window}
	ui.initWidgets()
	ui.renderIdleState()
	window.SetContent(ui.buildContent())
	return ui
}

func fakePreview(targetPath string) re.PreviewResult {
	options := re.DefaultRunOptions()
	return re.PreviewResult{
		TargetPath: targetPath,
		Options:    options,
		Plan: re.RenamePlan{
			Operations: []re.RenameOperation{
				{
					SourcePath:      filepath.Join(targetPath, "1화.srt"),
					DestinationName: "[BD] Example - 01.srt",
					DestinationPath: filepath.Join(targetPath, "[BD] Example - 01.srt"),
					MatchSource:     "rule",
					Episode:         "01",
				},
			},
		},
		Report: re.RunReport{
			TargetPath: targetPath,
			Status:     "canceled",
			Summary: re.RunSummary{
				PlannedRenames: 1,
				RuleRenames:    1,
			},
		},
	}
}

func TestPreparePreviewRefreshClearsExistingPreviewAndDisablesApply(t *testing.T) {
	ui := newTestApplication(t)

	targetPath := filepath.Join("tmp", "preview")
	ui.targetEntry.SetText(targetPath)
	ui.setPreview(fakePreview(targetPath))

	require.False(t, ui.applyButton.Disabled())
	require.NotNil(t, ui.preview)

	nextTarget := filepath.Join("tmp", "missing-target")
	ui.targetEntry.SetText(nextTarget)
	path, options, ok := ui.preparePreviewRefresh()

	require.True(t, ok)
	assert.Equal(t, nextTarget, path)
	assert.False(t, options.AI.Enabled)
	assert.Nil(t, ui.preview)
	assert.True(t, ui.applyButton.Disabled())
	assert.True(t, ui.operationsEmpty.Visible())
	assert.True(t, ui.loading)
	assert.Equal(t, "미리보기를 생성하는 중입니다...", ui.statusLabel.Text)
}

func TestRenderIdleStateShowsEmptyViewsAndDisablesApply(t *testing.T) {
	ui := newTestApplication(t)

	assert.Nil(t, ui.preview)
	assert.True(t, ui.operationsEmpty.Visible())
	assert.True(t, ui.skipsEmpty.Visible())
	assert.False(t, ui.operationsTable.Visible())
	assert.False(t, ui.skipsTable.Visible())
	assert.True(t, ui.applyButton.Disabled())
	assert.Equal(t, "준비됨", ui.statusLabel.Text)
}

func TestHandleApplyErrorForExpiredPreviewClearsPreviewAndDisablesApply(t *testing.T) {
	ui := newTestApplication(t)

	targetPath := filepath.Join("tmp", "preview")
	ui.targetEntry.SetText(targetPath)
	ui.setPreview(fakePreview(targetPath))

	require.False(t, ui.applyButton.Disabled())
	require.NotNil(t, ui.preview)

	ui.handleApplyError(re.PreviewExpiredError{
		Path:   filepath.Join(targetPath, "1화.srt"),
		Reason: "path contents changed after preview",
	})

	assert.Nil(t, ui.preview)
	assert.True(t, ui.applyButton.Disabled())
	assert.True(t, ui.operationsEmpty.Visible())
	assert.True(t, ui.errorBanner.Visible())
	assert.Contains(t, ui.errorLabel.Text, "preview 이후 폴더 상태가 바뀌었습니다")
}

func TestHandleInputChangeClearsStaleErrorAndShowsRefreshState(t *testing.T) {
	ui := newTestApplication(t)

	targetPath := filepath.Join("tmp", "preview")
	ui.targetEntry.SetText(targetPath)
	ui.setPreview(fakePreview(targetPath))
	ui.setBannerText(ui.errorBanner, ui.errorLabel, "old error")

	ui.targetEntry.SetText(filepath.Join("tmp", "other-preview"))

	assert.False(t, ui.errorBanner.Visible())
	assert.Equal(t, "", ui.errorLabel.Text)
	assert.Equal(t, "경로 또는 옵션이 바뀌었습니다. preview를 다시 생성하세요.", ui.statusLabel.Text)
	assert.True(t, ui.applyButton.Disabled())
}
