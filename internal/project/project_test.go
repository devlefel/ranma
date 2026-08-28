package project_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/devlefel/ranma/internal/project"
)

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := project.Write(root, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}

	p, err := project.Find(deep)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if p == nil {
		t.Fatal("Find não achou o .ranma.toml do ancestral")
	}
	if p.Use["railway"] != "lefel" {
		t.Errorf("Use[railway] = %q, quero lefel", p.Use["railway"])
	}
	if want := filepath.Join(root, project.FileName); p.Path != want {
		t.Errorf("Path = %q, quero %q", p.Path, want)
	}
}

func TestFindNearestWins(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := project.Write(root, map[string]string{"railway": "outer", "gh": "devlefel"}); err != nil {
		t.Fatal(err)
	}
	if err := project.Write(inner, map[string]string{"railway": "inner"}); err != nil {
		t.Fatal(err)
	}

	p, err := project.Find(inner)
	if err != nil {
		t.Fatal(err)
	}
	if p.Use["railway"] != "inner" {
		t.Errorf("Use[railway] = %q, quero inner", p.Use["railway"])
	}
	if _, ok := p.Use["gh"]; ok {
		t.Error("o arquivo mais próximo vence inteiro; não deve haver merge com o de cima")
	}
}

func TestFindReturnsNilWhenAbsent(t *testing.T) {
	p, err := project.Find(t.TempDir())
	if err != nil {
		t.Fatalf("ausência não é erro, deu: %v", err)
	}
	if p != nil {
		t.Errorf("quero nil, deu %+v", p)
	}
}

func TestWriteIsDeterministicAndSorted(t *testing.T) {
	dir := t.TempDir()
	if err := project.Write(dir, map[string]string{"railway": "lefel", "gh": "devlefel"}); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, project.FileName))
	if err != nil {
		t.Fatal(err)
	}
	want := "[use]\ngh = \"devlefel\"\nrailway = \"lefel\"\n"
	if string(raw) != want {
		t.Errorf("arquivo =\n%q\nquero\n%q", raw, want)
	}
}
