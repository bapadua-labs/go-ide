package main

import (
	"strings"

	"fyne.io/fyne/v2"
)

func (ed *editor) ensureGopls() {
	if ed.rootPath == "" {
		return
	}
	if err := ed.gopls.start(ed.goPath(), ed.rootPath); err != nil {
		// Autocomplete é opcional; não bloqueia o editor.
		return
	}
}

func (ed *editor) syncGoplsDocument() {
	if ed.filePath == "" || !strings.HasSuffix(ed.filePath, ".go") {
		return
	}
	ed.ensureGopls()
	_ = ed.gopls.syncDocument(ed.filePath, ed.entry.Text())
}

func (ed *editor) fetchCompletions(row, col int) {
	if ed.filePath == "" || !strings.HasSuffix(ed.filePath, ".go") {
		return
	}
	ed.ensureGopls()

	path := ed.filePath
	text := ed.entry.Text()

	go func() {
		items, err := ed.gopls.completions(path, text, row, col)
		if err != nil {
			return
		}
		fyne.Do(func() {
			if ed.filePath != path || ed.entry.Text() != text {
				return
			}
			ed.entry.ShowCompletions(items)
		})
	}()
}

func (ed *editor) restartGopls() {
	ed.gopls.stop()
	ed.ensureGopls()
	ed.syncGoplsDocument()
}
