package shim_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/shim"
)

func TestInstallWritesExecutableShims(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")

	if err := shim.Install(dir, "/usr/local/bin/ranma", []string{"railway", "gh"}); err != nil {
		t.Fatalf("Install: %v", err)
	}

	for _, bin := range []string{"railway", "gh"} {
		path := filepath.Join(dir, bin)
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("shim %s: %v", bin, err)
		}
		if fi.Mode().Perm()&0o111 == 0 {
			t.Errorf("shim %s não é executável (%04o)", bin, fi.Mode().Perm())
		}
		raw, _ := os.ReadFile(path)
		body := string(raw)
		if !strings.Contains(body, "/usr/local/bin/ranma") {
			t.Errorf("o shim deve chamar o ranma por caminho absoluto:\n%s", body)
		}
		if !strings.Contains(body, "exec ") || !strings.Contains(body, bin) {
			t.Errorf("shim %s malformado:\n%s", bin, body)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, shim.MarkerFile)); err != nil {
		t.Errorf("marcador ausente: %v", err)
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	if err := shim.Install(dir, "/x/ranma", []string{"railway"}); err != nil {
		t.Fatal(err)
	}
	if err := shim.Install(dir, "/x/ranma", []string{"gh"}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dir, "railway")); err == nil {
		t.Error("Install deve regenerar o diretório: shim de provider removido não pode sobrar")
	}
	if _, err := os.Stat(filepath.Join(dir, "gh")); err != nil {
		t.Errorf("shim novo ausente: %v", err)
	}
}

func TestUninstallRefusesUnmanagedDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "importante.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := shim.Uninstall(dir); err == nil {
		t.Fatal("quero erro: sem marcador, o ranma não pode apagar o diretório")
	}
	if _, err := os.Stat(filepath.Join(dir, "importante.sh")); err != nil {
		t.Error("Uninstall apagou um diretório que não era dele")
	}
}

func TestUninstallRemovesManagedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	if err := shim.Install(dir, "/x/ranma", []string{"railway"}); err != nil {
		t.Fatal(err)
	}
	if err := shim.Uninstall(dir); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("diretório de shims deveria ter sumido")
	}
}
