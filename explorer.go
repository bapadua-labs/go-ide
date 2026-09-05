package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var skipDirs = map[string]bool{
	".git":     true,
	".cursor":  true,
	".specify": true,
	"vendor":   true,
}

type packageExplorer struct {
	root  string
	tree  *widget.Tree
	panel fyne.CanvasObject
}

func newPackageExplorer(onFileSelect func(string)) *packageExplorer {
	pe := &packageExplorer{}

	pe.tree = widget.NewTree(
		pe.childIDs,
		pe.isBranch,
		func(_ bool) fyne.CanvasObject {
			return widget.NewLabel("")
		},
		pe.updateNode,
	)
	pe.tree.OnSelected = func(uid widget.TreeNodeID) {
		if uid == "" || pe.isBranch(uid) {
			return
		}
		onFileSelect(uid)
	}

	header := widget.NewLabelWithStyle("Explorador", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	pe.panel = container.NewBorder(
		header,
		nil, nil, nil,
		container.NewScroll(pe.tree),
	)
	return pe
}

func (pe *packageExplorer) setRoot(path string) {
	pe.root = path
	pe.tree.Refresh()
	if pe.root != "" {
		pe.tree.OpenBranch(pe.root)
	}
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

func (pe *packageExplorer) updateNode(uid widget.TreeNodeID, _ bool, obj fyne.CanvasObject) {
	label := obj.(*widget.Label)
	if uid == "" {
		if pe.root == "" {
			label.SetText("Nenhuma pasta aberta")
			return
		}
		label.SetText(filepath.Base(pe.root))
		return
	}

	name := filepath.Base(uid)
	if pe.isBranch(uid) {
		label.SetText("📁 " + name)
		return
	}

	if strings.HasSuffix(name, ".go") {
		label.SetText("🐹 " + name)
		return
	}
	label.SetText("📄 " + name)
}
