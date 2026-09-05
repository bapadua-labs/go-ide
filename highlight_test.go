package main

import "testing"

func kindAt(kinds map[int]syntaxKind, pos int) syntaxKind {
	if k, ok := kinds[pos]; ok {
		return k
	}
	return syntaxDefault
}

func TestGoSyntaxHighlight_function(t *testing.T) {
	text := "func Hello() {}"
	kinds := goSyntaxHighlight(text)

	if kindAt(kinds, 0) != syntaxKeyword {
		t.Fatalf("func deveria ser keyword")
	}
	if kindAt(kinds, 5) != syntaxFunction {
		t.Fatalf("Hello deveria ser function, got %v", kindAt(kinds, 5))
	}
}

func TestGoSyntaxHighlight_struct(t *testing.T) {
	text := "type User struct {}\nvar u User"
	kinds := goSyntaxHighlight(text)

	if kindAt(kinds, 5) != syntaxType {
		t.Fatalf("User deveria ser type/struct, got %v", kindAt(kinds, 5))
	}
	if kindAt(kinds, 26) != syntaxType {
		t.Fatalf("User reutilizado deveria ser type, got %v", kindAt(kinds, 26))
	}
}

func TestGoSyntaxHighlight_package(t *testing.T) {
	text := "package main\nimport \"fmt\""
	kinds := goSyntaxHighlight(text)

	if kindAt(kinds, 8) != syntaxPackage {
		t.Fatalf("main deveria ser package, got %v", kindAt(kinds, 8))
	}
	if kindAt(kinds, 22) != syntaxPackage {
		t.Fatalf("import path deveria ser package, got %v", kindAt(kinds, 22))
	}
}

func TestGoSyntaxHighlight_methodCall(t *testing.T) {
	text := "fmt.Println(x)"
	kinds := goSyntaxHighlight(text)

	if kindAt(kinds, 4) != syntaxFunction {
		t.Fatalf("Println deveria ser function, got %v", kindAt(kinds, 4))
	}
}

func TestGoSyntaxHighlight_ignoresComments(t *testing.T) {
	text := "// func Fake()\n/* type Bad struct */"
	kinds := goSyntaxHighlight(text)

	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			continue
		}
		if kindAt(kinds, i) != syntaxComment {
			t.Fatalf("pos %d deveria ser comment, got %v", i, kindAt(kinds, i))
		}
	}
}
