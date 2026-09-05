package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/fyne-io/terminal"
)

type termPanel struct {
	window     fyne.Window
	tabs       *container.DocTabs
	panel      fyne.CanvasObject
	workingDir string
	onChange   func(int)

	readyMu sync.Mutex
	ready   map[*terminal.Terminal]chan struct{}
}

func newTermPanel(w fyne.Window, startDir string, onChange func(int)) *termPanel {
	tp := &termPanel{
		window:     w,
		workingDir: startDir,
		onChange:   onChange,
		ready:      make(map[*terminal.Terminal]chan struct{}),
	}
	tp.tabs = container.NewDocTabs()
	tp.tabs.CreateTab = func() *container.TabItem {
		tab := tp.createTab(tp.workingDir)
		fyne.Do(tp.notifyChange)
		return tab
	}
	tp.tabs.OnClosed = func(tab *container.TabItem) {
		if t := tp.terminalFromTab(tab); t != nil {
			tp.clearReady(t)
		}
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
	tp.focusTerminal(tp.terminalFromTab(tab))
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
	tp.armReady(t)

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
			tp.clearReady(t)
			tp.tabs.Remove(tab)
			tp.notifyChange()
		})
	}()

	return tab
}

func (tp *termPanel) armReady(t *terminal.Terminal) {
	ch := make(chan struct{})
	var once sync.Once
	t.SetReadWriter(terminal.ReadWriterConfiguratorFunc(
		func(r io.Reader, w io.WriteCloser) (io.Reader, io.WriteCloser) {
			once.Do(func() { close(ch) })
			return r, w
		},
	))

	tp.readyMu.Lock()
	tp.ready[t] = ch
	tp.readyMu.Unlock()
}

func (tp *termPanel) clearReady(t *terminal.Terminal) {
	tp.readyMu.Lock()
	delete(tp.ready, t)
	tp.readyMu.Unlock()
}

func (tp *termPanel) waitReady(t *terminal.Terminal, timeout time.Duration) bool {
	tp.readyMu.Lock()
	ch := tp.ready[t]
	tp.readyMu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	case <-time.After(timeout):
		return false
	}
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
	go func() {
		if !tp.waitReady(t, 10*time.Second) {
			return
		}
		fyne.Do(func() {
			_, _ = t.Write([]byte("cd " + shellQuote(dir) + "\n"))
		})
	}()
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

func (tp *termPanel) focusActive() {
	tp.focusTerminal(tp.activeTerminal())
}

func (tp *termPanel) focusTerminal(t *terminal.Terminal) {
	if t == nil || tp.window == nil {
		return
	}
	tp.window.Canvas().Focus(t)
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
		// O fyne-io/terminal inicia com discardWriter (Write sempre "sucesso").
		// Só consideramos pronto quando SetReadWriter roda no open() do PTY real.
		if !tp.waitReady(t, 10*time.Second) {
			return
		}
		// Pequena folga para o shell imprimir o prompt inicial.
		time.Sleep(100 * time.Millisecond)
		fyne.Do(func() { fn(t) })
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
