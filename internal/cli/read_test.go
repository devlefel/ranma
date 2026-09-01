package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/cli"
	"github.com/devlefel/ranma/internal/project"
	"github.com/devlefel/ranma/internal/provider"
)

func fixture(t *testing.T, accounts string) (*provider.Registry, *account.Store) {
	t.Helper()
	reg, err := provider.Load("")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "accounts.toml")
	if err := os.WriteFile(path, []byte(accounts), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := account.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return reg, st
}

func TestListMasksCredentials(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_Fe26.2SEGREDO\"\n")

	var buf bytes.Buffer
	if err := cli.List(&buf, reg, st, ""); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "railway") || !strings.Contains(out, "lefel") {
		t.Errorf("saída deve listar provider e conta:\n%s", out)
	}
	if strings.Contains(out, "SEGREDO") {
		t.Fatalf("credencial vazou na saída:\n%s", out)
	}
	if !strings.Contains(out, "rw_Fe26.…") {
		t.Errorf("quero o valor mascarado:\n%s", out)
	}
}

func TestWhoamiReportsPerProvider(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	cwd := t.TempDir()
	if err := project.Write(cwd, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := cli.Whoami(&buf, reg, st, cwd); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	if !strings.Contains(out, "railway") || !strings.Contains(out, "lefel") {
		t.Errorf("quero railway → lefel:\n%s", out)
	}
	if !strings.Contains(out, "gh") || !strings.Contains(out, "não declarado") {
		t.Errorf("quero gh marcado como não declarado:\n%s", out)
	}
	if strings.Contains(out, "rw_x") {
		t.Fatalf("credencial vazou:\n%s", out)
	}
}

func TestLinkWritesDeclaration(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	cwd := t.TempDir()

	if err := cli.Link(&bytes.Buffer{}, reg, st, cwd, "railway", "lefel"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	p, err := project.Read(filepath.Join(cwd, project.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if p.Use["railway"] != "lefel" {
		t.Errorf("Use = %v, quero railway=lefel", p.Use)
	}
}

func TestLinkMergesWithExistingDeclaration(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n[gh.devlefel]\ntoken = \"gh_x\"\n")
	cwd := t.TempDir()
	if err := project.Write(cwd, map[string]string{"gh": "devlefel"}); err != nil {
		t.Fatal(err)
	}

	if err := cli.Link(&bytes.Buffer{}, reg, st, cwd, "railway", "lefel"); err != nil {
		t.Fatal(err)
	}

	p, _ := project.Read(filepath.Join(cwd, project.FileName))
	if p.Use["gh"] != "devlefel" || p.Use["railway"] != "lefel" {
		t.Errorf("Link apagou a declaração existente: %v", p.Use)
	}
}

func TestLinkRefusesToOverwriteUnreadableDeclaration(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n[gh.devlefel]\ntoken = \"gh_x\"\n")
	cwd := t.TempDir()
	path := filepath.Join(cwd, project.FileName)

	// A corrupt .ranma.toml that still carries another provider's declaration.
	if err := os.WriteFile(path, []byte("[use]\ngh = \"devlefel\"\nrailway = \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := cli.Link(&bytes.Buffer{}, reg, st, cwd, "railway", "lefel"); err == nil {
		t.Fatal("quero erro: um .ranma.toml ilegível não pode ser sobrescrito às cegas")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("o arquivo foi alterado apesar do erro:\nantes:  %q\ndepois: %q", before, after)
	}
}

func TestListRejectsUnknownProviderFilter(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")

	err := cli.List(&bytes.Buffer{}, reg, st, "nuvem")
	if err == nil {
		t.Fatal("quero erro para filtro de provider desconhecido, não uma lista vazia")
	}
	if !strings.Contains(err.Error(), "railway") {
		t.Errorf("a mensagem deve listar os providers conhecidos, deu: %v", err)
	}
}

func TestLinkRefusesUnknownProviderOrAccount(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	cwd := t.TempDir()

	if err := cli.Link(&bytes.Buffer{}, reg, st, cwd, "nuvem", "x"); err == nil {
		t.Error("quero erro para provider desconhecido")
	}
	if err := cli.Link(&bytes.Buffer{}, reg, st, cwd, "railway", "fantasma"); err == nil {
		t.Error("quero erro para conta não cadastrada")
	}
	if _, err := os.Stat(filepath.Join(cwd, project.FileName)); err == nil {
		t.Error("nenhum arquivo deve ser escrito quando o link é recusado")
	}
}
