package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateGoRootAcceptsCurrentInstall(t *testing.T) {
	root := defaultGoPath()
	if err := validateGoRoot(root); err != nil {
		t.Fatalf("defaultGoPath %q inválido: %v", root, err)
	}
	if err := validateGoRoot(root + string(os.PathSeparator)); err != nil {
		t.Fatalf("GOROOT com barra final deveria ser aceito: %v", err)
	}
}

func TestWithGoRootEnvReplacesExisting(t *testing.T) {
	env := withGoRootEnv([]string{"PATH=/bin", "GOROOT=/old", "HOME=/tmp"}, "/usr/local/go")
	found := false
	for _, kv := range env {
		if kv == "GOROOT=/usr/local/go" {
			found = true
		}
		if kv == "GOROOT=/old" {
			t.Fatal("GOROOT antigo não foi substituído")
		}
	}
	if !found {
		t.Fatalf("GOROOT novo ausente em %#v", env)
	}
}

func TestGoBinaryInRootCleansTrailingSlash(t *testing.T) {
	root := runtime.GOROOT()
	got := goBinaryInRoot(root + string(filepath.Separator))
	want := filepath.Join(filepath.Clean(root), "bin", "go")
	if runtime.GOOS == "windows" {
		want += ".exe"
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
