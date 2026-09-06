package main

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/dialog"
)

func (ed *editor) runCurrentFile() {
	if !ed.hasActiveEditor() || ed.filePath == "" {
		dialog.ShowInformation("Executar", "Salve o arquivo antes de executar.", ed.window)
		return
	}
	if !strings.HasSuffix(ed.filePath, ".go") {
		dialog.ShowInformation("Executar", "Apenas arquivos .go podem ser executados.", ed.window)
		return
	}
	if ed.modified {
		ed.writeToFile(ed.filePath)
	}

	goBin, err := resolveGoBinary(ed.goPath())
	if err != nil {
		dialog.ShowError(err, ed.window)
		return
	}

	dir := filepath.Dir(ed.filePath)
	fileName := filepath.Base(ed.filePath)

	ed.ensureOutputPanelOpen()
	ed.terminal.runGoFile(goBin, ed.goPath(), dir, fileName)
}

func (ed *editor) ensureOutputPanelOpen() {
	ed.termPanelOpen = true
	ed.terminal.ensureOutputTab()
	ed.syncTerminalPanel()
}
