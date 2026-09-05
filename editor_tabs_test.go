package main

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/container"
)

func TestFileTabTitle(t *testing.T) {
	if got := fileTabTitle("", false); got != "Sem título" {
		t.Fatalf("untitled: got %q", got)
	}
	if got := fileTabTitle("", true); got != "Sem título *" {
		t.Fatalf("untitled dirty: got %q", got)
	}
	if got := fileTabTitle("/tmp/proj/main.go", false); got != "main.go" {
		t.Fatalf("clean: got %q", got)
	}
	if got := fileTabTitle("/tmp/proj/main.go", true); got != "main.go *" {
		t.Fatalf("dirty: got %q", got)
	}
}

func TestFindFileTabByPath(t *testing.T) {
	a := filepath.Join(t.TempDir(), "a.go")
	b := filepath.Join(t.TempDir(), "b.go")
	tabs := []*fileTab{
		{path: normalizePath(a)},
		{path: normalizePath(b)},
		{path: ""},
	}

	if findFileTabByPath(tabs, a) != tabs[0] {
		t.Fatal("expected to find tab a")
	}
	if findFileTabByPath(tabs, b) != tabs[1] {
		t.Fatal("expected to find tab b")
	}
	if findFileTabByPath(tabs, filepath.Join(t.TempDir(), "c.go")) != nil {
		t.Fatal("expected nil for unknown path")
	}
	if findFileTabByPath(tabs, "") != nil {
		t.Fatal("empty path should not match untitled")
	}
}

func TestFindFileTabByItem(t *testing.T) {
	item1 := container.NewTabItem("a", nil)
	item2 := container.NewTabItem("b", nil)
	tabs := []*fileTab{
		{item: item1},
		{item: item2},
	}
	if findFileTabByItem(tabs, item1) != tabs[0] {
		t.Fatal("expected tab for item1")
	}
	if findFileTabByItem(tabs, item2) != tabs[1] {
		t.Fatal("expected tab for item2")
	}
	if findFileTabByItem(tabs, nil) != nil {
		t.Fatal("nil item")
	}
	if findFileTabByItem(tabs, container.NewTabItem("x", nil)) != nil {
		t.Fatal("unknown item")
	}
}
