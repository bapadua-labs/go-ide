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

// defaultGoPath descobre o GOROOT atual do ambiente (env, `go env`, PATH, runtime).
func defaultGoPath() string {
	candidates := []string{
		strings.TrimSpace(os.Getenv("GOROOT")),
	}
	if out, err := exec.Command("go", "env", "GOROOT").Output(); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(out)))
	}
	if bin, err := exec.LookPath("go"); err == nil {
		if resolved, err := filepath.EvalSymlinks(bin); err == nil {
			bin = resolved
		}
		candidates = append(candidates, filepath.Dir(filepath.Dir(bin)))
	}
	candidates = append(candidates, runtime.GOROOT())

	for _, root := range candidates {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || root == "." {
			continue
		}
		if validateGoRoot(root) == nil {
			return root
		}
	}
	return filepath.Clean(runtime.GOROOT())
}

func (ed *editor) goPath() string {
	stored := strings.TrimSpace(ed.app.Preferences().String(prefGoPath))
	if stored != "" {
		stored = filepath.Clean(stored)
		if validateGoRoot(stored) == nil {
			return stored
		}
	}
	return defaultGoPath()
}

// ensureGoRoot corrige preferência antiga/inválida para o GOROOT detectado agora.
func (ed *editor) ensureGoRoot() {
	stored := strings.TrimSpace(ed.app.Preferences().String(prefGoPath))
	discovered := defaultGoPath()
	if discovered == "" {
		return
	}
	if stored != "" {
		cleaned := filepath.Clean(stored)
		if validateGoRoot(cleaned) == nil {
			if cleaned != stored {
				ed.setGoPath(cleaned)
			}
			return
		}
	}
	ed.setGoPath(discovered)
}

func (ed *editor) setGoPath(path string) {
	ed.app.Preferences().SetString(prefGoPath, filepath.Clean(strings.TrimSpace(path)))
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
		path := goBinaryInRoot(filepath.Clean(goroot))
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
	goroot = filepath.Clean(strings.TrimSpace(goroot))
	if goroot != "" && goroot != "." {
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
	goBin, err := resolveGoBinary(goroot)
	if err != nil {
		return ""
	}
	out, err := exec.Command(goBin, "env", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func toolBinaryInRoot(root, tool string) string {
	if runtime.GOOS == "windows" {
		tool += ".exe"
	}
	return filepath.Join(filepath.Clean(root), "bin", tool)
}

func validateGoRoot(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." {
		return fmt.Errorf("informe o caminho da instalação do Go")
	}
	bin := goBinaryInRoot(path)
	info, err := os.Stat(bin)
	if err != nil {
		return fmt.Errorf("não foi encontrado o executável go em %s", bin)
	}
	if info.IsDir() {
		return fmt.Errorf("o caminho do executável go é um diretório")
	}
	return nil
}

// withGoRootEnv garante GOROOT e PATH com $GOROOT/bin para processos filhos
// (gopls, go run, goimports precisam achar o binário `go`).
func withGoRootEnv(base []string, goroot string) []string {
	goroot = filepath.Clean(strings.TrimSpace(goroot))
	if goroot == "" || goroot == "." {
		return base
	}
	binDir := filepath.Join(goroot, "bin")
	prefixGoRoot := "GOROOT="
	prefixPath := "PATH="

	out := make([]string, 0, len(base)+2)
	replacedGoRoot := false
	replacedPath := false
	for _, kv := range base {
		switch {
		case strings.HasPrefix(kv, prefixGoRoot):
			out = append(out, prefixGoRoot+goroot)
			replacedGoRoot = true
		case strings.HasPrefix(kv, prefixPath):
			pathVal := kv[len(prefixPath):]
			out = append(out, prefixPath+prependPathDir(pathVal, binDir))
			replacedPath = true
		default:
			out = append(out, kv)
		}
	}
	if !replacedGoRoot {
		out = append(out, prefixGoRoot+goroot)
	}
	if !replacedPath {
		out = append(out, prefixPath+binDir)
	}
	return out
}

func prependPathDir(pathVal, dir string) string {
	dir = filepath.Clean(dir)
	if dir == "" || dir == "." {
		return pathVal
	}
	if pathVal == "" {
		return dir
	}
	parts := filepath.SplitList(pathVal)
	filtered := make([]string, 0, len(parts)+1)
	filtered = append(filtered, dir)
	for _, p := range parts {
		if p == "" || filepath.Clean(p) == dir {
			continue
		}
		filtered = append(filtered, p)
	}
	return strings.Join(filtered, string(os.PathListSeparator))
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
			entry.SetText(filepath.Clean(uri.Path()))
		}, ed.window)
	})

	detect := widget.NewButton("Detectar", func() {
		entry.SetText(defaultGoPath())
	})

	form := []*widget.FormItem{
		widget.NewFormItem(
			"Caminho do Go (GOROOT)",
			container.NewBorder(nil, nil, nil, container.NewHBox(detect, browse), entry),
		),
	}

	dialog.ShowForm("Propriedades", "Salvar", "Cancelar", form, func(ok bool) {
		if !ok {
			return
		}
		ed.setGoPath(entry.Text)
		ed.restartGopls()
	}, ed.window)
}
