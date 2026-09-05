package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var skipDirs = map[string]bool{
	".git":     true,
	".cursor":  true,
	".specify": true,
	"vendor":   true,
}

type packageExplorer struct {
	root          string
	selected      string
	tree          *widget.Tree
	panel         fyne.CanvasObject
	window        fyne.Window
	onFileSelect  func(path string, permanent bool)
	onPathDeleted func(string)
	onPathRenamed func(oldPath, newPath string)
	selectingOnly bool
	lastClickUID  string
	lastClickAt   time.Time
}

// explorerTreeItem é o conteúdo de um nó da árvore; captura clique e clique direito.
type explorerTreeItem struct {
	widget.BaseWidget
	pe    *packageExplorer
	uid   string
	icon  *widget.Icon
	label *widget.Label
}

var _ fyne.Tappable = (*explorerTreeItem)(nil)
var _ fyne.SecondaryTappable = (*explorerTreeItem)(nil)

func newExplorerTreeItem(pe *packageExplorer) *explorerTreeItem {
	item := &explorerTreeItem{
		pe:    pe,
		icon:  widget.NewIcon(theme.DocumentIcon()),
		label: widget.NewLabel(""),
	}
	item.ExtendBaseWidget(item)
	return item
}

func (i *explorerTreeItem) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewHBox(i.icon, i.label))
}

func (i *explorerTreeItem) Tapped(_ *fyne.PointEvent) {
	if i.pe == nil || i.uid == "" {
		return
	}
	i.pe.handleItemTap(i.uid)
}

func (i *explorerTreeItem) TappedSecondary(ev *fyne.PointEvent) {
	if i.pe == nil || i.uid == "" {
		return
	}
	i.pe.showContextMenu(i.uid, ev)
}

func newPackageExplorer(win fyne.Window, onFileSelect func(string, bool)) *packageExplorer {
	pe := &packageExplorer{
		window:       win,
		onFileSelect: onFileSelect,
	}

	pe.tree = widget.NewTree(
		pe.childIDs,
		pe.isBranch,
		func(_ bool) fyne.CanvasObject {
			return newExplorerTreeItem(pe)
		},
		pe.updateNode,
	)
	pe.tree.OnSelected = func(uid widget.TreeNodeID) {
		pe.selected = uid
		if pe.selectingOnly {
			return
		}
		// Space / clique na área do nó (fora do conteúdo): abre preview.
		if uid == "" || pe.isBranch(uid) {
			return
		}
		pe.openSelectedFile(uid, false)
	}

	title := widget.NewLabelWithStyle("Explorador", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	actions := widget.NewToolbar(
		widget.NewToolbarAction(explorerNewFileIcon, pe.promptNewFile),
		widget.NewToolbarAction(explorerNewFolderIcon, pe.promptNewFolder),
	)
	header := container.NewBorder(nil, nil, title, actions)

	pe.panel = container.NewBorder(
		header,
		nil, nil, nil,
		pe.tree,
	)
	return pe
}

func (pe *packageExplorer) handleItemTap(uid string) {
	pe.selectNodeOnly(uid)
	if uid == "" || pe.isBranch(uid) {
		return
	}
	permanent := isExplorerDoubleClick(pe.lastClickUID, uid, pe.lastClickAt, time.Now(), explorerDoubleClickDelay())
	pe.lastClickUID = uid
	pe.lastClickAt = time.Now()
	pe.openSelectedFile(uid, permanent)
}

func (pe *packageExplorer) openSelectedFile(uid string, permanent bool) {
	if pe.onFileSelect == nil {
		return
	}
	pe.onFileSelect(uid, permanent)
}

func explorerDoubleClickDelay() time.Duration {
	if fyne.CurrentApp() != nil && fyne.CurrentApp().Driver() != nil {
		return fyne.CurrentApp().Driver().DoubleTapDelay()
	}
	return 300 * time.Millisecond
}

func isExplorerDoubleClick(prevUID, uid string, prevAt, now time.Time, delay time.Duration) bool {
	return uid != "" && uid == prevUID && now.Sub(prevAt) < delay
}

func (pe *packageExplorer) setRoot(path string) {
	pe.root = path
	pe.selected = ""
	pe.tree.UnselectAll()
	pe.tree.Refresh()
}

func (pe *packageExplorer) refresh() {
	pe.tree.Refresh()
}

func (pe *packageExplorer) selectNodeOnly(uid string) {
	pe.selectingOnly = true
	pe.tree.Select(uid)
	pe.selectingOnly = false
}

func (pe *packageExplorer) showContextMenu(uid string, ev *fyne.PointEvent) {
	if uid == "" || (pe.root != "" && samePath(uid, pe.root)) {
		return
	}
	pe.selectNodeOnly(uid)

	renameItem := fyne.NewMenuItemWithIcon("Renomear", theme.SearchReplaceIcon(), func() {
		pe.promptRename(uid)
	})
	deleteItem := fyne.NewMenuItemWithIcon("Excluir", theme.DeleteIcon(), func() {
		pe.promptDelete(uid)
	})
	menu := fyne.NewMenu("", renameItem, fyne.NewMenuItemSeparator(), deleteItem)

	canvas := pe.window.Canvas()
	if treeCanvas := fyne.CurrentApp().Driver().CanvasForObject(pe.tree); treeCanvas != nil {
		canvas = treeCanvas
	}
	widget.ShowPopUpMenuAtPosition(menu, canvas, ev.AbsolutePosition)
}

func (pe *packageExplorer) targetDir() (string, error) {
	if pe.root == "" {
		return "", fmt.Errorf("abra uma pasta de projeto primeiro")
	}
	if pe.selected == "" || pe.selected == pe.root {
		return pe.root, nil
	}
	if pe.isBranch(pe.selected) {
		return pe.selected, nil
	}
	return filepath.Dir(pe.selected), nil
}

func (pe *packageExplorer) promptNewFile() {
	pe.promptCreate("Novo arquivo Go", "main", false)
}

func (pe *packageExplorer) promptNewFolder() {
	pe.promptCreate("Nova pasta", "nova-pasta", true)
}

func (pe *packageExplorer) promptCreate(title, placeholder string, isDir bool) {
	parent, err := pe.targetDir()
	if err != nil {
		dialog.ShowInformation("Explorador", err.Error(), pe.window)
		return
	}

	entry := widget.NewEntry()
	entry.SetPlaceHolder(placeholder)
	label := "Nome"
	if !isDir {
		label = "Nome (sem .go)"
	}
	form := []*widget.FormItem{
		widget.NewFormItem(label, entry),
	}
	dialog.ShowForm(title, "Criar", "Cancelar", form, func(ok bool) {
		if !ok {
			return
		}
		name := strings.TrimSpace(entry.Text)
		if err := validateExplorerName(name); err != nil {
			dialog.ShowError(err, pe.window)
			return
		}
		if !isDir {
			name = ensureGoFileName(name)
		}
		fullPath := filepath.Join(parent, name)
		if _, err := os.Stat(fullPath); err == nil {
			dialog.ShowError(fmt.Errorf("%q já existe", name), pe.window)
			return
		} else if !os.IsNotExist(err) {
			dialog.ShowError(err, pe.window)
			return
		}

		if isDir {
			if err := os.Mkdir(fullPath, 0o755); err != nil {
				dialog.ShowError(err, pe.window)
				return
			}
		} else {
			f, err := os.OpenFile(fullPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
			if err != nil {
				dialog.ShowError(err, pe.window)
				return
			}
			_ = f.Close()
		}

		if parent != pe.root {
			pe.tree.OpenBranch(parent)
		}
		pe.refresh()
		pe.tree.Select(fullPath)
	}, pe.window)
}

func (pe *packageExplorer) promptRename(uid string) {
	if uid == "" || (pe.root != "" && samePath(uid, pe.root)) {
		return
	}
	if _, err := os.Stat(uid); err != nil {
		dialog.ShowError(err, pe.window)
		return
	}
	oldName := filepath.Base(uid)

	entry := widget.NewEntry()
	entry.SetText(oldName)
	form := []*widget.FormItem{
		widget.NewFormItem("Novo nome", entry),
	}
	dialog.ShowForm("Renomear", "Renomear", "Cancelar", form, func(ok bool) {
		if !ok {
			return
		}
		name := strings.TrimSpace(entry.Text)
		if err := validateExplorerName(name); err != nil {
			dialog.ShowError(err, pe.window)
			return
		}
		if name == oldName {
			return
		}
		newPath := filepath.Join(filepath.Dir(uid), name)
		if _, err := os.Stat(newPath); err == nil {
			dialog.ShowError(fmt.Errorf("%q já existe", name), pe.window)
			return
		} else if !os.IsNotExist(err) {
			dialog.ShowError(err, pe.window)
			return
		}
		if err := os.Rename(uid, newPath); err != nil {
			dialog.ShowError(err, pe.window)
			return
		}
		pe.refresh()
		if pe.onPathRenamed != nil {
			pe.onPathRenamed(uid, newPath)
		}
		parent := filepath.Dir(newPath)
		if parent != pe.root {
			pe.tree.OpenBranch(parent)
		}
		pe.selectNodeOnly(newPath)
	}, pe.window)
}

func (pe *packageExplorer) promptDelete(uid string) {
	if uid == "" || (pe.root != "" && samePath(uid, pe.root)) {
		return
	}
	info, err := os.Stat(uid)
	if err != nil {
		dialog.ShowError(err, pe.window)
		return
	}
	name := filepath.Base(uid)
	msg := fmt.Sprintf("Excluir permanentemente %q?", name)
	if info.IsDir() {
		msg = fmt.Sprintf("Excluir permanentemente a pasta %q e todo o conteúdo?", name)
	}
	dialog.ShowConfirm("Excluir", msg, func(ok bool) {
		if !ok {
			return
		}
		var removeErr error
		if info.IsDir() {
			removeErr = os.RemoveAll(uid)
		} else {
			removeErr = os.Remove(uid)
		}
		if removeErr != nil {
			dialog.ShowError(removeErr, pe.window)
			return
		}
		pe.selected = ""
		pe.tree.UnselectAll()
		pe.refresh()
		if pe.onPathDeleted != nil {
			pe.onPathDeleted(uid)
		}
	}, pe.window)
}

func validateExplorerName(name string) error {
	if name == "" {
		return fmt.Errorf("informe um nome")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("nome inválido")
	}
	if strings.ContainsAny(name, `/\`) || filepath.Base(name) != name {
		return fmt.Errorf("o nome não pode conter caminho")
	}
	return nil
}

// ensureGoFileName acrescenta .go se o usuário digitou só o nome.
func ensureGoFileName(name string) string {
	if strings.HasSuffix(strings.ToLower(name), ".go") {
		return name
	}
	return name + ".go"
}

func (pe *packageExplorer) childIDs(uid widget.TreeNodeID) []widget.TreeNodeID {
	path := uid
	if path == "" {
		if pe.root == "" {
			return nil
		}
		path = pe.root
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}

	var dirs, files []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && name != "go.mod" && name != "go.sum" {
			continue
		}
		if entry.IsDir() && skipDirs[name] {
			continue
		}

		fullPath := filepath.Join(path, name)
		if entry.IsDir() {
			dirs = append(dirs, fullPath)
		} else {
			files = append(files, fullPath)
		}
	}

	sort.Strings(dirs)
	sort.Strings(files)

	return append(dirs, files...)
}

func (pe *packageExplorer) isBranch(uid widget.TreeNodeID) bool {
	if uid == "" {
		return true
	}
	info, err := os.Stat(uid)
	return err == nil && info.IsDir()
}

func (pe *packageExplorer) updateNode(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
	item := obj.(*explorerTreeItem)
	item.uid = uid

	if uid == "" {
		item.icon.SetResource(theme.FolderIcon())
		if pe.root == "" {
			item.label.SetText("Nenhuma pasta aberta")
			return
		}
		item.label.SetText(filepath.Base(pe.root))
		return
	}

	name := filepath.Base(uid)
	if branch || pe.isBranch(uid) {
		item.icon.SetResource(theme.FolderIcon())
		item.label.SetText(name)
		return
	}

	if strings.HasSuffix(name, ".go") {
		item.icon.SetResource(theme.FileTextIcon())
	} else {
		item.icon.SetResource(theme.DocumentIcon())
	}
	item.label.SetText(name)
}
