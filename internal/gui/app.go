package gui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/zrma/re/pkg/re"
)

const (
	operationColumnSource = iota
	operationColumnDestination
	operationColumnEpisode
	operationColumnExtension
	operationColumnMatch
	operationColumnConfidence
	operationColumnCount
)

const (
	skipColumnSource = iota
	skipColumnReason
	skipColumnCount
)

var operationHeaders = []string{
	"원본 파일",
	"변경 후 파일",
	"에피소드",
	"확장자",
	"출처",
	"신뢰도",
}

var skipHeaders = []string{
	"파일",
	"이유",
}

type operationRow struct {
	Source          string
	SourcePath      string
	Destination     string
	DestinationPath string
	Episode         string
	Extension       string
	Match           string
	Confidence      string
	ConfidenceValue float64
}

type skipRow struct {
	Source     string
	SourcePath string
	Reason     string
}

type application struct {
	window fyne.Window

	targetEntry      *widget.Entry
	aiFallbackCheck  *widget.Check
	browseButton     *widget.Button
	refreshButton    *widget.Button
	applyButton      *widget.Button
	statusLabel      *widget.Label
	warningLabel     *widget.Label
	warningButton    *widget.Button
	warningBanner    *fyne.Container
	errorLabel       *widget.Label
	errorBanner      *fyne.Container
	summaryLabel     *widget.Label
	summaryStatus    *widget.Label
	summaryRenames   *widget.Label
	summarySkips     *widget.Label
	summaryReview    *widget.Label
	detailFolder     *widget.Label
	detailSource     *widget.Label
	detailTarget     *widget.Label
	detailEpisode    *widget.Label
	detailMatch      *widget.Label
	detailConfidence *widget.Label
	operationsTable  *widget.Table
	skipsTable       *widget.Table
	operationsEmpty  *fyne.Container
	skipsEmpty       *fyne.Container
	operations       []operationRow
	skips            []skipRow
	preview          *re.PreviewResult
	selectedOpIndex  int
	loading          bool
}

func Run() {
	ui := newApplication()
	ui.window.ShowAndRun()
}

func newApplication() *application {
	a := app.NewWithID("com.zrma.re.gui")
	window := a.NewWindow("re")
	window.Resize(fyne.NewSize(1360, 820))

	ui := &application{
		window: window,
	}
	ui.initWidgets()
	ui.renderIdleState()

	window.SetContent(ui.buildContent())
	return ui
}

func (ui *application) initWidgets() {
	ui.targetEntry = widget.NewEntry()
	ui.targetEntry.SetPlaceHolder("대상 폴더를 선택하세요")
	ui.targetEntry.OnChanged = func(string) {
		ui.handleInputChange()
	}
	ui.targetEntry.OnSubmitted = func(string) {
		ui.refreshPreview()
	}

	ui.aiFallbackCheck = widget.NewCheck("AI 보조 판정", func(bool) {
		ui.handleInputChange()
	})

	ui.browseButton = widget.NewButton("폴더 선택", func() {
		openDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				ui.showError(err)
				return
			}
			if uri == nil {
				return
			}

			ui.targetEntry.SetText(uri.Path())
			ui.refreshPreview()
		}, ui.window)
		openDialog.Show()
	})

	ui.refreshButton = widget.NewButton("새로고침", func() {
		ui.refreshPreview()
	})

	ui.applyButton = widget.NewButton("적용", func() {
		ui.confirmApply()
	})
	ui.applyButton.Importance = widget.HighImportance

	ui.statusLabel = widget.NewLabel("")
	ui.statusLabel.Wrapping = fyne.TextWrapWord
	ui.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
	ui.warningLabel = widget.NewLabel("")
	ui.warningLabel.Wrapping = fyne.TextWrapWord
	ui.warningButton = widget.NewButton("자세히 보기", func() {
		ui.showWarningsDialog()
	})
	ui.warningButton.Importance = widget.LowImportance
	ui.warningBanner = newMessageBanner(
		theme.WarningIcon(),
		theme.Color(theme.ColorNameWarning),
		container.NewBorder(nil, nil, nil, ui.warningButton, ui.warningLabel),
	)
	ui.errorLabel = widget.NewLabel("")
	ui.errorLabel.Wrapping = fyne.TextWrapWord
	ui.errorBanner = newMessageBanner(theme.ErrorIcon(), theme.Color(theme.ColorNameError), ui.errorLabel)
	ui.summaryLabel = widget.NewLabel("")
	ui.summaryLabel.Wrapping = fyne.TextWrapWord
	ui.summaryStatus = newMetricValueLabel()
	ui.summaryRenames = newMetricValueLabel()
	ui.summarySkips = newMetricValueLabel()
	ui.summaryReview = newMetricValueLabel()
	ui.detailFolder = newDetailValueLabel()
	ui.detailSource = newDetailValueLabel()
	ui.detailTarget = newDetailValueLabel()
	ui.detailEpisode = newDetailValueLabel()
	ui.detailMatch = newDetailValueLabel()
	ui.detailConfidence = newDetailValueLabel()

	ui.operationsTable = widget.NewTable(
		func() (int, int) {
			if len(ui.operations) == 0 {
				return 1, operationColumnCount
			}
			return len(ui.operations) + 1, operationColumnCount
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapOff
			label.Truncation = fyne.TextTruncateEllipsis
			return label
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			label := object.(*widget.Label)
			label.SetText(ui.operationCellText(id))
		},
	)
	ui.operationsTable.OnSelected = func(id widget.TableCellID) {
		ui.selectOperation(id.Row - 1)
	}
	ui.operationsTable.SetColumnWidth(operationColumnSource, 300)
	ui.operationsTable.SetColumnWidth(operationColumnDestination, 360)
	ui.operationsTable.SetColumnWidth(operationColumnEpisode, 90)
	ui.operationsTable.SetColumnWidth(operationColumnExtension, 70)
	ui.operationsTable.SetColumnWidth(operationColumnMatch, 90)
	ui.operationsTable.SetColumnWidth(operationColumnConfidence, 100)
	ui.operationsTable.SetRowHeight(0, 36)
	ui.operationsEmpty = newEmptyState(theme.InfoIcon(), "적용 예정 변경 항목이 없습니다.")

	ui.skipsTable = widget.NewTable(
		func() (int, int) {
			if len(ui.skips) == 0 {
				return 1, skipColumnCount
			}
			return len(ui.skips) + 1, skipColumnCount
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.Wrapping = fyne.TextWrapOff
			label.Truncation = fyne.TextTruncateEllipsis
			return label
		},
		func(id widget.TableCellID, object fyne.CanvasObject) {
			label := object.(*widget.Label)
			label.SetText(ui.skipCellText(id))
		},
	)
	ui.skipsTable.SetColumnWidth(skipColumnSource, 280)
	ui.skipsTable.SetColumnWidth(skipColumnReason, 380)
	ui.skipsTable.SetRowHeight(0, 36)
	ui.skipsEmpty = newEmptyState(theme.ConfirmIcon(), "건너뛴 항목이 없습니다.")
}

func (ui *application) buildContent() fyne.CanvasObject {
	detailForm := widget.NewForm(
		widget.NewFormItem("폴더", ui.detailFolder),
		widget.NewFormItem("원본 파일", ui.detailSource),
		widget.NewFormItem("변경 후 파일", ui.detailTarget),
		widget.NewFormItem("에피소드", ui.detailEpisode),
		widget.NewFormItem("출처", ui.detailMatch),
		widget.NewFormItem("신뢰도", ui.detailConfidence),
	)

	operationsBody := container.NewStack(ui.operationsTable, ui.operationsEmpty)
	skipsBody := container.NewStack(ui.skipsTable, ui.skipsEmpty)

	controls := newSection(
		"대상 폴더",
		"폴더를 선택하거나 경로를 붙여넣은 뒤 미리보기를 생성하세요.",
		container.NewVBox(
			container.NewBorder(
				nil,
				nil,
				nil,
				container.NewHBox(
					ui.refreshButton,
					ui.browseButton,
				),
				ui.targetEntry,
			),
			container.NewBorder(
				nil,
				nil,
				ui.aiFallbackCheck,
				nil,
				widget.NewLabel("입력이나 옵션이 바뀌면 기존 미리보기를 다시 생성해야 합니다."),
			),
		),
	)

	statusSection := newSection(
		"상태",
		"현재 미리보기와 경고 상태",
		container.NewVBox(
			ui.statusLabel,
			ui.warningBanner,
			ui.errorBanner,
		),
	)

	detailSection := newSection(
		"선택 항목",
		"표에서 행을 선택하면 상세 정보를 확인할 수 있습니다.",
		detailForm,
	)

	operationsSection := newSection(
		"미리보기",
		"적용 예정 변경 목록",
		operationsBody,
	)

	skipSection := newSection(
		"건너뛴 항목",
		"건너뛴 파일과 이유",
		skipsBody,
	)

	sidePanel := container.NewBorder(
		container.NewVBox(statusSection, detailSection),
		nil,
		nil,
		nil,
		skipSection,
	)

	center := container.NewHSplit(operationsSection, sidePanel)
	center.Offset = 0.67

	summaryMetrics := container.NewGridWithColumns(
		4,
		newMetricPanel("상태", ui.summaryStatus),
		newMetricPanel("변경 예정", ui.summaryRenames),
		newMetricPanel("건너뜀", ui.summarySkips),
		newMetricPanel("검토 필요", ui.summaryReview),
	)

	summarySection := newSection(
		"실행 요약",
		"preview 결과를 확인한 뒤 적용하세요.",
		container.NewVBox(
			summaryMetrics,
			ui.summaryLabel,
			container.NewHBox(
				widget.NewLabel("대량 rename 도구이므로 요약과 선택 항목을 다시 확인한 뒤 적용하세요."),
				layout.NewSpacer(),
			),
			container.NewHBox(
				layout.NewSpacer(),
				ui.applyButton,
			),
		),
	)

	return container.NewBorder(
		container.NewVBox(controls),
		summarySection,
		nil,
		nil,
		container.NewPadded(center),
	)
}

func newMetricValueLabel() *widget.Label {
	label := widget.NewLabel("-")
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

func newMetricPanel(title string, value *widget.Label) fyne.CanvasObject {
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	titleLabel.Wrapping = fyne.TextWrapOff

	background := canvas.NewRectangle(withAlpha(theme.Color(theme.ColorNameInputBackground), 255))
	background.StrokeColor = withAlpha(theme.Color(theme.ColorNameSeparator), 180)
	background.StrokeWidth = 1
	background.CornerRadius = 10

	content := container.NewPadded(container.NewVBox(
		titleLabel,
		value,
	))

	return container.NewStack(background, content)
}

func newDetailValueLabel() *widget.Label {
	label := widget.NewLabel("-")
	label.Wrapping = fyne.TextWrapWord
	return label
}

func newSection(title, subtitle string, content fyne.CanvasObject) fyne.CanvasObject {
	titleLabel := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitleLabel := widget.NewLabel(subtitle)
	subtitleLabel.Wrapping = fyne.TextWrapWord

	header := container.NewVBox(
		titleLabel,
		subtitleLabel,
		widget.NewSeparator(),
	)

	return container.NewBorder(header, nil, nil, nil, content)
}

func (ui *application) refreshSummaryMetrics() {
	if ui.preview == nil {
		ui.summaryStatus.SetText("-")
		ui.summaryRenames.SetText("0")
		ui.summarySkips.SetText("0")
		ui.summaryReview.SetText("0")
		return
	}

	summary := ui.preview.Report.Summary
	ui.summaryStatus.SetText(buildSummaryStatusText(ui.preview.Report, len(ui.preview.Warnings)))
	ui.summaryRenames.SetText(fmt.Sprintf("%d", summary.PlannedRenames))
	ui.summarySkips.SetText(fmt.Sprintf("%d", summary.Skips))
	ui.summaryReview.SetText(fmt.Sprintf("%d", summary.UnresolvedSubtitles+summary.UnresolvedMovies))
}

func (ui *application) setPreview(preview re.PreviewResult) {
	ui.preview = &preview
	ui.operations = buildOperationRows(preview.Plan.Operations)
	ui.skips = buildSkipRows(preview.Plan.Skips)
	ui.summaryLabel.SetText(buildSummaryText(preview.Report))
	ui.refreshSummaryMetrics()
	ui.setWarningState(preview.Warnings)
	ui.setBannerText(ui.errorBanner, ui.errorLabel, "")
	ui.setLoading(false, "")
	ui.refreshStatusLabel()
	ui.refreshDataViews()
	ui.operationsTable.Refresh()
	ui.skipsTable.Refresh()
	if len(ui.operations) > 0 {
		ui.operationsTable.Select(widget.TableCellID{Row: 1, Col: 0})
	} else {
		ui.selectOperation(-1)
	}
	ui.updateActionState()
}

func (ui *application) renderIdleState() {
	ui.clearPreviewDisplay("폴더를 선택하고 preview를 생성하세요.")
	ui.refreshStatusLabel()
	ui.updateActionState()
}

func (ui *application) clearPreviewDisplay(summary string) {
	ui.preview = nil
	ui.operations = nil
	ui.skips = nil
	ui.selectedOpIndex = -1
	ui.summaryLabel.SetText(summary)
	ui.refreshSummaryMetrics()
	ui.setWarningState(nil)
	ui.selectOperation(-1)
	ui.refreshDataViews()
}

func (ui *application) refreshPreview() {
	targetPath, options, ok := ui.preparePreviewRefresh()
	if !ok {
		return
	}

	go func(path string, opts re.RunOptions) {
		preview, err := re.BuildPreview(context.Background(), re.PreviewRequest{
			TargetPath: path,
			Options:    opts,
		})

		fyne.Do(func() {
			if err != nil {
				ui.setLoading(false, "")
				ui.showError(err)
				return
			}
			ui.setPreview(preview)
		})
	}(targetPath, options)
}

func (ui *application) preparePreviewRefresh() (string, re.RunOptions, bool) {
	targetPath := strings.TrimSpace(ui.targetEntry.Text)
	if targetPath == "" {
		dialog.NewInformation("폴더 선택", "먼저 대상 폴더를 선택하세요.", ui.window).Show()
		return "", re.RunOptions{}, false
	}

	options := re.DefaultRunOptions()
	options.OutputFormat = re.OutputFormatText
	options.AI.Enabled = ui.aiFallbackCheck.Checked

	ui.clearPreviewDisplay("미리보기를 생성하는 중입니다...")
	ui.setWarningState(nil)
	ui.setBannerText(ui.errorBanner, ui.errorLabel, "")
	ui.setLoading(true, "미리보기를 생성하는 중입니다...")
	return targetPath, options, true
}

func (ui *application) confirmApply() {
	if ui.preview == nil || len(ui.preview.Plan.Operations) == 0 || ui.loading {
		return
	}
	if !ui.previewMatchesCurrentInput() {
		dialog.NewInformation("Preview 재생성 필요", "경로 또는 옵션이 바뀌었습니다. preview를 다시 생성하세요.", ui.window).Show()
		return
	}

	preview := *ui.preview
	message := fmt.Sprintf(
		"%d건의 rename을 적용합니다.\nskip %d건은 그대로 유지됩니다.\n계속할까요?",
		len(preview.Plan.Operations),
		len(preview.Plan.Skips),
	)
	if len(preview.Warnings) > 0 {
		message = fmt.Sprintf("%s\n\n경고 %d건이 있습니다.\n- %s", message, len(preview.Warnings), summarizeWarnings(preview.Warnings, 3))
	}
	dialog.NewConfirm("Rename 적용", message, func(ok bool) {
		if !ok {
			return
		}
		ui.applyPreview(preview)
	}, ui.window).Show()
}

func (ui *application) applyPreview(preview re.PreviewResult) {
	ui.setLoading(true, "변경을 적용하는 중입니다...")

	go func(preview re.PreviewResult) {
		report, err := re.ApplyPreview(preview)

		fyne.Do(func() {
			if err != nil {
				ui.setLoading(false, "")
				ui.handleApplyError(err)
				return
			}

			dialog.NewInformation(
				"적용 완료",
				fmt.Sprintf("%d건 변경을 적용했습니다.", report.Summary.PlannedRenames),
				ui.window,
			).Show()

			ui.setLoading(false, "")
			ui.refreshPreview()
		})
	}(preview)
}

func (ui *application) handleApplyError(err error) {
	if errors.Is(err, re.ErrPreviewExpired) {
		ui.clearPreviewDisplay("preview 이후 폴더 상태가 바뀌었습니다. preview를 다시 생성하세요.")
		ui.setBannerText(ui.errorBanner, ui.errorLabel, fmt.Sprintf("preview 이후 폴더 상태가 바뀌었습니다: %v", err))
		dialog.NewInformation("새로고침 필요", "폴더 상태가 바뀌어 적용을 중단했습니다. preview를 다시 생성하세요.", ui.window).Show()
		ui.refreshStatusLabel()
		ui.updateActionState()
		return
	}

	ui.showError(err)
}

func (ui *application) setLoading(loading bool, message string) {
	ui.loading = loading
	if loading {
		ui.statusLabel.SetText(message)
		ui.setBannerText(ui.errorBanner, ui.errorLabel, "")
	}
	ui.updateActionState()
}

func (ui *application) showError(err error) {
	ui.setBannerText(ui.errorBanner, ui.errorLabel, err.Error())
	dialog.NewError(err, ui.window).Show()
	ui.refreshStatusLabel()
	ui.updateActionState()
}

func (ui *application) updateActionState() {
	if ui.loading {
		ui.targetEntry.Disable()
		ui.aiFallbackCheck.Disable()
		ui.browseButton.Disable()
		ui.refreshButton.Disable()
		ui.applyButton.Disable()
		return
	}

	ui.targetEntry.Enable()
	ui.aiFallbackCheck.Enable()
	ui.browseButton.Enable()
	ui.refreshButton.Enable()
	if ui.preview == nil || len(ui.preview.Plan.Operations) == 0 || !ui.previewMatchesCurrentInput() {
		ui.applyButton.Disable()
		return
	}
	ui.applyButton.Enable()
}

func (ui *application) handleInputChange() {
	if ui.loading {
		return
	}
	ui.setBannerText(ui.errorBanner, ui.errorLabel, "")
	ui.refreshStatusLabel()
	ui.updateActionState()
}

func (ui *application) operationCellText(id widget.TableCellID) string {
	if id.Row == 0 {
		return operationHeaders[id.Col]
	}
	if len(ui.operations) == 0 {
		return ""
	}

	row := ui.operations[id.Row-1]
	switch id.Col {
	case operationColumnSource:
		return middleEllipsis(row.Source, 20, 22)
	case operationColumnDestination:
		return middleEllipsis(row.Destination, 22, 24)
	case operationColumnEpisode:
		return row.Episode
	case operationColumnExtension:
		return row.Extension
	case operationColumnMatch:
		return displayMatchSource(row.Match)
	case operationColumnConfidence:
		return row.Confidence
	default:
		return ""
	}
}

func (ui *application) skipCellText(id widget.TableCellID) string {
	if id.Row == 0 {
		return skipHeaders[id.Col]
	}
	if len(ui.skips) == 0 {
		return ""
	}

	row := ui.skips[id.Row-1]
	switch id.Col {
	case skipColumnSource:
		return middleEllipsis(row.Source, 20, 22)
	case skipColumnReason:
		return row.Reason
	default:
		return ""
	}
}

func (ui *application) selectOperation(index int) {
	ui.selectedOpIndex = index
	if index < 0 || index >= len(ui.operations) {
		ui.detailFolder.SetText("-")
		ui.detailSource.SetText("-")
		ui.detailTarget.SetText("-")
		ui.detailEpisode.SetText("-")
		ui.detailMatch.SetText("-")
		ui.detailConfidence.SetText("-")
		return
	}

	row := ui.operations[index]
	ui.detailFolder.SetText(filepath.Dir(row.SourcePath))
	ui.detailSource.SetText(row.Source)
	ui.detailTarget.SetText(row.Destination)
	ui.detailEpisode.SetText(row.Episode)
	ui.detailMatch.SetText(displayMatchSource(row.Match))
	ui.detailConfidence.SetText(row.Confidence)
}

func (ui *application) setBannerText(banner *fyne.Container, label *widget.Label, text string) {
	label.SetText(text)
	if strings.TrimSpace(text) == "" {
		banner.Hide()
		return
	}
	banner.Show()
}

func (ui *application) refreshStatusLabel() {
	switch {
	case ui.loading:
		return
	case strings.TrimSpace(ui.errorLabel.Text) != "":
		ui.statusLabel.SetText("오류를 확인하세요.")
	case ui.preview != nil && !ui.previewMatchesCurrentInput():
		ui.statusLabel.SetText("경로 또는 옵션이 바뀌었습니다. preview를 다시 생성하세요.")
	case ui.preview != nil:
		ui.statusLabel.SetText(buildStatusText(ui.preview.Report, len(ui.preview.Warnings)))
	case strings.TrimSpace(ui.targetEntry.Text) == "":
		ui.statusLabel.SetText("준비됨")
	default:
		ui.statusLabel.SetText("미리보기를 생성하세요.")
	}
}

func (ui *application) previewMatchesCurrentInput() bool {
	if ui.preview == nil {
		return false
	}

	return normalizeTargetInput(ui.targetEntry.Text) == ui.preview.TargetPath &&
		ui.aiFallbackCheck.Checked == ui.preview.Options.AI.Enabled
}

func buildOperationRows(operations []re.RenameOperation) []operationRow {
	rows := make([]operationRow, 0, len(operations))
	for _, operation := range operations {
		confidence := "-"
		if operation.MatchSource == "ai" {
			confidence = fmt.Sprintf("%.2f", operation.Confidence)
		}

		rows = append(rows, operationRow{
			Source:          filepath.Base(operation.SourcePath),
			SourcePath:      operation.SourcePath,
			Destination:     operation.DestinationName,
			DestinationPath: operation.DestinationPath,
			Episode:         displayOrDash(operation.Episode),
			Extension:       strings.TrimPrefix(strings.ToLower(filepath.Ext(operation.SourcePath)), "."),
			Match:           operation.MatchSource,
			Confidence:      confidence,
			ConfidenceValue: operation.Confidence,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left := operationSortKey(rows[i])
		right := operationSortKey(rows[j])
		if left != right {
			return left < right
		}
		if rows[i].Match == "ai" && rows[j].Match == "ai" && rows[i].ConfidenceValue != rows[j].ConfidenceValue {
			return rows[i].ConfidenceValue < rows[j].ConfidenceValue
		}
		if rows[i].Episode != rows[j].Episode {
			return rows[i].Episode < rows[j].Episode
		}
		return strings.ToLower(rows[i].Source) < strings.ToLower(rows[j].Source)
	})
	return rows
}

func buildSkipRows(skips []re.SkipOperation) []skipRow {
	rows := make([]skipRow, 0, len(skips))
	for _, skip := range skips {
		rows = append(rows, skipRow{
			Source:     filepath.Base(skip.SourcePath),
			SourcePath: skip.SourcePath,
			Reason:     skip.Reason,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Source) < strings.ToLower(rows[j].Source)
	})
	return rows
}

func buildSummaryText(report re.RunReport) string {
	return fmt.Sprintf(
		"대상 폴더: %s\n규칙 기반 변경 %d건, AI 기반 변경 %d건\n미해결 영상 %d건, 미해결 자막 %d건",
		report.TargetPath,
		report.Summary.RuleRenames,
		report.Summary.AIRenames,
		report.Summary.UnresolvedMovies,
		report.Summary.UnresolvedSubtitles,
	)
}

func buildStatusText(report re.RunReport, warningCount int) string {
	if warningCount > 0 {
		return fmt.Sprintf("경고 %d건이 있습니다. 내용을 확인하세요.", warningCount)
	}

	switch {
	case report.Applied:
		return "적용 완료"
	case report.Status == "needs_review":
		return "검토가 필요한 항목이 있습니다."
	case report.Status == "noop":
		return "적용할 rename이 없습니다."
	default:
		return "미리보기가 준비되었습니다."
	}
}

func displayOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func newMessageBanner(iconResource fyne.Resource, accent color.Color, body fyne.CanvasObject) *fyne.Container {
	background := canvas.NewRectangle(withAlpha(accent, 24))
	background.StrokeColor = withAlpha(accent, 96)
	background.StrokeWidth = 1
	background.CornerRadius = 8

	icon := widget.NewIcon(iconResource)
	content := container.NewPadded(container.NewBorder(nil, nil, icon, nil, body))
	banner := container.NewStack(background, content)
	banner.Hide()
	return banner
}

func newEmptyState(iconResource fyne.Resource, text string) *fyne.Container {
	icon := widget.NewIcon(iconResource)
	label := widget.NewLabel(text)
	label.Alignment = fyne.TextAlignCenter
	label.Wrapping = fyne.TextWrapOff

	return container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(icon),
		container.NewCenter(label),
		layout.NewSpacer(),
	)
}

func withAlpha(source color.Color, alpha uint8) color.Color {
	r, g, b, _ := source.RGBA()
	return color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: alpha,
	}
}

func middleEllipsis(value string, head, tail int) string {
	runes := []rune(value)
	if len(runes) <= head+tail+3 {
		return value
	}
	if head < 0 {
		head = 0
	}
	if tail < 0 {
		tail = 0
	}
	return string(runes[:head]) + "..." + string(runes[len(runes)-tail:])
}

func operationSortKey(row operationRow) int {
	switch row.Match {
	case "ai":
		return 0
	case "rule":
		return 1
	default:
		return 2
	}
}

func (ui *application) setWarningState(warnings []string) {
	if len(warnings) == 0 {
		ui.warningButton.Hide()
		ui.setBannerText(ui.warningBanner, ui.warningLabel, "")
		return
	}

	ui.warningButton.Show()
	ui.setBannerText(ui.warningBanner, ui.warningLabel, fmt.Sprintf("경고 %d건이 있습니다. 적용 전에 확인하세요.", len(warnings)))
}

func (ui *application) showWarningsDialog() {
	if ui.preview == nil || len(ui.preview.Warnings) == 0 {
		return
	}

	dialog.NewInformation("경고 목록", strings.Join(ui.preview.Warnings, "\n\n"), ui.window).Show()
}

func (ui *application) refreshDataViews() {
	if len(ui.operations) == 0 {
		ui.operationsTable.Hide()
		ui.operationsEmpty.Show()
	} else {
		ui.operationsEmpty.Hide()
		ui.operationsTable.Show()
	}

	if len(ui.skips) == 0 {
		ui.skipsTable.Hide()
		ui.skipsEmpty.Show()
	} else {
		ui.skipsEmpty.Hide()
		ui.skipsTable.Show()
	}
}

func buildSummaryStatusText(report re.RunReport, warningCount int) string {
	if warningCount > 0 {
		return fmt.Sprintf("경고 %d건", warningCount)
	}
	if report.Applied {
		return "적용 완료"
	}
	switch report.Status {
	case "needs_review":
		return "검토 필요"
	case "noop":
		return "변경 없음"
	default:
		return "미리보기"
	}
}

func summarizeWarnings(warnings []string, limit int) string {
	if len(warnings) == 0 {
		return ""
	}
	if limit <= 0 || len(warnings) <= limit {
		return strings.Join(warnings, "\n- ")
	}
	return fmt.Sprintf("%s\n- 외 %d건", strings.Join(warnings[:limit], "\n- "), len(warnings)-limit)
}

func displayMatchSource(source string) string {
	switch source {
	case "ai":
		return "AI"
	case "rule":
		return "규칙"
	default:
		return displayOrDash(source)
	}
}

func normalizeTargetInput(targetPath string) string {
	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return "."
	}
	return filepath.Clean(trimmed)
}
