package main

import "testing"

func TestPlainTextFromMarkdown(t *testing.T) {
	input := "```go\nfunc heyBobo() {\n}\n```\n\n---\n\n// funcao que diz que voce e um bobao"
	got := plainTextFromMarkdown(input)
	want := "func heyBobo() {\n}\n\n// funcao que diz que voce e um bobao"
	if got != want {
		t.Fatalf("plainTextFromMarkdown() = %q, want %q", got, want)
	}
}

func TestExpandSnippet(t *testing.T) {
	got := expandSnippet("func ${1:name}() {\n\t$0\n}")
	want := "func name() {\n\t\n}"
	if got != want {
		t.Fatalf("expandSnippet() = %q, want %q", got, want)
	}
}

func TestWrapPlainText(t *testing.T) {
	got := wrapPlainText("one two three four five six seven", 10)
	if got != "one two\nthree four\nfive six\nseven" {
		t.Fatalf("wrapPlainText() = %q", got)
	}
}
