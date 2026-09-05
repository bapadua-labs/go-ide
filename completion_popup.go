package main

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// completionPopup é inspirado no CompletionEntry do fyne-x.
type completionPopup struct {
	popup       *widget.PopUp
	list        *completionList
	suggestions []completionSuggestion
	selected    int
	onAccept    func(completionSuggestion)
	onDismiss   func()
	itemHeight  float32
	anchor      fyne.CanvasObject
	cursorRow   int
}

type completionList struct {
	widget.List
	popup      *completionPopup
	navigating bool
}

func newCompletionPopup(anchor fyne.CanvasObject, onAccept func(completionSuggestion), onDismiss func()) *completionPopup {
	cp := &completionPopup{
		anchor:    anchor,
		onAccept:  onAccept,
		onDismiss: onDismiss,
		selected:  -1,
	}
	cp.list = &completionList{popup: cp}
	cp.list.List = widget.List{
		Length: func() int {
			return len(cp.suggestions)
		},
		CreateItem: func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		UpdateItem: func(i widget.ListItemID, o fyne.CanvasObject) {
			if i < 0 || i >= len(cp.suggestions) {
				return
			}
			o.(*widget.Label).SetText(displayLabel(cp.suggestions[i]))
		},
		OnSelected: func(id widget.ListItemID) {
			if !cp.list.navigating && id >= 0 && id < len(cp.suggestions) {
				cp.accept(id)
			}
			cp.list.navigating = false
		},
	}
	cp.list.ExtendBaseWidget(cp.list)
	return cp
}

func displayLabel(item completionSuggestion) string {
	if item.Detail != "" {
		return item.Label + "  " + item.Detail
	}
	return item.Label
}

func (cp *completionPopup) SetSuggestions(items []completionSuggestion, cursorRow int) {
	cp.suggestions = items
	cp.cursorRow = cursorRow
	cp.selected = -1
	if cp.list != nil {
		cp.list.UnselectAll()
		cp.list.Refresh()
	}
}

func (cp *completionPopup) Show() {
	if len(cp.suggestions) == 0 {
		cp.Hide()
		return
	}

	cnv := fyne.CurrentApp().Driver().CanvasForObject(cp.anchor)
	if cnv == nil {
		return
	}

	if cp.popup == nil {
		cp.popup = widget.NewPopUp(cp.list, cnv)
	}

	if cp.itemHeight == 0 {
		cp.itemHeight = widget.NewLabel("").MinSize().Height
	}

	cp.popup.Resize(cp.maxSize(cnv.Size()))
	cp.popup.ShowAtPosition(cp.popUpPos())
	cnv.Focus(cp.list)
}

func (cp *completionPopup) Hide() {
	if cp.popup != nil {
		cp.popup.Hide()
	}
	cp.selected = -1
	if cp.onDismiss != nil {
		cp.onDismiss()
	}
}

func (cp *completionPopup) Visible() bool {
	return cp.popup != nil && cp.popup.Visible()
}

func (cp *completionPopup) accept(index int) {
	if index < 0 || index >= len(cp.suggestions) {
		return
	}
	item := cp.suggestions[index]
	if cp.popup != nil {
		cp.popup.Hide()
	}
	if cp.onAccept != nil {
		cp.onAccept(item)
	}
}

func (cp *completionPopup) maxSize(canvasSize fyne.Size) fyne.Size {
	anchorPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(cp.anchor)
	anchorSize := cp.anchor.Size()

	listHeight := float32(len(cp.suggestions))*(cp.itemHeight+2*theme.Padding()+theme.SeparatorThicknessSize()) + 2*theme.Padding()
	maxHeight := canvasSize.Height - anchorPos.Y - anchorSize.Height - 2*theme.Padding()
	if listHeight > maxHeight {
		listHeight = maxHeight
	}
	if listHeight < cp.itemHeight+2*theme.Padding() {
		listHeight = cp.itemHeight + 2*theme.Padding()
	}

	width := anchorSize.Width
	if width < 240 {
		width = 240
	}
	return fyne.NewSize(width, listHeight)
}

func (cp *completionPopup) popUpPos() fyne.Position {
	anchorPos := fyne.CurrentApp().Driver().AbsolutePositionForObject(cp.anchor)
	lineHeight := widget.NewLabel("").MinSize().Height + theme.Padding()
	y := anchorPos.Y + float32(cp.cursorRow+1)*lineHeight
	return fyne.NewPos(anchorPos.X, y)
}

func (cp *completionPopup) TypedKey(ev *fyne.KeyEvent) bool {
	if !cp.Visible() {
		return false
	}

	switch ev.Name {
	case fyne.KeyDown:
		cp.moveSelection(1)
		return true
	case fyne.KeyUp:
		cp.moveSelection(-1)
		return true
	case fyne.KeyTab, fyne.KeyReturn, fyne.KeyEnter:
		if cp.selected >= 0 {
			cp.accept(cp.selected)
		} else if len(cp.suggestions) > 0 {
			cp.accept(0)
		}
		return true
	case fyne.KeyEscape:
		cp.Hide()
		return true
	}
	return false
}

func (cp *completionPopup) moveSelection(delta int) {
	if len(cp.suggestions) == 0 {
		return
	}
	cp.list.navigating = true
	if cp.selected < 0 {
		cp.selected = 0
	} else {
		cp.selected += delta
		if cp.selected < 0 {
			cp.selected = len(cp.suggestions) - 1
		}
		if cp.selected >= len(cp.suggestions) {
			cp.selected = 0
		}
	}
	cp.list.Select(cp.selected)
}

func (cl *completionList) TypedKey(event *fyne.KeyEvent) {
	cl.popup.TypedKey(event)
}

func (cl *completionList) TypedRune(r rune) {
	cl.popup.Hide()
}

func (cl *completionList) KeyDown(*fyne.KeyEvent) {}
func (cl *completionList) KeyUp(*fyne.KeyEvent) {}

var _ desktop.Keyable = (*completionList)(nil)

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
