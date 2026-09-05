package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	root     string
	selected string
	tree     *widget.Tree
	panel    fyne.CanvasObject
	window   fyne.Window
}

func newPackageExplorer(win fyne.Window, onFileSelect func(string)) *packageExplorer {
	pe := &packageExplorer{
		window: win,
	}

	pe.tree = widget.NewTree(
		pe.childIDs,
		pe.isBranch,
		func(_ bool) fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.DocumentIcon()),
				widget.NewLabel(""),
			)
		},
		pe.updateNode,
	)
	pe.tree.OnSelected = func(uid widget.TreeNodeID) {
		pe.selected = uid
		if uid == "" || pe.isBranch(uid) {
			return
		}
		onFileSelect(uid)
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

func (pe *packageExplorer) setRoot(path string) {
	pe.root = path
	pe.selected = ""
	pe.tree.UnselectAll()
	pe.tree.Refresh()
}

func (pe *packageExplorer) refresh() {
	pe.tree.Refresh()
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
	box := obj.(*fyne.Container)
	icon := box.Objects[0].(*widget.Icon)
	label := box.Objects[1].(*widget.Label)

	if uid == "" {
		icon.SetResource(theme.FolderIcon())
		if pe.root == "" {
			label.SetText("Nenhuma pasta aberta")
			return
		}
		label.SetText(filepath.Base(pe.root))
		return
	}

	name := filepath.Base(uid)
	if branch || pe.isBranch(uid) {
		icon.SetResource(theme.FolderIcon())
		label.SetText(name)
		return
	}

	if strings.HasSuffix(name, ".go") {
		icon.SetResource(theme.FileTextIcon())
	} else {
		icon.SetResource(theme.DocumentIcon())
	}
	label.SetText(name)
}
