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
