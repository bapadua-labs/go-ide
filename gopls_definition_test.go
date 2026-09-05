package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoplsDefinition(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mainGo := filepath.Join(root, "main.go")
	data, err := os.ReadFile(mainGo)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	// Find byte offset of "editor" type in "type editor struct"
	row, col := 0, 0
	found := false
	for i, line := range splitLines(text) {
		if idx := indexOf(line, "type editor struct"); idx >= 0 {
			row = i
			col = idx + len("type ")
			found = true
			break
		}
	}
	if !found {
		t.Fatal("could not find editor struct in main.go")
	}

	g := newGoplsClient()
	goroot := defaultGoPath()
	if err := g.start(goroot, root); err != nil {
		t.Fatalf("gopls start: %v", err)
	}
	defer g.stop()

	if err := g.openDocument(mainGo, text); err != nil {
		t.Fatalf("open doc: %v", err)
	}

	result, err := g.definition(mainGo, text, row, col)
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	locs := locationsFromDefinition(result)
	if len(locs) == 0 {
		t.Fatalf("no locations for editor struct, result type %T", result)
	}
	t.Logf("editor struct -> %s:%d:%d", locs[0].Path, locs[0].Row+1, locs[0].Col+1)

	// imported symbol
	row2, col2 := 0, 0
	found2 := false
	for i, line := range splitLines(text) {
		if idx := indexOf(line, "app.NewWithID"); idx >= 0 {
			row2 = i
			col2 = idx + len("app.")
			found2 = true
			break
		}
	}
	if !found2 {
		t.Fatal("app.NewWithID not found")
	}
	result2, err := g.definition(mainGo, text, row2, col2)
	if err != nil {
		t.Fatalf("definition app: %v", err)
	}
	locs2 := locationsFromDefinition(result2)
	if len(locs2) == 0 {
		t.Fatalf("no locations for app.NewWithID, result type %T", result2)
	}
	t.Logf("app.NewWithID -> %s:%d:%d", locs2[0].Path, locs2[0].Row+1, locs2[0].Col+1)
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
