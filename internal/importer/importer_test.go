package importer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/importer"
	"github.com/devlefel/ranma/internal/provider"
)

func seed(t *testing.T, rel, body string) string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return home
}

func TestImportRailwayConfig(t *testing.T) {
	home := seed(t, ".railway/config.json",
		`{"projects":{},"user":{"token":"rw_Fe26.2SEGREDO"},"linkedFunctions":null}`)
	spec := &provider.ImportSpec{Kind: "railway-config", Path: ".railway/config.json"}

	fields, err := importer.Import(spec, home, "qualquer")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if fields["token"] != "rw_Fe26.2SEGREDO" {
		t.Errorf("token = %q", fields["token"])
	}
}

func TestImportGhHostsPicksNamedAccount(t *testing.T) {
	body := "github.com:\n" +
		"    users:\n" +
		"        felipeb-souza:\n" +
		"            oauth_token: gho_ERRADO\n" +
		"        devlefel:\n" +
		"            oauth_token: ghp_CERTO\n" +
		"    user: felipeb-souza\n" +
		"    oauth_token: gho_ERRADO\n"
	home := seed(t, ".config/gh/hosts.yml", body)
	spec := &provider.ImportSpec{Kind: "gh-hosts", Path: ".config/gh/hosts.yml"}

	fields, err := importer.Import(spec, home, "devlefel")
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if fields["token"] != "ghp_CERTO" {
		t.Errorf("token = %q, quero o da conta nomeada, não o ativo", fields["token"])
	}
}

func TestImportGhHostsUnknownAccountListsOptions(t *testing.T) {
	body := "github.com:\n    users:\n        devlefel:\n            oauth_token: ghp_x\n"
	home := seed(t, ".config/gh/hosts.yml", body)
	spec := &provider.ImportSpec{Kind: "gh-hosts", Path: ".config/gh/hosts.yml"}

	_, err := importer.Import(spec, home, "fantasma")
	if err == nil {
		t.Fatal("quero erro para conta ausente no hosts.yml")
	}
	if !strings.Contains(err.Error(), "devlefel") {
		t.Errorf("a mensagem deve listar as contas disponíveis, deu: %v", err)
	}
	if strings.Contains(err.Error(), "ghp_x") {
		t.Error("a mensagem vazou o token")
	}
}

func TestImportMissingFile(t *testing.T) {
	spec := &provider.ImportSpec{Kind: "railway-config", Path: ".railway/config.json"}
	_, err := importer.Import(spec, t.TempDir(), "x")
	if err == nil {
		t.Fatal("quero erro quando o CLI nativo nunca logou")
	}
}

func TestImportUnknownKind(t *testing.T) {
	spec := &provider.ImportSpec{Kind: "inventado", Path: "x"}
	if _, err := importer.Import(spec, t.TempDir(), "x"); err == nil {
		t.Fatal("quero erro para kind desconhecido")
	}
}
