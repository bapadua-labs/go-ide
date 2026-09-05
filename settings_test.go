package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	pathOK := false
	for _, kv := range env {
		if kv == "GOROOT=/usr/local/go" {
			found = true
		}
		if kv == "GOROOT=/old" {
			t.Fatal("GOROOT antigo não foi substituído")
		}
		if strings.HasPrefix(kv, "PATH=") {
			pathVal := strings.TrimPrefix(kv, "PATH=")
			parts := filepath.SplitList(pathVal)
			if len(parts) == 0 || filepath.Clean(parts[0]) != filepath.Clean("/usr/local/go/bin") {
				t.Fatalf("PATH deveria começar com GOROOT/bin, got %q", pathVal)
			}
			pathOK = true
		}
	}
	if !found {
		t.Fatalf("GOROOT novo ausente em %#v", env)
	}
	if !pathOK {
		t.Fatalf("PATH com GOROOT/bin ausente em %#v", env)
	}
}

func TestPrependPathDir(t *testing.T) {
	sep := string(os.PathListSeparator)
	got := prependPathDir("/usr/bin"+sep+"/bin", "/usr/local/go/bin")
	want := "/usr/local/go/bin" + sep + "/usr/bin" + sep + "/bin"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// remove duplicata
	got = prependPathDir("/usr/local/go/bin"+sep+"/usr/bin", "/usr/local/go/bin")
	want = "/usr/local/go/bin" + sep + "/usr/bin"
	if got != want {
		t.Fatalf("dedupe: got %q want %q", got, want)
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
