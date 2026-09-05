package main

import (
	"sort"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
)

const (
	maxVisibleCompletions = 6
	maxCompletionLabelLen = 72
)

// completionPopup renderiza sugestões dentro do editor, sem overlay de canvas (não rouba foco).
type completionPopup struct {
	layer       *fyne.Container
	background  *canvas.Rectangle
	rows        *fyne.Container
	scroll      *container.Scroll
	anchor      *codeEditor
	suggestions []completionSuggestion
	selected    int
	onAccept    func(completionSuggestion)
	onDismiss   func()
	lineHeight  float32
}

func newCompletionPopup(anchor *codeEditor, layer *fyne.Container, onAccept func(completionSuggestion), onDismiss func()) *completionPopup {
	th := fyne.CurrentApp().Settings().Theme()
	bg := canvas.NewRectangle(th.Color(theme.ColorNameOverlayBackground, theme.VariantDark))
	bg.CornerRadius = th.Size(theme.SizeNamePopupRadius)

	cp := &completionPopup{
		anchor:     anchor,
		layer:      layer,
		background: bg,
		rows:       container.NewVBox(),
		onAccept:   onAccept,
		onDismiss:  onDismiss,
		selected:   0,
	}
	cp.scroll = container.NewScroll(cp.rows)
	cp.scroll.Direction = container.ScrollVerticalOnly
	layer.Objects = []fyne.CanvasObject{bg, cp.scroll}
	return cp
}

func (cp *completionPopup) compactLineHeight() float32 {
	if cp.lineHeight > 0 {
		return cp.lineHeight
	}
	th := fyne.CurrentApp().Settings().Theme()
	cp.lineHeight = th.Size(theme.SizeNameText) + 2
	return cp.lineHeight
}

func displayLabel(item completionSuggestion) string {
	label := item.Label
	if item.Detail != "" {
		label = item.Label + "  " + item.Detail
	}
	if len(label) > maxCompletionLabelLen {
		return label[:maxCompletionLabelLen-3] + "..."
	}
	return label
}

func (cp *completionPopup) SetSuggestions(items []completionSuggestion) {
	if len(items) > 50 {
		items = items[:50]
	}
	cp.suggestions = items
	if cp.selected >= len(items) {
		cp.selected = 0
	}
	cp.rebuild()
}

func (cp *completionPopup) rebuild() {
	th := fyne.CurrentApp().Settings().Theme()
	v := theme.VariantDark
	fg := th.Color(theme.ColorNameForeground, v)
	hl := th.Color(theme.ColorNameSelection, v)
	lineH := cp.compactLineHeight()

	rows := make([]fyne.CanvasObject, 0, len(cp.suggestions))
	panelW := cp.panelSize().Width - 2*theme.Padding()
	for i, item := range cp.suggestions {
		txt := canvas.NewText(displayLabel(item), fg)
		txt.TextSize = th.Size(theme.SizeNameText)
		txt.TextStyle = fyne.TextStyle{Monospace: true, Bold: i == cp.selected}
		row := container.NewWithoutLayout()
		row.Resize(fyne.NewSize(panelW, lineH))
		txt.Resize(fyne.NewSize(panelW, lineH))
		txt.Move(fyne.NewPos(theme.Padding(), 0))
		if i == cp.selected {
			bg := canvas.NewRectangle(hl)
			bg.Resize(fyne.NewSize(panelW, lineH))
			row.Objects = []fyne.CanvasObject{bg, txt}
		} else {
			row.Objects = []fyne.CanvasObject{txt}
		}
		rows = append(rows, row)
	}
	cp.rows.Objects = rows
	cp.rows.Refresh()
	cp.scrollToSelected()
}

func (cp *completionPopup) scrollToSelected() {
	if cp.scroll == nil || cp.selected < 0 {
		return
	}
	lineH := cp.compactLineHeight()
	maxOffset := float32(len(cp.suggestions)-maxVisibleCompletions) * lineH
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := float32(cp.selected) * lineH
	if offset > maxOffset {
		offset = maxOffset
	}
	cp.scroll.Offset = fyne.NewPos(0, offset)
	cp.scroll.Refresh()
}

func (cp *completionPopup) layoutAt(pos fyne.Position, size fyne.Size) {
	cp.background.Resize(size)
	cp.background.Move(fyne.NewPos(0, 0))
	cp.scroll.Resize(size)
	cp.scroll.Move(fyne.NewPos(0, 0))
	cp.layer.Resize(size)
	cp.layer.Move(pos)
}

func (cp *completionPopup) Show() {
	if len(cp.suggestions) == 0 {
		cp.Hide()
		return
	}

	pos := cp.anchor.completionPopupRelPos()
	size := cp.panelSize()
	cp.layer.Show()
	cp.layoutAt(pos, size)
	cp.scrollToSelected()
	cp.layer.Refresh()
	cp.anchor.Refresh()
}

func (cp *completionPopup) Hide() {
	if cp.layer != nil {
		cp.layer.Hide()
	}
	cp.selected = 0
	if cp.onDismiss != nil {
		cp.onDismiss()
	}
}

func (cp *completionPopup) Visible() bool {
	return cp.layer != nil && cp.layer.Visible()
}

func (cp *completionPopup) Selected() *completionSuggestion {
	if cp.selected < 0 || cp.selected >= len(cp.suggestions) {
		return nil
	}
	return &cp.suggestions[cp.selected]
}

func (cp *completionPopup) AcceptSelected() {
	if item := cp.Selected(); item != nil {
		cp.layer.Hide()
		if cp.onAccept != nil {
			cp.onAccept(*item)
		}
	}
}

func (cp *completionPopup) SelectNext() {
	if len(cp.suggestions) == 0 {
		return
	}
	cp.selected++
	if cp.selected >= len(cp.suggestions) {
		cp.selected = 0
	}
	cp.rebuild()
}

func (cp *completionPopup) SelectPrev() {
	if len(cp.suggestions) == 0 {
		return
	}
	cp.selected--
	if cp.selected < 0 {
		cp.selected = len(cp.suggestions) - 1
	}
	cp.rebuild()
}

func (cp *completionPopup) panelSize() fyne.Size {
	lineH := cp.compactLineHeight()
	visible := len(cp.suggestions)
	if visible > maxVisibleCompletions {
		visible = maxVisibleCompletions
	}
	height := float32(visible)*lineH + 2*theme.Padding()

	width := float32(400)
	editorSize := cp.anchor.Size()
	if editorSize.Width == 0 {
		editorSize = cp.anchor.MinSize()
	}
	if width > editorSize.Width-2*theme.Padding() {
		width = editorSize.Width - 2*theme.Padding()
	}
	if width < 160 {
		width = 160
	}
	return fyne.NewSize(width, height)
}

func identifierPrefixAt(line string, col int) string {
	if col > len(line) {
		col = len(line)
	}
	start := col
	for start > 0 && isIdentRune(line[start-1]) {
		start--
	}
	return line[start:col]
}

func isIdentRune(b byte) bool {
	return b == '_' || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b))
}

func filterCompletions(items []completionSuggestion, prefix string) []completionSuggestion {
	if len(items) == 0 {
		return nil
	}
	if prefix == "" {
		return items
	}

	out := make([]completionSuggestion, 0, len(items))
	for _, item := range items {
		if strings.HasPrefix(item.Label, prefix) {
			out = append(out, item)
		}
	}
	if len(out) == 0 {
		return nil
	}

	sort.SliceStable(out, func(i, j int) bool {
		li, lj := out[i].Label, out[j].Label
		if len(li) != len(lj) {
			return len(li) < len(lj)
		}
		return li < lj
	})
	return out
}

func shouldTriggerCompletion(text string, row, col int, typed string) bool {
	if typed == "." {
		return true
	}
	if len(typed) != 1 {
		return false
	}
	r := typed[0]
	if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
		return false
	}
	lines := strings.Split(text, "\n")
	if row < 0 || row >= len(lines) {
		return false
	}
	line := lines[row]
	if col > len(line) {
		col = len(line)
	}
	return strings.TrimSpace(line[:col]) != ""
}

func shouldKeepCompletion(text string, row, col int) bool {
	lines := strings.Split(text, "\n")
	if row < 0 || row >= len(lines) {
		return false
	}
	line := lines[row]
	if col > len(line) {
		col = len(line)
	}
	prefix := identifierPrefixAt(line, col)
	return prefix != "" || (col > 0 && line[col-1] == '.')
}
