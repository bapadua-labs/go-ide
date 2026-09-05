package main

import (
	"path/filepath"
	"testing"
)

func TestValidateExplorerName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"main", false},
		{"arquivo.go", false},
		{"nova-pasta", false},
		{"", true},
		{".", true},
		{"..", true},
		{"a/b", true},
		{`a\b`, true},
	}
	for _, tt := range tests {
		err := validateExplorerName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateExplorerName(%q) err=%v wantErr=%v", tt.name, err, tt.wantErr)
		}
	}
}

func TestEnsureGoFileName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"main", "main.go"},
		{"handler", "handler.go"},
		{"main.go", "main.go"},
		{"Main.GO", "Main.GO"},
		{"util_test.go", "util_test.go"},
	}
	for _, tt := range tests {
		if got := ensureGoFileName(tt.in); got != tt.want {
			t.Errorf("ensureGoFileName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPathUnderOrEqual(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	sub := filepath.Join(root, "pkg")
	subFile := filepath.Join(sub, "b.go")
	other := filepath.Join(t.TempDir(), "x.go")

	if !pathUnderOrEqual(root, file) {
		t.Errorf("file should be under root")
	}
	if !pathUnderOrEqual(root, root) {
		t.Errorf("root should be under itself")
	}
	if !pathUnderOrEqual(sub, subFile) {
		t.Errorf("subFile should be under sub")
	}
	if pathUnderOrEqual(sub, file) {
		t.Errorf("sibling file should not be under sub")
	}
	if pathUnderOrEqual(root, other) {
		t.Errorf("other tree should not match")
	}
}

func TestRewritePathPrefix(t *testing.T) {
	root := t.TempDir()
	oldDir := filepath.Join(root, "old")
	newDir := filepath.Join(root, "new")
	oldFile := filepath.Join(oldDir, "a.go")

	if got := rewritePathPrefix(oldDir, oldDir, newDir); got != newDir {
		t.Errorf("dir rename: got %q want %q", got, newDir)
	}
	wantFile := filepath.Join(newDir, "a.go")
	if got := rewritePathPrefix(oldFile, oldDir, newDir); got != wantFile {
		t.Errorf("nested file: got %q want %q", got, wantFile)
	}
	if got := rewritePathPrefix(oldFile, oldFile, filepath.Join(root, "b.go")); got != filepath.Join(root, "b.go") {
		t.Errorf("file rename: got %q", got)
	}
}
