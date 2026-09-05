package main

import (
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
)

// fileTab representa um buffer aberto na barra de abas do editor.
type fileTab struct {
	path     string // vazio = sem título
	modified bool
	preview  bool // true = aba temporária (substituída por outro clique simples no explorador)
	entry    *codeEditor
	item     *container.TabItem
}

func fileTabTitle(path string, modified bool) string {
	return fileTabTitleWithPreview(path, modified, false)
}

func fileTabTitleWithPreview(path string, modified, preview bool) string {
	name := "Sem título"
	if path != "" {
		name = filepath.Base(path)
	}
	if preview {
		// DocTabs não expõe itálico; marcador visual leve estilo preview.
		name = "∘ " + name
	}
	if modified {
		return name + " *"
	}
	return name
}

func findPreviewTab(tabs []*fileTab) *fileTab {
	for _, t := range tabs {
		if t != nil && t.preview {
			return t
		}
	}
	return nil
}

func findFileTabByPath(tabs []*fileTab, path string) *fileTab {
	if path == "" {
		return nil
	}
	path = normalizePath(path)
	for _, t := range tabs {
		if t != nil && t.path != "" && samePath(t.path, path) {
			return t
		}
	}
	return nil
}

func findFileTabByItem(tabs []*fileTab, item *container.TabItem) *fileTab {
	if item == nil {
		return nil
	}
	for _, t := range tabs {
		if t != nil && t.item == item {
			return t
		}
	}
	return nil
}

func (ed *editor) initFileTabs() {
	ed.fileTabs = container.NewDocTabs()
	ed.fileTabs.CloseIntercept = ed.onFileTabCloseIntercept
	ed.fileTabs.OnSelected = ed.onFileTabSelected
	ed.fileTabs.CreateTab = func() *container.TabItem {
		tab := ed.createUntitledTab()
		return tab.item
	}
}

func (ed *editor) wireEditor(entry *codeEditor) {
	entry.SetPlaceHolder("Comece a digitar...")
	entry.OnChanged = func(_ string) {
		ed.markActiveModified()
	}
	entry.OnCompletion = func(row, col int) {
		ed.fetchCompletions(row, col)
	}
	entry.onAppShortcut = ed.handleAppShortcut
	entry.onGoToDefinition = ed.goToDefinitionAt
	entry.onFindReferences = ed.findReferencesAt
	entry.onRename = ed.renameSymbolAt
	entry.onHover = ed.fetchHover
	entry.onSignatureHelp = ed.fetchSignatureHelp
}

func (ed *editor) createUntitledTab() *fileTab {
	entry := newCodeEditor()
	ed.wireEditor(entry)
	tab := &fileTab{
		entry: entry,
	}
	tab.item = container.NewTabItem(fileTabTitle("", false), entry)
	ed.tabs = append(ed.tabs, tab)
	return tab
}

func (ed *editor) createFileTab(path, content string) *fileTab {
	path = normalizePath(path)
	entry := newCodeEditor()
	ed.wireEditor(entry)
	entry.SetText(content)
	tab := &fileTab{
		path:  path,
		entry: entry,
	}
	tab.item = container.NewTabItem(fileTabTitle(path, false), entry)
	ed.tabs = append(ed.tabs, tab)
	return tab
}

func (ed *editor) findTabByPath(path string) *fileTab {
	return findFileTabByPath(ed.tabs, path)
}

func (ed *editor) findTabByItem(item *container.TabItem) *fileTab {
	return findFileTabByItem(ed.tabs, item)
}

func (ed *editor) activeFileTab() *fileTab {
	if ed.fileTabs == nil {
		return nil
	}
	return ed.findTabByItem(ed.fileTabs.Selected())
}

func (ed *editor) hasActiveEditor() bool {
	return ed.activeFileTab() != nil && ed.entry != nil
}

func (ed *editor) refreshTabTitle(tab *fileTab) {
	if tab == nil || tab.item == nil || ed.fileTabs == nil {
		return
	}
	tab.item.Text = fileTabTitleWithPreview(tab.path, tab.modified, tab.preview)
	ed.fileTabs.Refresh()
}

func (ed *editor) pinTab(tab *fileTab) {
	if tab == nil || !tab.preview {
		return
	}
	tab.preview = false
	ed.refreshTabTitle(tab)
}

func (ed *editor) findPreviewTab() *fileTab {
	return findPreviewTab(ed.tabs)
}

func (ed *editor) markActiveModified() {
	tab := ed.activeFileTab()
	if tab == nil {
		return
	}
	if tab.preview {
		tab.preview = false
	}
	tab.modified = true
	ed.modified = true
	ed.refreshTabTitle(tab)
	ed.updateTitle()
	ed.syncGoplsDocument()
}

func (ed *editor) setActiveModified(modified bool) {
	tab := ed.activeFileTab()
	if tab != nil {
		tab.modified = modified
		ed.refreshTabTitle(tab)
	}
	ed.modified = modified
	ed.updateTitle()
}

func (ed *editor) applyActiveTab(tab *fileTab) {
	if tab == nil {
		ed.entry = nil
		ed.filePath = ""
		ed.modified = false
		ed.updateTitle()
		return
	}
	ed.entry = tab.entry
	ed.filePath = tab.path
	ed.modified = tab.modified
	ed.updateTitle()
	ed.syncGoplsDocument()
	if ed.window != nil {
		ed.window.Canvas().Focus(tab.entry)
	}
}

func (ed *editor) activateTab(tab *fileTab) {
	if tab == nil || tab.item == nil || ed.fileTabs == nil {
		return
	}
	ed.fileTabs.Select(tab.item)
	ed.applyActiveTab(tab)
}

func (ed *editor) onFileTabSelected(item *container.TabItem) {
	ed.applyActiveTab(ed.findTabByItem(item))
}

func (ed *editor) onFileTabCloseIntercept(item *container.TabItem) {
	tab := ed.findTabByItem(item)
	if tab == nil {
		ed.fileTabs.Remove(item)
		ed.applyActiveTab(ed.activeFileTab())
		return
	}
	if !tab.modified {
		ed.closeFileTab(tab)
		return
	}
	dialog.ShowConfirm(
		"Alterações não salvas",
		"Deseja descartar as alterações?",
		func(ok bool) {
			if ok {
				ed.closeFileTab(tab)
			}
		},
		ed.window,
	)
}

func (ed *editor) closeFileTab(tab *fileTab) {
	if tab == nil {
		return
	}
	if tab.path != "" && strings.HasSuffix(tab.path, ".go") {
		_ = ed.gopls.closeDocument(tab.path)
	}
	for i, t := range ed.tabs {
		if t == tab {
			ed.tabs = append(ed.tabs[:i], ed.tabs[i+1:]...)
			break
		}
	}
	if tab.item != nil && ed.fileTabs != nil {
		ed.fileTabs.Remove(tab.item)
	}
	ed.applyActiveTab(ed.activeFileTab())
}

func (ed *editor) closeActiveTab() {
	tab := ed.activeFileTab()
	if tab == nil {
		return
	}
	ed.onFileTabCloseIntercept(tab.item)
}

func (ed *editor) anyDirtyTabs() bool {
	for _, t := range ed.tabs {
		if t != nil && t.modified {
			return true
		}
	}
	return false
}

// openOrFocusFile foca a aba existente ou cria uma permanente com o conteúdo do disco.
func (ed *editor) openOrFocusFile(path string) *fileTab {
	return ed.openOrFocusFileMode(path, false)
}

// openOrFocusFilePreview abre em aba temporária (substitui a preview anterior, se houver).
func (ed *editor) openOrFocusFilePreview(path string) *fileTab {
	return ed.openOrFocusFileMode(path, true)
}

func (ed *editor) openOrFocusFileMode(path string, preview bool) *fileTab {
	path = normalizePath(path)
	if path == "" {
		return nil
	}
	if tab := ed.findTabByPath(path); tab != nil {
		if !preview {
			ed.pinTab(tab)
		}
		ed.activateTab(tab)
		return tab
	}

	data, err := os.ReadFile(path)
	if err != nil {
		dialog.ShowError(err, ed.window)
		return nil
	}
	content := string(data)

	if preview {
		if existing := ed.findPreviewTab(); existing != nil {
			if existing.modified {
				ed.pinTab(existing)
			} else {
				ed.loadPathIntoTab(existing, path, content)
				existing.preview = true
				ed.refreshTabTitle(existing)
				ed.activateTab(existing)
				ed.afterOpenFile(path)
				return existing
			}
		}
	}

	tab := ed.createFileTab(path, content)
	tab.preview = preview
	ed.refreshTabTitle(tab)
	ed.fileTabs.Append(tab.item)
	ed.activateTab(tab)
	ed.afterOpenFile(path)
	return tab
}

func (ed *editor) loadPathIntoTab(tab *fileTab, path, content string) {
	if tab == nil || tab.entry == nil {
		return
	}
	path = normalizePath(path)
	old := tab.path
	if old != "" && strings.HasSuffix(old, ".go") && !samePath(old, path) {
		_ = ed.gopls.closeDocument(old)
	}
	tab.path = path
	tab.modified = false
	tab.entry.SetText(content)
	tab.entry.SetDiagnostics(nil)
	tab.entry.SetCursor(0, 0)
	ed.refreshTabTitle(tab)
}

func (ed *editor) afterOpenFile(path string) {
	if ed.rootPath == "" {
		ed.rootPath = filepath.Dir(path)
	}
	ed.ensureGopls()
	ed.syncGoplsDocument()
}

func (ed *editor) reloadTab(tab *fileTab) {
	if tab == nil || tab.path == "" {
		return
	}
	data, err := os.ReadFile(tab.path)
	if err != nil {
		dialog.ShowError(err, ed.window)
		return
	}
	row, col := tab.entry.CursorRow(), tab.entry.CursorCol()
	tab.entry.SetText(string(data))
	tab.entry.SetDiagnostics(nil)
	tab.entry.SetCursor(row, col)
	tab.modified = false
	ed.refreshTabTitle(tab)
	if tab == ed.activeFileTab() {
		ed.filePath = tab.path
		ed.modified = false
		ed.updateTitle()
		ed.syncGoplsDocument()
	}
}

func (ed *editor) updateActiveTabPath(path string) {
	path = normalizePath(path)
	tab := ed.activeFileTab()
	if tab == nil {
		ed.filePath = path
		return
	}
	tab.modified = false
	ed.pinTab(tab)
	ed.updateTabPath(tab, path)
}

func (ed *editor) updateTabPath(tab *fileTab, newPath string) {
	if tab == nil {
		return
	}
	newPath = normalizePath(newPath)
	old := tab.path
	if old != "" && strings.HasSuffix(old, ".go") && !samePath(old, newPath) {
		_ = ed.gopls.closeDocument(old)
	}
	tab.path = newPath
	ed.refreshTabTitle(tab)
	if tab == ed.activeFileTab() {
		ed.filePath = newPath
		ed.modified = tab.modified
		ed.updateTitle()
		if strings.HasSuffix(newPath, ".go") {
			ed.syncGoplsDocument()
		}
	}
}

// onExplorerPathDeleted fecha abas cujo caminho foi removido (arquivo ou pasta).
func (ed *editor) onExplorerPathDeleted(path string) {
	path = normalizePath(path)
	if path == "" {
		return
	}
	var toClose []*fileTab
	for _, t := range ed.tabs {
		if t != nil && t.path != "" && pathUnderOrEqual(path, t.path) {
			toClose = append(toClose, t)
		}
	}
	for _, t := range toClose {
		ed.closeFileTab(t)
	}
}

// onExplorerPathRenamed atualiza caminhos de abas afetadas por rename no explorador.
func (ed *editor) onExplorerPathRenamed(oldPath, newPath string) {
	oldPath = normalizePath(oldPath)
	newPath = normalizePath(newPath)
	if oldPath == "" || newPath == "" {
		return
	}
	for _, t := range ed.tabs {
		if t == nil || t.path == "" {
			continue
		}
		if !pathUnderOrEqual(oldPath, t.path) {
			continue
		}
		ed.updateTabPath(t, rewritePathPrefix(t.path, oldPath, newPath))
	}
}
