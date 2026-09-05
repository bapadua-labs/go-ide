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
	"fyne.io/fyne/v2/theme"
)

type editor struct {
	app           fyne.App
	window        fyne.Window
	entry         *codeEditor
	explorer      *packageExplorer
	terminal      *termPanel
	gopls         *goplsClient
	editorSplit   *container.Split
	fileTabs      *container.DocTabs
	tabs          []*fileTab
	filePath      string
	rootPath      string
	modified      bool
	termPanelOpen bool
	termOffset    float64
	lastTermCount int
	config        ideConfig
}

func main() {
	a := app.NewWithID("github.com/bapadua/go-ide")
	a.Settings().SetTheme(newRainbowTheme())
	w := a.NewWindow("Editor de Texto")

	ed := &editor{
		app:    a,
		window: w,
		gopls:  newGoplsClient(),
		config: loadConfig(),
	}
	ed.explorer = newPackageExplorer(w, ed.openFileFromExplorer)
	ed.terminal = newTermPanel(w, "", ed.onTerminalTabsChanged)
	ed.initFileTabs()
	ed.setupGoplsFeatures()
	ed.ensureGoRoot()

	ed.buildMenu()
	ed.setupShortcuts()

	ed.termOffset = 0.72
	ed.terminal.panel.Hide()
	ed.editorSplit = container.NewVSplit(
		ed.fileTabs,
		ed.terminal.panel,
	)

	split := container.NewHSplit(
		ed.explorer.panel,
		ed.editorSplit,
	)
	split.SetOffset(0.22)
	w.SetContent(split)
	w.SetPadded(false)
	w.Resize(fyne.NewSize(1000, 700))
	w.SetCloseIntercept(func() {
		ed.confirmQuit()
	})

	// Estado inicial: uma aba sem título (como um buffer vazio pronto para digitar).
	ed.newFile()

	if ed.config.LastFolder != "" {
		ed.openFolderPath(ed.config.LastFolder)
	}
	w.ShowAndRun()
}

func (ed *editor) buildMenu() {
	newItem := fyne.NewMenuItemWithIcon("Novo", theme.DocumentCreateIcon(), ed.newFile)
	openItem := fyne.NewMenuItemWithIcon("Abrir...", theme.DocumentIcon(), ed.openFile)
	openFolderItem := fyne.NewMenuItemWithIcon("Abrir pasta...", theme.FolderOpenIcon(), ed.openFolder)
	saveItem := fyne.NewMenuItemWithIcon("Salvar", theme.DocumentSaveIcon(), ed.saveFile)
	saveItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyS,
		Modifier: fyne.KeyModifierControl,
	}
	saveAsItem := fyne.NewMenuItemWithIcon("Salvar como...", theme.DownloadIcon(), ed.saveFileAs)
	formatItem := fyne.NewMenuItemWithIcon("Formatar documento", theme.ViewRefreshIcon(), ed.formatDocument)
	formatItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyF,
		Modifier: fyne.KeyModifierShift | fyne.KeyModifierAlt,
	}
	propertiesItem := fyne.NewMenuItemWithIcon("Propriedades...", theme.InfoIcon(), ed.showProperties)
	closeItem := fyne.NewMenuItemWithIcon("Fechar", theme.CancelIcon(), ed.closeActiveTab)
	closeItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyW,
		Modifier: fyne.KeyModifierControl,
	}

	fileItems := []*fyne.MenuItem{newItem, openItem, openFolderItem}
	fileItems = append(fileItems, ed.recentFolderMenuItems()...)
	fileItems = append(fileItems, saveItem, saveAsItem, formatItem, propertiesItem, closeItem)
	fileMenu := fyne.NewMenu("Arquivo", fileItems...)
	quitItem := fyne.NewMenuItemWithIcon("Sair", theme.LogoutIcon(), ed.confirmQuit)
	fileMenu.Items = append(fileMenu.Items, quitItem)

	toggleTermItem := fyne.NewMenuItemWithIcon("Terminal", theme.ComputerIcon(), ed.toggleTerminal)
	toggleTermItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyBackTick,
		Modifier: fyne.KeyModifierControl,
	}
	viewMenu := fyne.NewMenu("Exibir", toggleTermItem)

	newTermTabItem := fyne.NewMenuItemWithIcon("Nova aba", theme.ContentAddIcon(), ed.openTerminalTab)
	newTermTabItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyT,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	}
	termMenu := fyne.NewMenu("Terminal", newTermTabItem)

	runFileItem := fyne.NewMenuItemWithIcon("Executar arquivo", theme.MediaPlayIcon(), ed.runCurrentFile)
	runFileItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyF5}
	runMenu := fyne.NewMenu("Executar", runFileItem)

	goDefItem := fyne.NewMenuItemWithIcon("Ir para definição", theme.NavigateNextIcon(), func() {
		if !ed.hasActiveEditor() {
			return
		}
		ed.goToDefinitionAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	})
	goDefItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyF12}
	findRefsItem := fyne.NewMenuItemWithIcon("Encontrar referências", theme.SearchIcon(), func() {
		if !ed.hasActiveEditor() {
			return
		}
		ed.findReferencesAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	})
	findRefsItem.Shortcut = &desktop.CustomShortcut{
		KeyName:  fyne.KeyF12,
		Modifier: fyne.KeyModifierShift,
	}
	renameItem := fyne.NewMenuItemWithIcon("Renomear símbolo", theme.SearchReplaceIcon(), func() {
		if !ed.hasActiveEditor() {
			return
		}
		ed.renameSymbolAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	})
	renameItem.Shortcut = &desktop.CustomShortcut{KeyName: fyne.KeyF2}
	navMenu := fyne.NewMenu("Navegação", goDefItem, findRefsItem, renameItem)

	ed.window.SetMainMenu(fyne.NewMainMenu(fileMenu, navMenu, viewMenu, termMenu, runMenu))
}

func (ed *editor) setupShortcuts() {
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyS,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		ed.saveFile()
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyF,
		Modifier: fyne.KeyModifierShift | fyne.KeyModifierAlt,
	}, func(_ fyne.Shortcut) {
		ed.formatDocument()
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyBackTick,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		ed.toggleTerminal()
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyF5,
	}, func(_ fyne.Shortcut) {
		ed.runCurrentFile()
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyW,
		Modifier: fyne.KeyModifierControl,
	}, func(_ fyne.Shortcut) {
		ed.closeActiveTab()
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF12}, func(_ fyne.Shortcut) {
		if !ed.hasActiveEditor() {
			return
		}
		ed.goToDefinitionAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyF12,
		Modifier: fyne.KeyModifierShift,
	}, func(_ fyne.Shortcut) {
		if !ed.hasActiveEditor() {
			return
		}
		ed.findReferencesAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	})
	ed.window.Canvas().AddShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyF2}, func(_ fyne.Shortcut) {
		if !ed.hasActiveEditor() {
			return
		}
		ed.renameSymbolAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	})
}

func (ed *editor) handleAppShortcut(shortcut fyne.Shortcut) {
	cs, ok := shortcut.(*desktop.CustomShortcut)
	if !ok {
		return
	}
	if cs.KeyName == fyne.KeyW && cs.Modifier == fyne.KeyModifierControl {
		ed.closeActiveTab()
		return
	}
	if !ed.hasActiveEditor() {
		return
	}
	if cs.KeyName == fyne.KeyF12 && cs.Modifier == fyne.KeyModifierShift {
		ed.findReferencesAt(ed.entry.CursorRow(), ed.entry.CursorCol())
		return
	}
	if cs.KeyName == fyne.KeyF12 && cs.Modifier == 0 {
		ed.goToDefinitionAt(ed.entry.CursorRow(), ed.entry.CursorCol())
		return
	}
	if cs.KeyName == fyne.KeyF2 {
		ed.renameSymbolAt(ed.entry.CursorRow(), ed.entry.CursorCol())
	}
}

func (ed *editor) onTerminalTabsChanged(count int) {
	if count == 0 {
		ed.termPanelOpen = false
	} else if ed.lastTermCount == 0 {
		ed.termPanelOpen = true
	}
	ed.lastTermCount = count
	ed.syncTerminalPanel()
}

func (ed *editor) syncTerminalPanel() {
	if ed.terminal.tabCount() == 0 || !ed.termPanelOpen {
		ed.terminal.panel.Hide()
		ed.editorSplit.Refresh()
		return
	}
	ed.terminal.panel.Show()
	offset := ed.termOffset
	if offset >= 0.99 {
		offset = 0.72
	}
	ed.editorSplit.SetOffset(offset)
	ed.editorSplit.Refresh()
}

func (ed *editor) openTerminalTab() {
	ed.termPanelOpen = true
	ed.terminal.newTab()
	ed.syncTerminalPanel()
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
	if ed.termPanelOpen {
		ed.terminal.focusActive()
	}
}

func (ed *editor) updateTitle() {
	title := "Editor de Texto"
	if ed.rootPath != "" && ed.filePath == "" && !ed.modified {
		title = filepath.Base(ed.rootPath) + " — Editor de Texto"
	}
	if ed.filePath != "" {
		title = filepath.Base(ed.filePath)
	} else if ed.hasActiveEditor() {
		title = "Sem título"
	}
	if ed.modified {
		title += " *"
	}
	ed.window.SetTitle(title)
}

func (ed *editor) newFile() {
	tab := ed.createUntitledTab()
	ed.fileTabs.Append(tab.item)
	ed.activateTab(tab)
}

func (ed *editor) openFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		ed.loadFile(reader.URI().Path())
	}, ed.window)
}

func (ed *editor) recentFolderMenuItems() []*fyne.MenuItem {
	if len(ed.config.RecentFolders) == 0 {
		return nil
	}
	items := []*fyne.MenuItem{fyne.NewMenuItemSeparator()}
	for _, path := range ed.config.RecentFolders {
		folderPath := path
		label := filepath.Base(folderPath)
		if label == "" || label == "." || label == string(os.PathSeparator) {
			label = folderPath
		}
		item := fyne.NewMenuItemWithIcon(label, theme.FolderIcon(), func() {
			ed.openFolderPath(folderPath)
		})
		items = append(items, item)
	}
	return items
}

func (ed *editor) openFolder() {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		ed.openFolderPath(uri.Path())
	}, ed.window)
}

func (ed *editor) openFolderPath(path string) {
	if !folderExists(path) {
		dialog.ShowError(os.ErrNotExist, ed.window)
		return
	}
	ed.rootPath = normalizePath(path)
	ed.explorer.setRoot(ed.rootPath)
	ed.terminal.setWorkingDir(ed.rootPath)
	// gopls initialize é lento; não bloquear a UI (cursor de ocupado).
	go ed.ensureGopls()
	ed.updateTitle()

	ed.config.setLastFolder(ed.rootPath)
	ed.config.addRecentFolder(ed.rootPath)
	_ = ed.config.save()

	ed.buildMenu()
}

func (ed *editor) openFileFromExplorer(path string) {
	ed.loadFile(path)
}

func (ed *editor) loadFile(path string) {
	ed.openOrFocusFile(path)
}

func (ed *editor) saveFile() {
	if !ed.hasActiveEditor() {
		return
	}
	if ed.filePath == "" {
		ed.saveFileAs()
		return
	}
	ed.writeToFile(ed.filePath)
}

func (ed *editor) saveFileAs() {
	if !ed.hasActiveEditor() {
		return
	}
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

		ed.updateActiveTabPath(path)
	}, ed.window)
}

func (ed *editor) writeToFile(path string) {
	if !ed.hasActiveEditor() {
		return
	}
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

	ed.setActiveModified(false)
}

func (ed *editor) writeTo(_ string, w io.Writer) error {
	if ed.entry == nil {
		return nil
	}
	_, err := io.WriteString(w, ed.entry.Text())
	return err
}

func (ed *editor) confirmQuit() {
	if !ed.anyDirtyTabs() {
		ed.saveConfigOnExit()
		ed.window.Close()
		return
	}

	dialog.ShowConfirm(
		"Alterações não salvas",
		"Há arquivos com alterações não salvas. Deseja sair mesmo assim?",
		func(ok bool) {
			if ok {
				ed.saveConfigOnExit()
				ed.window.Close()
			}
		},
		ed.window,
	)
}

func (ed *editor) saveConfigOnExit() {
	if ed.rootPath != "" {
		ed.config.setLastFolder(ed.rootPath)
	}
	_ = ed.config.save()
}
