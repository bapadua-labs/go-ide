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
			if msg == "" {
				msg = strings.TrimSpace(string(out))
			}
			if msg != "" {
				return "", fmt.Errorf("goimports: %s", msg)
			}
		}
		return "", err
	}
	return string(out), nil
}

// applyGoFormat formata o buffer ativo com goimports (imports + gofmt).
// Sem efeito se não houver editor ou o arquivo não for .go.
func (ed *editor) applyGoFormat() error {
	if !ed.hasActiveEditor() || ed.entry == nil {
		return nil
	}
	if ed.filePath != "" && !strings.HasSuffix(ed.filePath, ".go") {
		return nil
	}

	srcdir := ed.rootPath
	if ed.filePath != "" {
		srcdir = filepath.Dir(ed.filePath)
	}

	src := ed.entry.Text()
	formatted, err := formatGoSource(src, ed.goPath(), srcdir)
	if err != nil {
		return err
	}
	if formatted == src {
		return nil
	}

	row, col := ed.entry.CursorRow(), ed.entry.CursorCol()
	ed.entry.SetText(formatted)
	ed.entry.SetCursor(row, col)
	ed.setActiveModified(true)
	ed.syncGoplsDocument()
	return nil
}

func (ed *editor) formatDocument() {
	if !ed.hasActiveEditor() {
		return
	}
	if ed.filePath != "" && !strings.HasSuffix(ed.filePath, ".go") {
		dialog.ShowInformation("Formatar", "Apenas arquivos .go podem ser formatados.", ed.window)
		return
	}
	if err := ed.applyGoFormat(); err != nil {
		dialog.ShowError(err, ed.window)
	}
}
