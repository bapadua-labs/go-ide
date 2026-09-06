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

// goTabWidth é a largura visual do tab usada pelo gofmt (8 colunas).
const goTabWidth = 8

type editorSnapshot struct {
	text      string
	cursorRow int
	cursorCol int
}

type codeEditor struct {
	widget.BaseWidget

	grid             *widget.TextGrid
	scroll           *container.Scroll
	text             string
	cursorRow        int
	cursorCol        int
	selAnchorRow     int
	selAnchorCol     int
	selActive        bool
	selecting        bool
	cursorVisible    bool
	blinkStop        chan struct{}
	caret            *canvas.Rectangle
	placeholder      string
	hasFocus         bool
	ctrlDown         bool
	shiftDown        bool
	pendingRefresh   bool
	completion       *completionPopup
	completionOpen   bool
	completionLayer  *fyne.Container
	rawCompletions   []completionSuggestion
	completionTimer  *time.Timer
	undoStack        []editorSnapshot
	onAppShortcut    func(fyne.Shortcut)
	onGoToDefinition func(row, col int)
	onFindReferences func(row, col int)
	onRename         func(row, col int)
	onHover          func(row, col int)
	onSignatureHelp  func(row, col int)
	diagnostics      []fileDiagnostic
	hoverLayer       *fyne.Container
	signatureLayer   *fyne.Container
	hoverPopup       *lspTextPopup
	signaturePopup   *lspTextPopup
	hoverTimer       *time.Timer
	lastHoverRow     int
	lastHoverCol     int
	OnChanged        func(string)
	OnCompletion     func(row, col int)
}

func newCodeEditor() *codeEditor {
	ed := &codeEditor{
		grid: widget.NewTextGrid(),
	}
	ed.grid.ShowLineNumbers = true
	ed.grid.Scroll = fyne.ScrollNone
	ed.grid.TabWidth = goTabWidth
	ed.ExtendBaseWidget(ed)
	return ed
}

// AcceptsTab faz o Fyne entregar Tab ao editor em vez de trocar o foco.
func (ed *codeEditor) AcceptsTab() bool {
	return true
}

func (ed *codeEditor) CreateRenderer() fyne.WidgetRenderer {
	ed.scroll = container.NewScroll(ed.grid)
	ed.scroll.OnScrolled = func(_ fyne.Position) {
		ed.updateCaret()
	}
	ed.completionLayer = container.NewWithoutLayout()
	ed.completionLayer.Hide()
	ed.hoverLayer = container.NewWithoutLayout()
	ed.hoverLayer.Hide()
	ed.signatureLayer = container.NewWithoutLayout()
	ed.signatureLayer.Hide()
	ed.hoverPopup = newLSPTextPopup(ed.hoverLayer)
	ed.signaturePopup = newLSPTextPopup(ed.signatureLayer)
	th := fyne.CurrentApp().Settings().Theme()
	ed.caret = canvas.NewRectangle(th.Color(theme.ColorNameForeground, theme.VariantDark))
	ed.caret.Hidden = true
	if ed.pendingRefresh || ed.placeholder != "" || ed.text != "" {
		ed.pendingRefresh = false
		ed.doRefreshGrid(ed.MinSize())
	}
	return &codeEditorRenderer{
		editor:    ed,
		scroll:    ed.scroll,
		caret:     ed.caret,
		overlay:   ed.completionLayer,
		hover:     ed.hoverLayer,
		signature: ed.signatureLayer,
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
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		ed.shiftDown = true
	}
}

func (ed *codeEditor) KeyUp(key *fyne.KeyEvent) {
	switch key.Name {
	case desktop.KeyControlLeft, desktop.KeyControlRight:
		ed.ctrlDown = false
	case desktop.KeyShiftLeft, desktop.KeyShiftRight:
		ed.shiftDown = false
	}
}

func (ed *codeEditor) modifierShift() bool {
	if ed.shiftDown {
		return true
	}
	if fyne.CurrentApp() == nil {
		return false
	}
	if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
		return d.CurrentKeyModifiers()&fyne.KeyModifierShift != 0
	}
	return false
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

	extend := ed.modifierShift()

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
		if ed.modifierShift() {
			return // Shift+Tab: não insere tab (desindentação fica para depois)
		}
		ed.insertString("\t")
	case fyne.KeyEscape:
		if ed.completionOpen {
			ed.hideCompletion()
			return
		}
		if ed.hasSelection() {
			ed.clearSelection()
			ed.refreshGrid()
		}
	case fyne.KeyLeft, fyne.KeyRight:
		ed.hideCompletion()
		switch ev.Name {
		case fyne.KeyLeft:
			ed.moveLeft(extend)
		case fyne.KeyRight:
			ed.moveRight(extend)
		}
	case fyne.KeyUp:
		if !extend && ed.completionOpen && ed.completion != nil {
			ed.completion.SelectPrev()
			ed.completion.Show()
			return
		}
		ed.moveUp(extend)
	case fyne.KeyDown:
		if !extend && ed.completionOpen && ed.completion != nil {
			ed.completion.SelectNext()
			ed.completion.Show()
			return
		}
		ed.moveDown(extend)
	case fyne.KeyHome:
		ed.hideCompletion()
		ed.moveHome(extend)
	case fyne.KeyEnd:
		ed.hideCompletion()
		ed.moveEnd(extend)
	case fyne.KeyF12:
		if ed.onGoToDefinition != nil {
			ed.onGoToDefinition(ed.cursorRow, ed.cursorCol)
		}
	case fyne.KeyF2:
		if ed.onRename != nil {
			ed.onRename(ed.cursorRow, ed.cursorCol)
		}
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
	ed.clearSelection()
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

func (ed *codeEditor) SetCursor(row, col int) {
	ed.cursorRow = row
	ed.cursorCol = col
	ed.clearSelection()
	ed.clampCursor()
	ed.refreshGrid()
}

func (ed *codeEditor) SetDiagnostics(diags []fileDiagnostic) {
	ed.diagnostics = diags
	ed.refreshGrid()
}

func (ed *codeEditor) ShowHover(content string) {
	ed.ShowHoverAt(ed.cursorRow, ed.cursorCol, content)
}

func (ed *codeEditor) ShowHoverAt(row, byteCol int, content string) {
	ed.signaturePopup.Hide()
	pos := ed.popupPositionFor(row, byteCol)
	ed.hoverPopup.ShowAt(pos, content)
}

func (ed *codeEditor) ShowSignatureHelp(content string) {
	ed.hoverPopup.Hide()
	pos := ed.completionPopupRelPos()
	ed.signaturePopup.ShowAt(pos, content)
}

func (ed *codeEditor) popupPositionFor(row, byteCol int) fyne.Position {
	if ed.grid == nil {
		return fyne.NewPos(0, 0)
	}
	gridCol := ed.gutterCols() + ed.displayColFor(row, byteCol)
	cellPos := ed.grid.PositionForCursorLocation(row, gridCol)
	below := ed.grid.PositionForCursorLocation(row+1, gridCol)
	return ed.gridOriginInEditor().Add(fyne.NewPos(cellPos.X, below.Y+2))
}

func (ed *codeEditor) displayColFor(row, byteCol int) int {
	lines := ed.lines()
	if row < 0 || row >= len(lines) {
		return 0
	}
	line := lines[row]
	col := 0
	for i := 0; i < byteCol && i < len(line); {
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
		col++
	}
	return col
}

func (ed *codeEditor) hideLSPPopups() {
	ed.hoverPopup.Hide()
	ed.signaturePopup.Hide()
}

func (ed *codeEditor) scheduleHover(row, col int) {
	if ed.onHover == nil {
		return
	}
	if ed.hoverTimer != nil {
		ed.hoverTimer.Stop()
	}
	ed.lastHoverRow = row
	ed.lastHoverCol = col
	ed.hoverTimer = time.AfterFunc(400*time.Millisecond, func() {
		fyne.Do(func() {
			if ed.lastHoverRow != row || ed.lastHoverCol != col {
				return
			}
			if ed.onHover != nil {
				ed.onHover(ed.lastHoverRow, ed.lastHoverCol)
			}
		})
	})
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
	return visualColAtByte(lines[ed.cursorRow], ed.cursorCol, goTabWidth)
}

// nextTabStop retorna a próxima coluna de tab stop após (ou em) col.
func nextTabStop(col, tabWidth int) int {
	if tabWidth <= 0 {
		tabWidth = goTabWidth
	}
	return ((col / tabWidth) + 1) * tabWidth
}

// visualColAtByte converte offset em bytes na linha para coluna visual (tabs expandem).
func visualColAtByte(line string, byteCol, tabWidth int) int {
	if byteCol > len(line) {
		byteCol = len(line)
	}
	if byteCol < 0 {
		byteCol = 0
	}
	vis := 0
	for i := 0; i < byteCol; {
		r, size := utf8.DecodeRuneInString(line[i:])
		i += size
		if r == '\t' {
			vis = nextTabStop(vis, tabWidth)
		} else {
			vis++
		}
	}
	return vis
}

// byteColAtVisual converte coluna visual para offset em bytes na linha.
func byteColAtVisual(line string, visualCol, tabWidth int) int {
	if visualCol <= 0 {
		return 0
	}
	vis := 0
	byteCol := 0
	for byteCol < len(line) && vis < visualCol {
		r, size := utf8.DecodeRuneInString(line[byteCol:])
		if r == '\t' {
			next := nextTabStop(vis, tabWidth)
			if visualCol < next {
				return byteCol
			}
			vis = next
			byteCol += size
			continue
		}
		vis++
		byteCol += size
	}
	return byteCol
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
	row, col := ed.rowColFromPoint(pos)
	ed.cursorRow = row
	ed.cursorCol = col
	ed.clampCursor()
	ed.cursorVisible = true
	ed.hideCompletion()
	ed.refreshGrid()
}

func (ed *codeEditor) rowColFromPoint(pos fyne.Position) (int, int) {
	if ed.grid == nil {
		return 0, 0
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
	return row, byteColAtVisual(line, textCol, goTabWidth)
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
	insert := expandSnippet(item.InsertText)
	ed.pushUndo()
	ed.text = ed.text[:from] + insert + ed.text[to:]
	ed.clearSelection()
	ed.setCursorFromOffset(from + len(insert))
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
	ed.clearSelection()
	ed.hideCompletion()
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) hasSelection() bool {
	if !ed.selActive {
		return false
	}
	start, end := ed.selectionOffsets()
	return start < end
}

func (ed *codeEditor) clearSelection() {
	ed.selActive = false
	ed.selAnchorRow = ed.cursorRow
	ed.selAnchorCol = ed.cursorCol
}

func (ed *codeEditor) ensureSelectionAnchor() {
	if ed.selActive {
		return
	}
	ed.selAnchorRow = ed.cursorRow
	ed.selAnchorCol = ed.cursorCol
	ed.selActive = true
}

func (ed *codeEditor) syncSelectionAfterMove(extend bool) {
	if extend {
		ed.selActive = ed.selAnchorRow != ed.cursorRow || ed.selAnchorCol != ed.cursorCol
		return
	}
	ed.clearSelection()
}

func (ed *codeEditor) byteOffsetAt(row, col int) int {
	lines := ed.lines()
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	if col < 0 {
		col = 0
	}
	if col > len(lines[row]) {
		col = len(lines[row])
	}
	offset := 0
	for i := 0; i < row; i++ {
		offset += len(lines[i]) + 1
	}
	return offset + col
}

func (ed *codeEditor) selectionOffsets() (int, int) {
	cur := ed.cursorByteOffset()
	if !ed.selActive {
		return cur, cur
	}
	anchor := ed.byteOffsetAt(ed.selAnchorRow, ed.selAnchorCol)
	if anchor > cur {
		return cur, anchor
	}
	return anchor, cur
}

func (ed *codeEditor) deleteSelection() bool {
	if !ed.hasSelection() {
		return false
	}
	start, end := ed.selectionOffsets()
	ed.pushUndo()
	ed.text = ed.text[:start] + ed.text[end:]
	ed.clearSelection()
	ed.setCursorFromOffset(start)
	ed.refreshGrid()
	ed.notifyChanged()
	return true
}

func (ed *codeEditor) insertString(s string) {
	if s == "" {
		return
	}
	if ed.hasSelection() {
		start, end := ed.selectionOffsets()
		ed.pushUndo()
		ed.text = ed.text[:start] + s + ed.text[end:]
		ed.clearSelection()
		ed.setCursorFromOffset(start + len(s))
		ed.refreshGrid()
		ed.notifyChanged()
		if strings.Contains(s, "(") && ed.onSignatureHelp != nil {
			ed.onSignatureHelp(ed.cursorRow, ed.cursorCol)
		}
		return
	}
	ed.pushUndo()
	offset := ed.cursorByteOffset()
	ed.text = ed.text[:offset] + s + ed.text[offset:]
	ed.setCursorFromOffset(offset + len(s))
	ed.refreshGrid()
	ed.notifyChanged()
	if strings.Contains(s, "(") && ed.onSignatureHelp != nil {
		ed.onSignatureHelp(ed.cursorRow, ed.cursorCol)
	}
}

func (ed *codeEditor) deleteBefore() {
	if ed.deleteSelection() {
		return
	}
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
	if ed.deleteSelection() {
		return
	}
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

func (ed *codeEditor) moveLeft(extend bool) {
	if !extend && ed.hasSelection() {
		start, _ := ed.selectionOffsets()
		ed.clearSelection()
		ed.setCursorFromOffset(start)
		ed.refreshGrid()
		return
	}
	if extend {
		ed.ensureSelectionAnchor()
	}
	if ed.cursorCol > 0 {
		_, size := utf8.DecodeLastRuneInString(ed.lines()[ed.cursorRow][:ed.cursorCol])
		ed.cursorCol -= size
	} else if ed.cursorRow > 0 {
		ed.cursorRow--
		ed.cursorCol = ed.lineLen(ed.cursorRow)
	}
	ed.syncSelectionAfterMove(extend)
	ed.refreshGrid()
}

func (ed *codeEditor) moveRight(extend bool) {
	if !extend && ed.hasSelection() {
		_, end := ed.selectionOffsets()
		ed.clearSelection()
		ed.setCursorFromOffset(end)
		ed.refreshGrid()
		return
	}
	if extend {
		ed.ensureSelectionAnchor()
	}
	line := ed.lines()[ed.cursorRow]
	if ed.cursorCol < len(line) {
		_, size := utf8.DecodeRuneInString(line[ed.cursorCol:])
		ed.cursorCol += size
	} else if ed.cursorRow < len(ed.lines())-1 {
		ed.cursorRow++
		ed.cursorCol = 0
	}
	ed.syncSelectionAfterMove(extend)
	ed.refreshGrid()
}

func (ed *codeEditor) moveUp(extend bool) {
	if !extend && ed.hasSelection() {
		start, _ := ed.selectionOffsets()
		ed.clearSelection()
		ed.setCursorFromOffset(start)
		ed.refreshGrid()
		return
	}
	if extend {
		ed.ensureSelectionAnchor()
	}
	if ed.cursorRow > 0 {
		ed.cursorRow--
		ed.clampCursor()
	}
	ed.syncSelectionAfterMove(extend)
	ed.refreshGrid()
}

func (ed *codeEditor) moveDown(extend bool) {
	if !extend && ed.hasSelection() {
		_, end := ed.selectionOffsets()
		ed.clearSelection()
		ed.setCursorFromOffset(end)
		ed.refreshGrid()
		return
	}
	if extend {
		ed.ensureSelectionAnchor()
	}
	if ed.cursorRow < len(ed.lines())-1 {
		ed.cursorRow++
		ed.clampCursor()
	}
	ed.syncSelectionAfterMove(extend)
	ed.refreshGrid()
}

func (ed *codeEditor) moveHome(extend bool) {
	if extend {
		ed.ensureSelectionAnchor()
	} else if ed.hasSelection() {
		ed.clearSelection()
	}
	ed.cursorCol = 0
	ed.syncSelectionAfterMove(extend)
	ed.refreshGrid()
}

func (ed *codeEditor) moveEnd(extend bool) {
	if extend {
		ed.ensureSelectionAnchor()
	} else if ed.hasSelection() {
		ed.clearSelection()
	}
	ed.cursorCol = ed.lineLen(ed.cursorRow)
	ed.syncSelectionAfterMove(extend)
	ed.refreshGrid()
}

func (ed *codeEditor) paste() {
	content := fyne.CurrentApp().Clipboard().Content()
	if content == "" {
		return
	}
	ed.insertString(content)
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
	ed.clearSelection()
	ed.setCursorFromOffset(start)
	ed.refreshGrid()
	ed.notifyChanged()
}

func (ed *codeEditor) selectAll() {
	lines := ed.lines()
	ed.selAnchorRow = 0
	ed.selAnchorCol = 0
	ed.cursorRow = len(lines) - 1
	ed.cursorCol = ed.lineLen(ed.cursorRow)
	ed.selActive = ed.cursorRow != 0 || ed.cursorCol != 0 || len(ed.text) > 0
	ed.refreshGrid()
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
	selBG := th.Color(theme.ColorNameSelection, theme.VariantDark)
	selStart, selEnd := 0, 0
	if !isPlaceholder && ed.hasSelection() {
		selStart, selEnd = ed.selectionOffsets()
	}

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
			} else if d := diagnosticForBytePos(display, ed.diagnostics, bytePos); d != nil {
				style = diagnosticStyle(d.Severity, fg)
			} else if c := bracketColorAt(display, bracketIdx, bytePos); c != nil {
				style.FGColor = c
			} else if c := syntaxColorAt(display, syntaxIdx, bytePos); c != nil {
				style.FGColor = c
			}
			if selEnd > selStart && bytePos >= selStart && bytePos < selEnd {
				style.BGColor = selBG
			}
			cells = append(cells, widget.TextGridCell{Rune: r, Style: style})
			if r == '\t' {
				next := nextTabStop(len(cells)-1, goTabWidth)
				for len(cells) < next {
					cells = append(cells, widget.TextGridCell{Rune: ' ', Style: style})
				}
			}
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
var _ fyne.Tabbable = (*codeEditor)(nil)

func (ed *codeEditor) TypedShortcut(shortcut fyne.Shortcut) {
	switch s := shortcut.(type) {
	case *desktop.CustomShortcut:
		switch {
		case s.KeyName == fyne.KeyF12 && s.Modifier == fyne.KeyModifierShift && ed.onFindReferences != nil:
			ed.onFindReferences(ed.cursorRow, ed.cursorCol)
			return
		case s.KeyName == fyne.KeyF12 && ed.onGoToDefinition != nil:
			ed.onGoToDefinition(ed.cursorRow, ed.cursorCol)
			return
		case s.KeyName == fyne.KeyF2 && ed.onRename != nil:
			ed.onRename(ed.cursorRow, ed.cursorCol)
			return
		}
	case *fyne.ShortcutUndo:
		ed.undo()
	case *fyne.ShortcutPaste:
		content := s.Clipboard.Content()
		if content == "" {
			return
		}
		ed.insertString(content)
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
	if d, ok := fyne.CurrentApp().Driver().(desktop.Driver); ok {
		if d.CurrentKeyModifiers()&fyne.KeyModifierControl != 0 {
			return
		}
	}
	// MouseDown/MouseMoved já cuidam do clique e do drag; evita limpar a seleção.
	if ed.selecting || ed.hasSelection() {
		return
	}
	ed.setCursorFromPoint(ev.Position)
	ed.clearSelection()
}

func (ed *codeEditor) MouseDown(ev *desktop.MouseEvent) {
	if ev.Button != desktop.MouseButtonPrimary {
		return
	}
	if c := fyne.CurrentApp().Driver().CanvasForObject(ed); c != nil {
		c.Focus(ed)
	}
	if ev.Modifier&fyne.KeyModifierControl != 0 && ed.onGoToDefinition != nil {
		row, byteCol := ed.rowColFromPoint(ev.Position)
		ed.onGoToDefinition(row, byteCol)
		return
	}

	row, col := ed.rowColFromPoint(ev.Position)
	ed.selecting = true
	ed.cursorVisible = true
	ed.hideCompletion()
	ed.hideLSPPopups()

	if ev.Modifier&fyne.KeyModifierShift != 0 {
		if !ed.selActive {
			ed.selAnchorRow = ed.cursorRow
			ed.selAnchorCol = ed.cursorCol
		}
		ed.cursorRow = row
		ed.cursorCol = col
		ed.clampCursor()
		ed.selActive = ed.selAnchorRow != ed.cursorRow || ed.selAnchorCol != ed.cursorCol
	} else {
		ed.cursorRow = row
		ed.cursorCol = col
		ed.clampCursor()
		ed.selAnchorRow = ed.cursorRow
		ed.selAnchorCol = ed.cursorCol
		ed.selActive = false
	}
	ed.refreshGrid()
}

func (ed *codeEditor) MouseUp(ev *desktop.MouseEvent) {
	if ev.Button == desktop.MouseButtonPrimary {
		ed.selecting = false
	}
}

func (ed *codeEditor) MouseIn(*desktop.MouseEvent) {}

func (ed *codeEditor) MouseMoved(ev *desktop.MouseEvent) {
	if ed.selecting {
		row, col := ed.rowColFromPoint(ev.Position)
		if row != ed.cursorRow || col != ed.cursorCol {
			ed.cursorRow = row
			ed.cursorCol = col
			ed.clampCursor()
			ed.selActive = ed.selAnchorRow != ed.cursorRow || ed.selAnchorCol != ed.cursorCol
			ed.cursorVisible = true
			ed.refreshGrid()
		}
		return
	}

	if ed.onHover == nil || ed.grid == nil {
		return
	}
	gridPos := ed.gridPointFromEditorPoint(ev.Position)
	_, col := ed.grid.CursorLocationForPosition(gridPos)
	textCol := col - ed.gutterCols()
	if textCol < 0 {
		ed.hideLSPPopups()
		return
	}
	byteRow, byteCol := ed.rowColFromPoint(ev.Position)
	lines := ed.lines()
	if byteRow < 0 || byteRow >= len(lines) {
		ed.hideLSPPopups()
		return
	}
	_, _, word := identifierAt(ed.text, byteRow, byteCol)
	if word == "" {
		ed.hideLSPPopups()
		return
	}
	ed.scheduleHover(byteRow, byteCol)
}

func (ed *codeEditor) MouseOut() {
	if ed.hoverTimer != nil {
		ed.hoverTimer.Stop()
	}
	ed.hideLSPPopups()
}

var _ desktop.Mouseable = (*codeEditor)(nil)
var _ desktop.Hoverable = (*codeEditor)(nil)

type codeEditorRenderer struct {
	editor    *codeEditor
	scroll    *container.Scroll
	caret     *canvas.Rectangle
	overlay   *fyne.Container
	hover     *fyne.Container
	signature *fyne.Container
}

func (r *codeEditorRenderer) Layout(size fyne.Size) {
	r.scroll.Resize(size)
	r.scroll.Move(fyne.NewPos(0, 0))
	if r.hover != nil {
		r.hover.Resize(size)
		r.hover.Move(fyne.NewPos(0, 0))
	}
	if r.signature != nil {
		r.signature.Resize(size)
		r.signature.Move(fyne.NewPos(0, 0))
	}
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
	return []fyne.CanvasObject{r.scroll, r.caret, r.overlay, r.hover, r.signature}
}

func (r *codeEditorRenderer) Destroy() {
	r.editor.stopCursorBlink()
}
