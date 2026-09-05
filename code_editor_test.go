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
