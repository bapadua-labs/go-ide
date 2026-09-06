package main

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestCodeEditorUndoStack(t *testing.T) {
	ed := &codeEditor{text: "hello world", cursorRow: 0, cursorCol: 11}
	ed.pushUndo()
	ed.text = "changed"
	ed.cursorCol = 7

	ed.undo()
	if ed.text != "hello world" {
		t.Fatalf("após undo: %q", ed.text)
	}
	if ed.cursorCol != 11 {
		t.Fatalf("cursor após undo: %d", ed.cursorCol)
	}
}

func TestCodeEditorTypedShortcutUndo(t *testing.T) {
	ed := &codeEditor{text: "abc"}
	ed.pushUndo()
	ed.text = ""

	ed.TypedShortcut(&fyne.ShortcutUndo{})
	if ed.text != "abc" {
		t.Fatalf("Ctrl+Z via TypedShortcut: %q", ed.text)
	}
}

func TestCodeEditorSelectionOffsets(t *testing.T) {
	ed := &codeEditor{text: "abc\ndef", cursorRow: 1, cursorCol: 2}
	ed.selAnchorRow = 0
	ed.selAnchorCol = 1
	ed.selActive = true

	start, end := ed.selectionOffsets()
	if start != 1 || end != 6 {
		t.Fatalf("offsets: got %d,%d want 1,6", start, end)
	}
	if !ed.hasSelection() {
		t.Fatal("esperava seleção ativa")
	}
}

func TestCodeEditorSelectAll(t *testing.T) {
	ed := &codeEditor{text: "ab\nc"}
	ed.selectAll()
	if !ed.hasSelection() {
		t.Fatal("Ctrl+A deve criar seleção")
	}
	start, end := ed.selectionOffsets()
	if start != 0 || end != len(ed.text) {
		t.Fatalf("selectAll offsets: %d,%d", start, end)
	}
}

func TestCodeEditorDeleteSelection(t *testing.T) {
	ed := &codeEditor{text: "hello world", cursorRow: 0, cursorCol: 5}
	ed.selAnchorRow = 0
	ed.selAnchorCol = 0
	ed.selActive = true

	if !ed.deleteSelection() {
		t.Fatal("deleteSelection deveria apagar")
	}
	if ed.text != " world" {
		t.Fatalf("texto após delete: %q", ed.text)
	}
	if ed.hasSelection() {
		t.Fatal("seleção deveria ser limpa")
	}
	if ed.cursorCol != 0 {
		t.Fatalf("cursor: %d", ed.cursorCol)
	}
}

func TestCodeEditorInsertReplacesSelection(t *testing.T) {
	ed := &codeEditor{text: "abcdef", cursorRow: 0, cursorCol: 4}
	ed.selAnchorRow = 0
	ed.selAnchorCol = 1
	ed.selActive = true

	ed.insertString("XY")
	if ed.text != "aXYef" {
		t.Fatalf("insert sobre seleção: %q", ed.text)
	}
	if ed.hasSelection() {
		t.Fatal("seleção deveria ser limpa após insert")
	}
	if ed.cursorCol != 3 {
		t.Fatalf("cursor: %d", ed.cursorCol)
	}
}

func TestCodeEditorMoveLeftCollapsesSelection(t *testing.T) {
	ed := &codeEditor{text: "abcdef", cursorRow: 0, cursorCol: 4}
	ed.selAnchorRow = 0
	ed.selAnchorCol = 1
	ed.selActive = true

	ed.moveLeft(false)
	if ed.hasSelection() {
		t.Fatal("seta sem Shift deve colapsar seleção")
	}
	if ed.cursorCol != 1 {
		t.Fatalf("cursor deveria ir ao início da seleção: %d", ed.cursorCol)
	}
}

func TestCodeEditorMoveRightExtendsSelection(t *testing.T) {
	ed := &codeEditor{text: "abcdef", cursorRow: 0, cursorCol: 2}
	ed.moveRight(true)
	if !ed.hasSelection() {
		t.Fatal("Shift+Right deve criar seleção")
	}
	start, end := ed.selectionOffsets()
	if start != 2 || end != 3 {
		t.Fatalf("offsets: %d,%d", start, end)
	}
}

func TestCodeEditorCutClearsSelection(t *testing.T) {
	// cut usa clipboard da app; sem app, testamos o caminho de delete via offsets
	ed := &codeEditor{text: "hello", cursorRow: 0, cursorCol: 4}
	ed.selAnchorRow = 0
	ed.selAnchorCol = 1
	ed.selActive = true

	start, end := ed.selectionOffsets()
	ed.text = ed.text[:start] + ed.text[end:]
	ed.clearSelection()
	ed.setCursorFromOffset(start)

	if ed.text != "ho" {
		t.Fatalf("cut simulado: %q", ed.text)
	}
	if ed.hasSelection() {
		t.Fatal("seleção deveria ser limpa")
	}
}

func TestCodeEditorAcceptsTab(t *testing.T) {
	ed := &codeEditor{}
	if !ed.AcceptsTab() {
		t.Fatal("codeEditor deve aceitar Tab (fyne.Tabbable)")
	}
}

func TestCodeEditorTypedKeyTabInsertsGoTab(t *testing.T) {
	ed := &codeEditor{text: "func() {", cursorRow: 0, cursorCol: 8}
	ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if ed.text != "func() {\t" {
		t.Fatalf("Tab deve inserir \\t (gofmt): %q", ed.text)
	}
	if ed.cursorCol != 9 {
		t.Fatalf("cursor após Tab: %d", ed.cursorCol)
	}
}

func TestCodeEditorTypedKeyShiftTabDoesNotInsert(t *testing.T) {
	ed := &codeEditor{text: "x", cursorRow: 0, cursorCol: 1, shiftDown: true}
	ed.TypedKey(&fyne.KeyEvent{Name: fyne.KeyTab})
	if ed.text != "x" {
		t.Fatalf("Shift+Tab não deve inserir: %q", ed.text)
	}
}

func TestVisualColAtByteGoTabs(t *testing.T) {
	line := "\thello"
	if got := visualColAtByte(line, 0, goTabWidth); got != 0 {
		t.Fatalf("col 0: %d", got)
	}
	if got := visualColAtByte(line, 1, goTabWidth); got != 8 {
		t.Fatalf("após tab: %d want 8", got)
	}
	if got := visualColAtByte(line, 2, goTabWidth); got != 9 {
		t.Fatalf("após 'h': %d want 9", got)
	}
}

func TestByteColAtVisualGoTabs(t *testing.T) {
	line := "\thello"
	if got := byteColAtVisual(line, 0, goTabWidth); got != 0 {
		t.Fatalf("visual 0: %d", got)
	}
	if got := byteColAtVisual(line, 4, goTabWidth); got != 0 {
		t.Fatalf("meio do tab deve mapear para o \\t: %d", got)
	}
	if got := byteColAtVisual(line, 8, goTabWidth); got != 1 {
		t.Fatalf("tab stop: %d want 1", got)
	}
	if got := byteColAtVisual(line, 9, goTabWidth); got != 2 {
		t.Fatalf("após h: %d want 2", got)
	}
}

