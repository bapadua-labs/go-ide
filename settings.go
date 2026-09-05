package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

const prefGoPath = "goPath"

func defaultGoPath() string {
	return runtime.GOROOT()
}

func (ed *editor) goPath() string {
	return ed.app.Preferences().StringWithFallback(prefGoPath, defaultGoPath())
}

func (ed *editor) setGoPath(path string) {
	ed.app.Preferences().SetString(prefGoPath, path)
}

func goBinaryInRoot(root string) string {
	return toolBinaryInRoot(root, "go")
}

func resolveGoBinary(goroot string) (string, error) {
	name := "go"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if goroot != "" {
		path := goBinaryInRoot(goroot)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf(
		"go não encontrado; confira o GOROOT em Propriedades ou o PATH do sistema",
	)
}

func resolveToolBinary(goroot, tool string) (string, error) {
	name := tool
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	for _, dir := range goToolBinDirs(goroot) {
		path := filepath.Join(dir, name)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf(
		"%s não encontrado; instale com: go install golang.org/x/tools/cmd/%s@latest",
		tool, tool,
	)
}

func goToolBinDirs(goroot string) []string {
	var dirs []string
	if goroot != "" {
		dirs = append(dirs, filepath.Join(goroot, "bin"))
	}
	if gobin := strings.TrimSpace(goEnvValue(goroot, "GOBIN")); gobin != "" {
		dirs = append(dirs, gobin)
	}
	for _, root := range filepath.SplitList(strings.TrimSpace(goEnvValue(goroot, "GOPATH"))) {
		if root != "" {
			dirs = append(dirs, filepath.Join(root, "bin"))
		}
	}
	return dirs
}

func goEnvValue(goroot, key string) string {
	if goroot == "" {
		return ""
	}
	out, err := exec.Command(goBinaryInRoot(goroot), "env", key).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func toolBinaryInRoot(root, tool string) string {
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	return filepath.Join(root, "bin", tool)
}

func validateGoRoot(path string) error {
	if path == "" {
		return fmt.Errorf("informe o caminho da instalação do Go")
	}
	info, err := os.Stat(goBinaryInRoot(path))
	if err != nil {
		return fmt.Errorf("não foi encontrado o executável go em %s", goBinaryInRoot(path))
	}
	if info.IsDir() {
		return fmt.Errorf("o caminho do executável go é um diretório")
	}
	return nil
}

func (ed *editor) showProperties() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(defaultGoPath())
	entry.SetText(ed.goPath())
	entry.Validator = validateGoRoot

	browse := widget.NewButton("...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil || uri == nil {
				return
			}
			entry.SetText(uri.Path())
		}, ed.window)
	})

	form := []*widget.FormItem{
		widget.NewFormItem("Caminho do Go (GOROOT)", container.NewBorder(nil, nil, nil, browse, entry)),
	}

	dialog.ShowForm("Propriedades", "Salvar", "Cancelar", form, func(ok bool) {
		if !ok {
			return
		}
		ed.setGoPath(entry.Text)
		ed.restartGopls()
	}, ed.window)
}
