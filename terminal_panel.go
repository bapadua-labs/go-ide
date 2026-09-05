package main

import (
	"os"
	"strings"

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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
