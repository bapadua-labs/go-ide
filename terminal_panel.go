package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/fyne-io/terminal"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
)

type termPanel struct {
	window     fyne.Window
	tabs       *container.DocTabs
	panel      fyne.CanvasObject
	workingDir string
	onChange   func(int)
}

func newTermPanel(w fyne.Window, startDir string, onChange func(int)) *termPanel {
	tp := &termPanel{
		window:     w,
		workingDir: startDir,
		onChange:   onChange,
	}
	tp.tabs = container.NewDocTabs()
	tp.tabs.CreateTab = func() *container.TabItem {
		tab := tp.createTab(tp.workingDir)
		fyne.Do(tp.notifyChange)
		return tab
	}
	tp.tabs.OnClosed = func(_ *container.TabItem) {
		tp.notifyChange()
	}

	tp.panel = tp.tabs
	tp.setupShortcuts()
	return tp
}

func (tp *termPanel) setupShortcuts() {
	tp.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName:  fyne.KeyT,
		Modifier: fyne.KeyModifierControl | fyne.KeyModifierShift,
	}, func(_ fyne.Shortcut) {
		tp.newTab()
	})
}

func (tp *termPanel) newTab() {
	tab := tp.createTab(tp.workingDir)
	tp.tabs.Append(tab)
	tp.tabs.Select(tab)
	tp.window.Canvas().Focus(tp.terminalFromTab(tab))
	tp.notifyChange()
}

func (tp *termPanel) tabCount() int {
	return len(tp.tabs.Items)
}

func (tp *termPanel) notifyChange() {
	if tp.onChange != nil {
		tp.onChange(tp.tabCount())
	}
}

func (tp *termPanel) createTab(startDir string) *container.TabItem {
	t := terminal.New()
	dir := startDir
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if dir != "" {
		t.SetStartDir(dir)
	}

	tab := container.NewTabItem("Terminal", t)

	listen := make(chan terminal.Config)
	go func() {
		for config := range listen {
			fyne.Do(func() {
				if config.Title == "" {
					tab.Text = "Terminal"
				} else {
					tab.Text = config.Title
				}
				tp.tabs.Refresh()
			})
		}
	}()
	t.AddListener(listen)

	go func() {
		_ = t.RunLocalShell()
		fyne.Do(func() {
			tp.tabs.Remove(tab)
			tp.notifyChange()
		})
	}()

	return tab
}

func (tp *termPanel) setWorkingDir(dir string) {
	if dir == "" {
		return
	}
	tp.workingDir = dir

	t := tp.activeTerminal()
	if t == nil {
		return
	}
	t.SetStartDir(dir)
	_, _ = t.Write([]byte("cd " + shellQuote(dir) + "\n"))
}

func (tp *termPanel) activeTerminal() *terminal.Terminal {
	if tp.tabs.Selected() == nil {
		return nil
	}
	return tp.terminalFromTab(tp.tabs.Selected())
}

func (tp *termPanel) terminalFromTab(tab *container.TabItem) *terminal.Terminal {
	if tab == nil {
		return nil
	}
	term, _ := tab.Content.(*terminal.Terminal)
	return term
}

func (tp *termPanel) runCommand(command string) {
	tp.whenReady(func(t *terminal.Terminal) {
		if !strings.HasSuffix(command, "\n") {
			command += "\n"
		}
		_, _ = t.Write([]byte(command))
	})
}

func (tp *termPanel) runGoFile(goBin, dir, file string) {
	tp.whenReady(func(t *terminal.Terminal) {
		go func() {
			cmd := exec.Command(goBin, "run", file)
			cmd.Dir = dir
			out, err := cmd.CombinedOutput()
			text := string(out)
			if err != nil && text == "" {
				text = err.Error() + "\n"
			}
			tp.displayOutput(t, text)
		}()
	})
}

func (tp *termPanel) whenReady(fn func(*terminal.Terminal)) {
	if tp.tabCount() == 0 {
		tp.newTab()
	}
	t := tp.activeTerminal()
	if t == nil {
		return
	}
	go func() {
		for range 200 {
			if _, err := t.Write([]byte("")); err == nil {
				fyne.Do(func() { fn(t) })
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
}

func (tp *termPanel) displayOutput(t *terminal.Terminal, output string) {
	var script string
	if runtime.GOOS == "windows" {
		escaped := strings.ReplaceAll(output, "'", "''")
		script = fmt.Sprintf("Clear-Host; Write-Output '%s'\n", escaped)
	} else {
		encoded := base64.StdEncoding.EncodeToString([]byte(output))
		// Comando de uma linha evita prompts PS2 (>) do heredoc em shell interativo.
		script = fmt.Sprintf(
			"stty -echo 2>/dev/null; clear; printf '%%s' '%s' | base64 -d; stty echo 2>/dev/null\n",
			encoded,
		)
	}

	payload := []byte(script)
	for range 20 {
		if _, err := t.Write(payload); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
