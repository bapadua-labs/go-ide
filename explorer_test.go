package main

import "testing"

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
