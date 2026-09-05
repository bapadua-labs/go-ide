package main

import (
	"testing"
)

func TestBracketColors_nested(t *testing.T) {
	text := "func f() { if (a[b]) {} }"
	colors := bracketColors(text)

	if colors[7] != 0 { // (
		t.Fatalf("abertura () em 7: got %d want 0", colors[7])
	}
	if colors[8] != 0 { // )
		t.Fatalf("fechamento () em 8: got %d want 0", colors[8])
	}
	if colors[10] != 0 { // {
		t.Fatalf("abertura { em 10: got %d want 0", colors[10])
	}
	if colors[14] != 1 { // (
		t.Fatalf("abertura ( em 14: got %d want 1", colors[14])
	}
	if colors[16] != 2 { // [
		t.Fatalf("abertura [ em 16: got %d want 2", colors[16])
	}
}

func TestBracketColors_ignoresStrings(t *testing.T) {
	text := `s := "({[]})"`
	colors := bracketColors(text)

	for i := 0; i < len(text); i++ {
		if text[i] == '(' || text[i] == ')' || text[i] == '[' || text[i] == ']' || text[i] == '{' || text[i] == '}' {
			if _, ok := colors[i]; ok {
				t.Fatalf("bracket em string não deveria ser colorido: pos %d", i)
			}
		}
	}
}

func TestBracketColors_ignoresLineComment(t *testing.T) {
	text := "x := 1 // ( unmatched"
	colors := bracketColors(text)

	for i := 0; i < len(text); i++ {
		if text[i] == '(' {
			if _, ok := colors[i]; ok {
				t.Fatalf("bracket em comentário não deveria ser colorido: pos %d", i)
			}
		}
	}
}

func TestBracketColors_mismatch(t *testing.T) {
	colors := bracketColors("func() }")
	for i := 0; i < len("func() }"); i++ {
		if string("func() }")[i] == '}' {
			if colors[i] != -1 {
				t.Fatalf("bracket sem par deveria ser -1, got %d", colors[i])
			}
		}
	}
}
