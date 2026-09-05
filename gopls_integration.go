package main

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (ed *editor) setupGoplsFeatures() {
	ed.gopls.setDiagnosticsHandler(func(path string, diags []fileDiagnostic) {
		fyne.Do(func() {
			ed.onDiagnostics(path, diags)
		})
	})
}

func (ed *editor) ensureGopls() {
	if ed.rootPath == "" {
		return
	}
	if err := ed.gopls.start(ed.goPath(), ed.rootPath); err != nil {
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
			if ed.filePath != path {
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

func (ed *editor) onDiagnostics(path string, diags []fileDiagnostic) {
	if !samePath(ed.filePath, path) {
		return
	}
	ed.entry.SetDiagnostics(diags)
}

func (ed *editor) goToDefinitionAt(row, col int) {
	if !ed.isGoFile() {
		dialog.ShowInformation("Definição", "Abra um arquivo Go (.go).", ed.window)
		return
	}
	if ed.rootPath == "" {
		dialog.ShowInformation("Definição", "Abra uma pasta de projeto (Arquivo → Abrir pasta...).", ed.window)
		return
	}
	ed.ensureGopls()
	if !ed.gopls.isReady() {
		dialog.ShowInformation("Definição", "O gopls não está disponível. Instale com:\ngo install golang.org/x/tools/gopls@latest", ed.window)
		return
	}

	text := ed.entry.Text()
	row, col, word := snapToIdentifier(text, row, col)
	if word == "" {
		dialog.ShowInformation("Definição", "Posicione o cursor sobre um identificador.", ed.window)
		return
	}

	ed.syncGoplsDocument()
	path := normalizePath(ed.filePath)

	go func() {
		result, err := ed.gopls.definition(path, text, row, col)
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, ed.window)
			})
			return
		}
		locs := locationsFromDefinition(result)
		if len(locs) == 0 {
			fyne.Do(func() {
				dialog.ShowInformation("Definição", "Nenhuma definição encontrada para \""+word+"\".", ed.window)
			})
			return
		}
		loc := locs[0]
		fyne.Do(func() {
			ed.navigateTo(loc)
		})
	}()
}

func (ed *editor) findReferencesAt(row, col int) {
	if !ed.isGoFile() {
		return
	}
	ed.ensureGopls()
	path := ed.filePath
	text := ed.entry.Text()
	go func() {
		locs, err := ed.gopls.references(path, text, row, col)
		if err != nil {
			return
		}
		refs := locationsFromReferences(locs)
		fyne.Do(func() {
			ed.showReferences(refs)
		})
	}()
}

func (ed *editor) renameSymbolAt(row, col int) {
	if !ed.isGoFile() {
		return
	}
	_, _, word := identifierAt(ed.entry.Text(), row, col)
	if word == "" {
		dialog.ShowInformation("Renomear", "Posicione o cursor sobre um identificador.", ed.window)
		return
	}

	entry := widget.NewEntry()
	entry.SetText(word)
	form := []*widget.FormItem{
		widget.NewFormItem("Novo nome", entry),
	}
	dialog.ShowForm("Renomear símbolo", "Renomear", "Cancelar", form, func(ok bool) {
		if !ok || strings.TrimSpace(entry.Text) == "" || entry.Text == word {
			return
		}
		ed.applyRename(row, col, entry.Text)
	}, ed.window)
}

func (ed *editor) applyRename(row, col int, newName string) {
	ed.ensureGopls()
	path := ed.filePath
	text := ed.entry.Text()
	go func() {
		edit, err := ed.gopls.rename(path, text, row, col, newName)
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, ed.window)
			})
			return
		}
		changed, err := applyWorkspaceEdit(edit)
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, ed.window)
			})
			return
		}
		fyne.Do(func() {
			ed.afterWorkspaceEdit(changed)
		})
	}()
}

func (ed *editor) afterWorkspaceEdit(changed []string) {
	for _, path := range changed {
		if path == ed.filePath {
			ed.reloadCurrentFile()
			continue
		}
		if ed.rootPath != "" && strings.HasPrefix(path, ed.rootPath+string(os.PathSeparator)) {
			_ = ed.gopls.syncDocument(path, readFileOrEmpty(path))
		}
	}
	ed.explorer.setRoot(ed.rootPath)
}

func (ed *editor) reloadCurrentFile() {
	if ed.filePath == "" {
		return
	}
	row := ed.entry.CursorRow()
	col := ed.entry.CursorCol()
	ed.loadFileAt(ed.filePath, row, col)
}

func (ed *editor) fetchHover(row, col int) {
	if !ed.isGoFile() {
		return
	}
	ed.ensureGopls()
	path := ed.filePath
	text := ed.entry.Text()
	go func() {
		hover, err := ed.gopls.hover(path, text, row, col)
		if err != nil || hover == nil {
			return
		}
		content := hoverContentsText(hover.Contents)
		if content == "" {
			return
		}
		fyne.Do(func() {
			if ed.filePath != path {
				return
			}
			if d := ed.entry.diagnosticAt(row, col); d != nil && d.Message != "" {
				ed.entry.ShowHoverAt(row, col, d.Message)
				return
			}
			ed.entry.ShowHoverAt(row, col, content)
		})
	}()
}

func (ed *editor) fetchSignatureHelp(row, col int) {
	if !ed.isGoFile() {
		return
	}
	ed.ensureGopls()
	path := ed.filePath
	text := ed.entry.Text()
	go func() {
		help, err := ed.gopls.signatureHelp(path, text, row, col)
		if err != nil || help == nil {
			return
		}
		content := signatureHelpText(help)
		if content == "" {
			return
		}
		fyne.Do(func() {
			if ed.filePath != path {
				return
			}
			ed.entry.ShowSignatureHelp(content)
		})
	}()
}

func (ed *editor) navigateTo(loc sourceLocation) {
	loc.Path = normalizePath(loc.Path)
	if loc.Path == "" {
		return
	}
	if samePath(loc.Path, ed.filePath) {
		ed.entry.SetCursor(loc.Row, loc.Col)
		return
	}
	ed.withDiscardConfirmation(func() {
		ed.loadFileAt(loc.Path, loc.Row, loc.Col)
	})
}

func (ed *editor) showReferences(refs []sourceLocation) {
	if len(refs) == 0 {
		dialog.ShowInformation("Referências", "Nenhuma referência encontrada.", ed.window)
		return
	}

	list := widget.NewList(
		func() int { return len(refs) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(i widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(locationLabel(refs[i]))
		},
	)
	list.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(refs) {
			return
		}
		loc := refs[id]
		ed.navigateTo(loc)
	}

	content := container.NewBorder(
		widget.NewLabelWithStyle("Referências encontradas", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		list,
	)
	dialog.ShowCustom("Referências", "Fechar", content, ed.window)
}

func (ed *editor) isGoFile() bool {
	return ed.filePath != "" && strings.HasSuffix(ed.filePath, ".go")
}

func (ed *editor) loadFileAt(path string, row, col int) {
	path = normalizePath(path)
	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, ed.window)
		return
	}

	if ed.filePath != "" && strings.HasSuffix(ed.filePath, ".go") {
		_ = ed.gopls.closeDocument(ed.filePath)
	}

	ed.entry.SetText(string(data))
	ed.filePath = path
	ed.modified = false
	ed.updateTitle()
	ed.entry.SetCursor(row, col)

	if ed.rootPath == "" {
		ed.rootPath = filepath.Dir(path)
	}
	ed.ensureGopls()
	ed.syncGoplsDocument()
}
