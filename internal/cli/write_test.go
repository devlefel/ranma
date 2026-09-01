package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/cli"
	"github.com/devlefel/ranma/internal/provider"
)

func emptyStore(t *testing.T) (*provider.Registry, *account.Store, string) {
	t.Helper()
	reg, err := provider.Load("")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "accounts.toml")
	st, err := account.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return reg, st, path
}

func TestAddPromptsForDerivedField(t *testing.T) {
	reg, st, path := emptyStore(t)
	var prompted string
	reader := func(prompt string) (string, error) {
		prompted = prompt
		return "re_SEGREDO", nil
	}

	var buf bytes.Buffer
	if err := cli.Add(&buf, reg, st, "resend", "dev", false, reader, t.TempDir()); err != nil {
		t.Fatalf("Add: %v", err)
	}

	if !strings.Contains(prompted, "api_key") {
		t.Errorf("o prompt deve nomear o campo derivado do provider, deu %q", prompted)
	}
	if strings.Contains(buf.String(), "re_SEGREDO") {
		t.Fatalf("a confirmação vazou a credencial:\n%s", buf.String())
	}

	reloaded, err := account.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	fields, ok := reloaded.Get("resend", "dev")
	if !ok || fields["api_key"] != "re_SEGREDO" {
		t.Errorf("conta não foi persistida: %v %v", fields, ok)
	}
}

func TestAddWithImport(t *testing.T) {
	reg, st, path := emptyStore(t)
	home := t.TempDir()
	cfg := filepath.Join(home, ".railway", "config.json")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(`{"user":{"token":"rw_IMPORTADO"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	refuse := func(string) (string, error) { t.Fatal("--import não deve pedir nada"); return "", nil }

	var buf bytes.Buffer
	if err := cli.Add(&buf, reg, st, "railway", "lefel", true, refuse, home); err != nil {
		t.Fatalf("Add --import: %v", err)
	}

	reloaded, _ := account.Load(path)
	fields, ok := reloaded.Get("railway", "lefel")
	if !ok || fields["token"] != "rw_IMPORTADO" {
		t.Errorf("import não persistiu: %v %v", fields, ok)
	}
}

func TestAddRejectsUnknownProvider(t *testing.T) {
	reg, st, _ := emptyStore(t)
	reader := func(string) (string, error) { return "x", nil }

	err := cli.Add(&bytes.Buffer{}, reg, st, "nuvem", "x", false, reader, t.TempDir())
	if err == nil {
		t.Fatal("quero erro para provider desconhecido")
	}
	if !strings.Contains(err.Error(), "railway") {
		t.Errorf("a mensagem deve listar os providers conhecidos, deu: %v", err)
	}
}

func TestAddRejectsEmptySecret(t *testing.T) {
	reg, st, _ := emptyStore(t)
	reader := func(string) (string, error) { return "   ", nil }

	if err := cli.Add(&bytes.Buffer{}, reg, st, "railway", "lefel", false, reader, t.TempDir()); err == nil {
		t.Fatal("quero erro para credencial vazia")
	}
}

func TestRemove(t *testing.T) {
	reg, st, path := emptyStore(t)
	reader := func(string) (string, error) { return "rw_x", nil }
	if err := cli.Add(&bytes.Buffer{}, reg, st, "railway", "lefel", false, reader, t.TempDir()); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := cli.Remove(&buf, st, "railway", "lefel"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	reloaded, _ := account.Load(path)
	if _, ok := reloaded.Get("railway", "lefel"); ok {
		t.Error("conta continua no arquivo depois do rm")
	}

	if err := cli.Remove(&buf, st, "railway", "lefel"); err == nil {
		t.Error("remover conta inexistente deve dar erro")
	}
}
