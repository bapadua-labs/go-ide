package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2/dialog"
)

func formatGoSource(src, goroot, srcdir string) (string, error) {
	bin, err := resolveToolBinary(goroot, "goimports")
	if err != nil {
		return "", err
	}

	args := []string{}
	if srcdir != "" {
		args = append(args, "-srcdir", srcdir)
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(src)
	cmd.Env = withGoRootEnv(os.Environ(), goroot)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := strings.TrimSpace(string(exitErr.Stderr))
			if msg != "" {
				return "", fmt.Errorf("goimports: %s", msg)
			}
		}
		return "", err
	}
	return string(out), nil
}

func (ed *editor) formatDocument() {
	if ed.filePath != "" && !strings.HasSuffix(ed.filePath, ".go") {
		dialog.ShowInformation("Formatar", "Apenas arquivos .go podem ser formatados.", ed.window)
		return
	}

	srcdir := ed.rootPath
	if ed.filePath != "" {
		srcdir = filepath.Dir(ed.filePath)
	}

	src := ed.entry.Text()
	formatted, err := formatGoSource(src, ed.goPath(), srcdir)
	if err != nil {
		dialog.ShowError(err, ed.window)
		return
	}
	if formatted == src {
		return
	}

	ed.entry.SetText(formatted)
	ed.modified = true
	ed.updateTitle()
}
