package main

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const caretWidth = 2
const maxUndoStack = 100

type editorSnapshot struct {
	text      string
	cursorRow int
	cursorCol int
}

type codeEditor struct {
	widget.BaseWidget

	grid           *widget.TextGrid
	scroll         *container.Scroll
	text           string
	cursorRow      int
	cursorCol      int
	cursorVisible  bool
	blinkStop      chan struct{}
	caret          *canvas.Rectangle
	placeholder    string
	hasFocus       bool
	ctrlDown       bool
	pendingRefresh bool
	completion     *completionPopup
	completionOpen bool
	completionLayer *fyne.Container
	rawCompletions []completionSuggestion
	completionTimer *time.Timer
	undoStack       []editorSnapshot
	onAppShortcut   func(fyne.Shortcut)
	OnChanged       func(string)
	OnCompletion    func(row, col int)
}

func newCodeEditor() *codeEditor {
	ed := &codeEditor{
		grid: widget.NewTextGrid(),
	}
	ed.grid.ShowLineNumbers = true
	ed.grid.Scroll = fyne.ScrollNone
	ed.ExtendBaseWidget(ed)
	return ed
}

func (ed *codeEditor) CreateRenderer() fyne.WidgetRenderer {
	ed.scroll = container.NewScroll(ed.grid)
	ed.scroll.OnScrolled = func(_ fyne.Position) {
		ed.updateCaret()
	}
	ed.completionLayer = container.NewWithoutLayout()
	ed.completionLayer.Hide()
	th := fyne.CurrentApp().Settings().Theme()
	ed.caret = canvas.NewRectangle(th.Color(theme.ColorNameForeground, theme.VariantDark))
	ed.caret.Hidden = true
	if ed.pendingRefresh || ed.placeholder != "" || ed.text != "" {
		ed.pendingRefresh = false
		ed.doRefreshGrid(ed.MinSize())
	}
	return &codeEditorRenderer{
		editor:  ed,
		scroll:  ed.scroll,
		caret:   ed.caret,
		overlay: ed.completionLayer,
	}
}

func (ed *codeEditor) MinSize() fyne.Size {
	return fyne.NewSize(200, 120)
}

func (ed *codeEditor) FocusGained() {
	ed.hasFocus = true
	ed.cursorVisible = true
	ed.startCursorBlink()
	ed.refreshGrid()
}

func (ed *codeEditor) FocusLost() {
	ed.hasFocus = false
	ed.stopCursorBlink()
	ed.refreshGrid()
}

func (ed *codeEditor) KeyDown(key *fyne.KeyEvent) {
	switch key.Name {
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		ed.ctrlDown = true
	}
}

func (ed *codeEditor) KeyUp(key *fyne.KeyEvent) {
	switch key.Name {
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		ed.ctrlDown = false
	}
}

func (ed *codeEditor) TypedRune(r rune) {
	s := string(r)
	ed.insertString(s)
	if ed.completionOpen {
		ed.refreshCompletionDisplay()
	}
	ed.scheduleCompletion(s)
}

func (ed *codeEditor) TypedKey(ev *fyne.KeyEvent) {
	if ed.ctrlDown {
		switch ev.Name {
		case fyne.KeyV:
			ed.paste()
			return
		case fyne.KeyC:
			ed.copy()
			return
		case fyne.KeyX:
			ed.cut()
			return
		case fyne.KeyA:
			ed.selectAll()
			return
		case fyne.KeySpace:
			ed.requestCompletion()
			return
		}
	}

	switch ev.Name {
	case fyne.KeyBackspace:
		ed.deleteBefore()
		ed.afterEditCompletion()
	case fyne.KeyDelete:
		ed.deleteAfter()
		ed.afterEditCompletion()
	case fyne.KeyReturn, fyne.KeyEnter:
		if ed.acceptCompletion() {
			return
		}
		ed.insertString("\n")
	case fyne.KeyTab:
		if ed.acceptCompletion() {
			return
		}
		ed.insertString("\t")
	case fyne.KeyEscape:
		if ed.completionOpen {
			ed.hideCompletion()
			return
		}
	case fyne.KeyLeft, fyne.KeyRight:
		ed.hideCompletion()
		switch ev.Name {
		case fyne.KeyLeft:
			ed.moveLeft()
		case fyne.KeyRight:
			ed.moveRight()
		}
	case fyne.KeyUp:
		if ed.completionOpen && ed.completion != nil {
			ed.completion.SelectPrev()
			ed.completion.Show()
			return
		}
		ed.moveUp()
	case fyne.KeyDown:
		if ed.completionOpen && ed.completion != nil {
			ed.completion.SelectNext()
			ed.completion.Show()
			return
		}
		ed.moveDown()
	case fyne.KeyHome:
		ed.hideCompletion()
		ed.cursorCol = 0
		ed.refreshGrid()
	case fyne.KeyEnd:
		ed.hideCompletion()
		ed.cursorCol = ed.lineLen(ed.cursorRow)
		ed.refreshGrid()
	}
}

func (ed *codeEditor) Text() string {
	return ed.text
}

func (ed *codeEditor) SetText(text string) {
	ed.undoStack = nil
	ed.text = text
	ed.cursorRow = 0
	ed.cursorCol = 0
	ed.rawCompletions = nil
	ed.hideCompletion()
	ed.refreshGrid()
}

func (ed *codeEditor) CursorRow() int {
	return ed.cursorRow
}

func (ed *codeEditor) CursorCol() int {
	return ed.cursorCol
}

func (ed *codeEditor) initCompletion() {
	if ed.completion != nil {
		return
	}
	ed.completion = newCompletionPopup(
		ed,
		ed.completionLayer,
		ed.applyCompletion,
		func() { ed.completionOpen = false },
	)
}

func (ed *codeEditor) lineNumberWidth() int {
	if ed.grid == nil || !ed.grid.ShowLineNumbers {
		return 0
	}
	lineCount := len(ed.grid.Rows)
	if lineCount == 0 {
		lineCount = 1
	}
	return len(fmt.Sprintf("%d", lineCount))
}

func (ed *codeEditor) gutterCols() int {
	if ed.lineNumberWidth() == 0 {
		return 0
	}
	return ed.lineNumberWidth() + 1
}

func (ed *codeEditor) gridColForCursor() int {
	return ed.gutterCols() + ed.cursorDisplayCol()
}

func (ed *codeEditor) gridOriginInEditor() fyne.Position {
	driver := fyne.CurrentApp().Driver()
	return driver.AbsolutePositionForObject(ed.grid).Subtract(driver.AbsolutePositionForObject(ed))
}

func (ed *codeEditor) gridPointFromEditorPoint(pos fyne.Position) fyne.Position {
	return pos.Subtract(ed.gridOriginInEditor())
}

func (ed *codeEditor) cursorDisplayCol() int {
	lines := ed.lines()
	if ed.cursorRow < 0 || ed.cursorRow >= len(lines) {
		return 0
	}
	line := lines[ed.cursorRow]
	col := 0
	for i := 0; i < ed.cursorCol && i < len(line); {
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
		col++
	}
	return col
}

func (ed *codeEditor) caretPosition() fyne.Position {
	if ed.grid == nil {
		return fyne.NewPos(0, 0)
	}
	cellPos := ed.grid.PositionForCursorLocation(ed.cursorRow, ed.gridColForCursor())
	return ed.gridOriginInEditor().Add(cellPos)
}

func (ed *codeEditor) completionPopupRelPos() fyne.Position {
	if ed.grid == nil {
		return fyne.NewPos(0, 0)
	}

	cellPos := ed.grid.PositionForCursorLocation(ed.cursorRow, ed.gridColForCursor())
	below := ed.grid.PositionForCursorLocation(ed.cursorRow+1, ed.gridColForCursor())
	return ed.gridOriginInEditor().Add(fyne.NewPos(cellPos.X, below.Y))
}

func (ed *codeEditor) setCursorFromPoint(pos fyne.Position) {
	if ed.grid == nil {
		return
	}

	gridPos := ed.gridPointFromEditorPoint(pos)
	row, col := ed.grid.CursorLocationForPosition(gridPos)
	textCol := col - ed.gutterCols()
	if textCol < 0 {
		textCol = 0
	}

	lines := ed.lines()
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}

	line := lines[row]
	byteCol := 0
	for i := 0; i < textCol && byteCol < len(line); i++ {
		_, size := utf8.DecodeRuneInString(line[byteCol:])
		byteCol += size
	}
	if textCol > utf8.RuneCountInString(line) {
		byteCol = len(line)
	}

	ed.cursorRow = row
	ed.cursorCol = byteCol
	ed.clampCursor()
	ed.cursorVisible = true
	ed.hideCompletion()
	ed.refreshGrid()
}

func (ed *codeEditor) startCursorBlink() {
	ed.stopCursorBlink()
	ed.blinkStop = make(chan struct{})
	stop := ed.blinkStop
	go func() {
		ticker := time.NewTicker(530 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fyne.Do(func() {
					if ed.blinkStop != stop {
						return
					}
					ed.cursorVisible = !ed.cursorVisible
					ed.updateCaret()
				})
			case <-stop:
				return
			}
		}
	}()
}

func (ed *codeEditor) stopCursorBlink() {
	if ed.blinkStop != nil {
		close(ed.blinkStop)
		ed.blinkStop = nil
	}
}

func (ed *codeEditor) updateCaret() {
	if ed.caret == nil || ed.grid == nil {
		return
	}
	top := ed.grid.PositionForCursorLocation(ed.cursorRow, ed.gridColForCursor())
	bottom := ed.grid.PositionForCursorLocation(ed.cursorRow+1, ed.gridColForCursor())
	ed.caret.Move(ed.caretPosition())
	ed.caret.Resize(fyne.NewSize(caretWidth, bottom.Y-top.Y))
	ed.caret.Hidden = !ed.hasFocus || !ed.cursorVisible
	ed.caret.Refresh()
}

func (ed *codeEditor) acceptCompletion() bool {
	if !ed.completionOpen || ed.completion == nil || ed.completion.Selected() == nil {
		return false
	}
	ed.completion.AcceptSelected()
	return true
}

func (ed *codeEditor) requestCompletion() {
	if ed.OnCompletion != nil {
		ed.OnCompletion(ed.cursorRow, ed.cursorCol)
	}
}

func (ed *codeEditor) scheduleCompletion(typed string) {
	trigger := ed.completionOpen || shouldTriggerCompletion(ed.text, ed.cursorRow, ed.cursorCol, typed)
	if !trigger {
		return
	}
	if ed.completionTimer != nil {
		ed.completionTimer.Stop()
	}
	delay := 200 * time.Millisecond
	if ed.completionOpen {
		delay = 120 * time.Millisecond
	}
	ed.completionTimer = time.AfterFunc(delay, func() {
		fyne.Do(func() {
			ed.requestCompletion()
		})
	})
}

func (ed *codeEditor) afterEditCompletion() {
	if ed.completionOpen {
		ed.refreshCompletionDisplay()
	}
	ed.scheduleCompletion("")
}

func (ed *codeEditor) identifierPrefixAtCursor() string {
	lines := ed.lines()
	if ed.cursorRow < 0 || ed.cursorRow >= len(lines) {
		return ""
	}
	return identifierPrefixAt(lines[ed.cursorRow], ed.cursorCol)
}

func (ed *codeEditor) refreshCompletionDisplay() {
	if !shouldKeepCompletion(ed.text, ed.cursorRow, ed.cursorCol) {
		ed.hideCompletion()
		return
	}
	filtered := filterCompletions(ed.rawCompletions, ed.identifierPrefixAtCursor())
	if len(filtered) == 0 {
		ed.hideCompletion()
		return
	}
	ed.initCompletion()
	ed.completion.SetSuggestions(filtered)
	ed.completionOpen = true
	ed.completion.Show()
}

func (ed *codeEditor) ShowCompletions(items []completionSuggestion) {
	ed.rawCompletions = items
	ed.refreshCompletionDisplay()
}

func (ed *codeEditor) hideCompletion() {
	if ed.completion != nil {
		ed.completion.Hide()
	}
	ed.completionOpen = false
}

func (ed *codeEditor) applyCompletion(item completionSuggestion) {
	if ed.completion != nil {
		ed.completion.Hide()
	}
	ed.completionOpen = false
	ed.rawCompletions = nil
	from := item.ReplaceFrom
	to := item.ReplaceTo
	if from < 0 {
		from = 0
	}
	if to > len(ed.text) {
		to = len(ed.text)
	}
	if from > to {
		from, to = to, from
	}
	ed.pushUndo()
	ed.text = ed.text[:from] + item.InsertText + ed.text[to:]
	ed.setCursorFromOffset(from + len(item.InsertText))
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) SetPlaceHolder(placeholder string) {
	ed.placeholder = placeholder
	ed.pendingRefresh = true
}

func (ed *codeEditor) lines() []string {
	if ed.text == "" {
		return []string{""}
	}
	return strings.Split(ed.text, "\n")
}

func (ed *codeEditor) lineLen(row int) int {
	lines := ed.lines()
	if row < 0 || row >= len(lines) {
		return 0
	}
	return len(lines[row])
}

func (ed *codeEditor) clampCursor() {
	lines := ed.lines()
	if ed.cursorRow < 0 {
		ed.cursorRow = 0
	}
	if ed.cursorRow >= len(lines) {
		ed.cursorRow = len(lines) - 1
	}
	if ed.cursorCol < 0 {
		ed.cursorCol = 0
	}
	if ed.cursorCol > len(lines[ed.cursorRow]) {
		ed.cursorCol = len(lines[ed.cursorRow])
	}
}

func (ed *codeEditor) cursorByteOffset() int {
	lines := ed.lines()
	offset := 0
	for i := 0; i < ed.cursorRow; i++ {
		offset += len(lines[i]) + 1
	}
	return offset + ed.cursorCol
}

func (ed *codeEditor) setCursorFromOffset(offset int) {
	if offset < 0 {
		offset = 0
	}
	if offset > len(ed.text) {
		offset = len(ed.text)
	}
	row := 0
	remaining := offset
	for i, line := range ed.lines() {
		lineLen := len(line)
		if remaining <= lineLen {
			ed.cursorRow = i
			ed.cursorCol = remaining
			return
		}
		remaining -= lineLen + 1
		row++
	}
	ed.cursorRow = row
	ed.cursorCol = ed.lineLen(ed.cursorRow)
}

func (ed *codeEditor) notifyChanged() {
	if ed.OnChanged != nil {
		ed.OnChanged(ed.text)
	}
}

func (ed *codeEditor) pushUndo() {
	if len(ed.undoStack) >= maxUndoStack {
		ed.undoStack = ed.undoStack[1:]
	}
	ed.undoStack = append(ed.undoStack, editorSnapshot{
		text:      ed.text,
		cursorRow: ed.cursorRow,
		cursorCol: ed.cursorCol,
	})
}

func (ed *codeEditor) undo() {
	if len(ed.undoStack) == 0 {
		return
	}
	snap := ed.undoStack[len(ed.undoStack)-1]
	ed.undoStack = ed.undoStack[:len(ed.undoStack)-1]
	ed.text = snap.text
	ed.cursorRow = snap.cursorRow
	ed.cursorCol = snap.cursorCol
	ed.hideCompletion()
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) insertString(s string) {
	if s == "" {
		return
	}
	ed.pushUndo()
	offset := ed.cursorByteOffset()
	ed.text = ed.text[:offset] + s + ed.text[offset:]
	ed.setCursorFromOffset(offset + len(s))
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) deleteBefore() {
	offset := ed.cursorByteOffset()
	if offset == 0 {
		return
	}
	ed.pushUndo()
	_, size := utf8.DecodeLastRuneInString(ed.text[:offset])
	ed.text = ed.text[:offset-size] + ed.text[offset:]
	ed.setCursorFromOffset(offset - size)
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) deleteAfter() {
	offset := ed.cursorByteOffset()
	if offset >= len(ed.text) {
		return
	}
	ed.pushUndo()
	_, size := utf8.DecodeRuneInString(ed.text[offset:])
	ed.text = ed.text[:offset] + ed.text[offset+size:]
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) moveLeft() {
	if ed.cursorCol > 0 {
		ed.cursorCol--
	} else if ed.cursorRow > 0 {
		ed.cursorRow--
		ed.cursorCol = ed.lineLen(ed.cursorRow)
	}
	ed.refreshGrid()
}

func (ed *codeEditor) moveRight() {
	if ed.cursorCol < ed.lineLen(ed.cursorRow) {
		ed.cursorCol++
	} else if ed.cursorRow < len(ed.lines())-1 {
		ed.cursorRow++
		ed.cursorCol = 0
	}
	ed.refreshGrid()
}

func (ed *codeEditor) moveUp() {
	if ed.cursorRow > 0 {
		ed.cursorRow--
		ed.clampCursor()
		ed.refreshGrid()
	}
}

func (ed *codeEditor) moveDown() {
	if ed.cursorRow < len(ed.lines())-1 {
		ed.cursorRow++
		ed.clampCursor()
		ed.refreshGrid()
	}
}

func (ed *codeEditor) paste() {
	content := fyne.CurrentApp().Clipboard().Content()
	if content == "" {
		return
	}
	ed.pushUndo()
	offset := ed.cursorByteOffset()
	ed.text = ed.text[:offset] + content + ed.text[offset:]
	ed.setCursorFromOffset(offset + len(content))
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) copy() {
	start, end := ed.selectionOffsets()
	if start == end {
		return
	}
	fyne.CurrentApp().Clipboard().SetContent(ed.text[start:end])
}

func (ed *codeEditor) cut() {
	start, end := ed.selectionOffsets()
	if start == end {
		return
	}
	ed.pushUndo()
	fyne.CurrentApp().Clipboard().SetContent(ed.text[start:end])
	ed.text = ed.text[:start] + ed.text[end:]
	ed.setCursorFromOffset(start)
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) selectAll() {
	ed.cursorRow = len(ed.lines()) - 1
	ed.cursorCol = ed.lineLen(ed.cursorRow)
	ed.refreshGrid()
}

func (ed *codeEditor) selectionOffsets() (int, int) {
	return 0, len(ed.text)
}

func (ed *codeEditor) refreshGrid() {
	if ed.hasFocus {
		ed.cursorVisible = true
	}
	if ed.scroll == nil {
		ed.pendingRefresh = true
		ed.Refresh()
		return
	}
	size := ed.scroll.Size()
	if size.Width == 0 || size.Height == 0 {
		size = ed.MinSize()
	}
	ed.doRefreshGrid(size)
	if ed.completionOpen && ed.completion != nil {
		ed.completion.Show()
	}
}

func (ed *codeEditor) doRefreshGrid(size fyne.Size) {
	display := ed.text
	isPlaceholder := false
	if display == "" && ed.placeholder != "" && !ed.hasFocus {
		display = ed.placeholder
		isPlaceholder = true
	}

	th := fyne.CurrentApp().Settings().Theme()
	fg := th.Color(theme.ColorNameForeground, theme.VariantDark)
	placeholderFG := th.Color(theme.ColorNamePlaceHolder, theme.VariantDark)

	bracketIdx := bracketColors(display)
	syntaxIdx := goSyntaxHighlight(display)
	lines := strings.Split(display, "\n")
	if len(lines) == 0 {
		lines = []string{""}
	}

	rows := make([]widget.TextGridRow, len(lines))
	bytePos := 0
	for row, line := range lines {
		cells := make([]widget.TextGridCell, 0, len(line)+1)
		for col := 0; col < len(line); {
			r, size := utf8.DecodeRuneInString(line[col:])
			style := &widget.CustomTextGridStyle{FGColor: fg}
			if isPlaceholder {
				style.FGColor = placeholderFG
			} else if c := bracketColorAt(display, bracketIdx, bytePos); c != nil {
				style.FGColor = c
			} else if c := syntaxColorAt(display, syntaxIdx, bytePos); c != nil {
				style.FGColor = c
			}
			cells = append(cells, widget.TextGridCell{Rune: r, Style: style})
			col += size
			bytePos += size
		}
		rows[row] = widget.TextGridRow{Cells: cells}
		bytePos++ // newline
	}

	ed.grid.Rows = rows

	width := size.Width
	if width < 200 {
		width = 200
	}
	ed.grid.Resize(fyne.NewSize(width, size.Height))
	ed.grid.Refresh()
	ed.updateCaret()
}

var _ desktop.Keyable = (*codeEditor)(nil)
var _ fyne.Shortcutable = (*codeEditor)(nil)

func (ed *codeEditor) TypedShortcut(shortcut fyne.Shortcut) {
	switch s := shortcut.(type) {
	case *fyne.ShortcutUndo:
		ed.undo()
	case *fyne.ShortcutPaste:
		content := s.Clipboard.Content()
		if content == "" {
			return
		}
		ed.pushUndo()
		offset := ed.cursorByteOffset()
		ed.text = ed.text[:offset] + content + ed.text[offset:]
		ed.setCursorFromOffset(offset + len(content))
		ed.refreshGrid()
		ed.notifyChanged()
	case *fyne.ShortcutCopy:
		ed.copy()
	case *fyne.ShortcutCut:
		ed.cut()
	case *fyne.ShortcutSelectAll:
		ed.selectAll()
	default:
		if ed.onAppShortcut != nil {
			ed.onAppShortcut(shortcut)
		}
	}
}

func (ed *codeEditor) Tapped(ev *fyne.PointEvent) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(ed); c != nil {
		c.Focus(ed)
	}
	ed.setCursorFromPoint(ev.Position)
}

type codeEditorRenderer struct {
	editor  *codeEditor
	scroll  *container.Scroll
	caret   *canvas.Rectangle
	overlay *fyne.Container
}

func (r *codeEditorRenderer) Layout(size fyne.Size) {
	r.scroll.Resize(size)
	r.scroll.Move(fyne.NewPos(0, 0))
	r.editor.updateCaret()
	r.layoutCompletion()
}

func (r *codeEditorRenderer) layoutCompletion() {
	if r.overlay == nil || !r.overlay.Visible() || r.editor.completion == nil {
		return
	}
	pos := r.editor.completionPopupRelPos()
	panel := r.editor.completion.panelSize()
	r.editor.completion.layoutAt(pos, panel)
}

func (r *codeEditorRenderer) MinSize() fyne.Size {
	return r.scroll.MinSize()
}

func (r *codeEditorRenderer) Refresh() {
	r.scroll.Refresh()
	r.editor.updateCaret()
	if r.overlay.Visible() {
		r.layoutCompletion()
		r.overlay.Refresh()
	}
}

func (r *codeEditorRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.scroll, r.caret, r.overlay}
}

func (r *codeEditorRenderer) Destroy() {
	r.editor.stopCursorBlink()
}
