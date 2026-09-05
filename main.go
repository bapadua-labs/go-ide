package main

import (
	"io"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

type editor struct {
	window      fyne.Window
	entry       *widget.Entry
	explorer    *packageExplorer
	terminal    *termPanel
	editorSplit *container.Split
	filePath    string
	rootPath    string
	modified     bool
	termPanelOpen bool
	termOffset    float64
}

func main() {
	a := app.NewWithID("github.com/bapadua/go-ide")
	w := a.NewWindow("Editor de Texto")

	ed := &editor{
		window: w,
		entry:  widget.NewMultiLineEntry(),
	}
	ed.explorer = newPackageExplorer(ed.openFileFromExplorer)
	ed.terminal = newTermPanel(w, "", ed.onTerminalTabsChanged)
	ed.entry.SetPlaceHolder("Comece a digitar...")
	ed.entry.OnChanged = func(_ string) {
		ed.modified = true
		ed.updateTitle()
	}

	ed.buildMenu()
	ed.setupShortcuts()

	ed.termOffset = 0.72
	ed.editorSplit = container.NewVSplit(
		ed.entry,
		ed.terminal.panel,
	)
	ed.editorSplit.SetOffset(1.0)

	split := container.NewHSplit(
		ed.explorer.panel,
		ed.editorSplit,
	)
	split.SetOffset(0.22)
	w.SetContent(split)
	w.SetPadded(false)
	w.Resize(fyne.NewSize(1000, 700))
	w.SetCloseIntercept(func() {
		ed.confirmClose()
	})
	w.ShowAndRun()
}

func (ed *editor) buildMenu() {
	newItem := fyne.NewMenuItem("Novo", ed.newFile)
	openItem := fyne.NewMenuItem("Abrir...", ed.openFile)
	openFolderItem := fyne.NewMenuItem("Abrir pasta...", ed.openFolder)
	saveItem := fyne.NewMenuItem("Salvar", ed.saveFile)
	saveAsItem := fyne.NewMenuItem("Salvar como...", ed.saveFileAs)

	fileMenu := fyne.NewMenu("Arquivo", newItem, openItem, openFolderItem, saveItem, saveAsItem)
	quitItem := fyne.NewMenuItem("Sair", ed.confirmClose)
	fileMenu.Items = append(fileMenu.Items, quitItem)

	toggleTermItem := fyne.NewMenuItem("Terminal", ed.toggleTerminal)
	toggleTermItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyBackTick,
		Modifier: fyne.KeyModifierControl,
	}
	viewMenu := fyne.NewMenu("Exibir", toggleTermItem)

	newTermTabItem := fyne.NewMenuItem("Nova aba", ed.openTerminalTab)
	newTermTabItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyT,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	}
	termMenu := fyne.NewMenu("Terminal", newTermTabItem)

	ed.window.SetMainMenu(fyne.NewMainMenu(fileMenu, viewMenu, termMenu))
}

func (ed *editor) setupShortcuts() {
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyBackTick,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		ed.toggleTerminal()
	})
}

func (ed *editor) onTerminalTabsChanged(count int) {
	if count == 0 {
		ed.editorSplit.SetOffset(1.0)
		ed.termPanelOpen = false
		return
	}
	ed.termPanelOpen = true
	ed.syncTerminalPanel()
}

func (ed *editor) syncTerminalPanel() {
	if ed.terminal.tabCount() == 0 {
		ed.editorSplit.SetOffset(1.0)
		return
	}
	if !ed.termPanelOpen {
		ed.editorSplit.SetOffset(1.0)
		return
	}
	offset := ed.termOffset
	if offset >= 0.99 {
		offset = 0.72
	}
	ed.editorSplit.SetOffset(offset)
}

func (ed *editor) openTerminalTab() {
	ed.termPanelOpen = true
	ed.terminal.newTab()
}

func (ed *editor) toggleTerminal() {
	if ed.terminal.tabCount() == 0 {
		ed.openTerminalTab()
		return
	}
	if ed.termPanelOpen {
		if ed.editorSplit.Offset < 0.99 {
			ed.termOffset = ed.editorSplit.Offset
		}
		ed.termPanelOpen = false
	} else {
		ed.termPanelOpen = true
	}
	ed.syncTerminalPanel()
}

func (ed *editor) updateTitle() {
	title := "Editor de Texto"
	if ed.filePath != "" {
		title = filepath.Base(ed.filePath)
	}
	if ed.modified {
		title += " *"
	}
	ed.window.SetTitle(title)
}

func (ed *editor) newFile() {
	ed.withDiscardConfirmation(func() {
		ed.entry.SetText("")
		ed.filePath = ""
		ed.modified = false
		ed.updateTitle()
	})
}

func (ed *editor) openFile() {
	ed.withDiscardConfirmation(func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			ed.loadFile(reader.URI().Path())
		}, ed.window)
	})
}

func (ed *editor) openFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		ed.rootPath = uri.Path()
		ed.explorer.setRoot(ed.rootPath)
		ed.terminal.setWorkingDir(ed.rootPath)
		ed.window.SetTitle(filepath.Base(ed.rootPath) + " — Editor de Texto")
	}, ed.window)
}

func (ed *editor) openFileFromExplorer(path string) {
	ed.withDiscardConfirmation(func() {
		ed.loadFile(path)
	})
}

func (ed *editor) loadFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, ed.window)
		return
	}

	ed.entry.SetText(string(data))
	ed.filePath = path
	ed.modified = false
	ed.updateTitle()
}

func (ed *editor) saveFile() {
	if ed.filePath == "" {
		ed.saveFileAs()
		return
	}
	ed.writeToFile(ed.filePath)
}

func (ed *editor) saveFileAs() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		path := writer.URI().Path()
		if err := ed.writeTo(path, writer); err != nil {
			dialog.ShowError(err, ed.window)
			return
		}

		ed.filePath = path
		ed.modified = false
		ed.updateTitle()
	}, ed.window)
}

func (ed *editor) writeToFile(path string) {
	f, err := os.Create(path)
	if err != nil {
		dialog.ShowError(err, ed.window)
		return
	}
	defer f.Close()

	if err := ed.writeTo(path, f); err != nil {
		dialog.ShowError(err, ed.window)
		return
	}

	ed.modified = false
	ed.updateTitle()
}

func (ed *editor) writeTo(_ string, w io.Writer) error {
	_, err := io.WriteString(w, ed.entry.Text)
	return err
}

func (ed *editor) withDiscardConfirmation(action func()) {
	if !ed.modified {
		action()
		return
	}

	dialog.ShowConfirm(
		"Alterações não salvas",
		"Deseja descartar as alterações?",
		func(ok bool) {
			if ok {
				action()
			}
		},
		ed.window,
	)
}

func (ed *editor) confirmClose() {
	if !ed.modified {
		ed.window.Close()
		return
	}

	dialog.ShowConfirm(
		"Alterações não salvas",
		"Deseja salvar antes de sair?",
		func(save bool) {
			if !save {
				ed.window.Close()
				return
			}
			if ed.filePath != "" {
				ed.writeToFile(ed.filePath)
				if !ed.modified {
					ed.window.Close()
				}
				return
			}
			ed.saveFileAsOnClose()
		},
		ed.window,
	)
}

func (ed *editor) saveFileAsOnClose() {
	dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()

		path := writer.URI().Path()
		if err := ed.writeTo(path, writer); err != nil {
			dialog.ShowError(err, ed.window)
			return
		}

		ed.filePath = path
		ed.modified = false
		ed.window.Close()
	}, ed.window)
}
