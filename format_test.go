package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestFormatGoSource(t *testing.T) {
	src := "package main\n\nfunc main(){fmt.Println(\"ok\")}\n"
	formatted, err := formatGoSource(src, runtime.GOROOT(), t.TempDir())
	if err != nil {
		if strings.Contains(err.Error(), "não encontrado") {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	if !strings.Contains(formatted, `"fmt"`) {
		t.Fatalf("expected goimports to add fmt import:\n%s", formatted)
	}
	if !strings.Contains(formatted, "func main() {") {
		t.Fatalf("unexpected output:\n%s", formatted)
	}
}

func TestFormatGoSourceInvalid(t *testing.T) {
	_, err := formatGoSource("package main\nfunc {", runtime.GOROOT(), t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid Go source")
	}
	if strings.Contains(err.Error(), "não encontrado") {
		t.Skip(err.Error())
	}
}
