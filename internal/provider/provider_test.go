package provider_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/devlefel/ranma/internal/provider"
)

func TestLoadBuiltins(t *testing.T) {
	reg, err := provider.Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p, ok := reg.ByBin("railway")
	if !ok {
		t.Fatal("provider railway não encontrado por bin")
	}
	if p.Name != "railway" {
		t.Errorf("Name = %q, quero %q", p.Name, "railway")
	}
	if got := p.Env["RAILWAY_API_TOKEN"]; got != "{{token}}" {
		t.Errorf("Env[RAILWAY_API_TOKEN] = %q, quero %q", got, "{{token}}")
	}
	if !slices.Contains(p.Clear, "RAILWAY_TOKEN") {
		t.Errorf("Clear = %v, quero conter RAILWAY_TOKEN", p.Clear)
	}
	if p.Import == nil || p.Import.Kind != "railway-config" {
		t.Errorf("Import = %+v, quero kind railway-config", p.Import)
	}

	if got := reg.Names(); !slices.Equal(got, []string{"gh", "railway", "resend"}) {
		t.Errorf("Names() = %v, quero [gh railway resend] em ordem", got)
	}
}

func TestUserFileOverridesBuiltin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.toml")
	body := "[railway]\nbin = \"railway-beta\"\nenv = { RAILWAY_API_TOKEN = \"{{token}}\" }\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := provider.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	p, ok := reg.ByName("railway")
	if !ok {
		t.Fatal("provider railway sumiu")
	}
	if p.Bin != "railway-beta" {
		t.Errorf("Bin = %q, quero railway-beta", p.Bin)
	}
	if len(p.Clear) != 0 {
		t.Errorf("Clear = %v, quero vazio: o bloco do usuário substitui o embutido inteiro", p.Clear)
	}
	if _, ok := reg.ByBin("railway"); ok {
		t.Error("bin railway ainda indexado; o índice deve seguir o bin novo")
	}
}

func TestUserFileAddsProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.toml")
	body := "[vercel]\nbin = \"vercel\"\nenv = { VERCEL_TOKEN = \"{{token}}\" }\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	reg, err := provider.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, ok := reg.ByBin("vercel"); !ok {
		t.Error("provider vercel do usuário não foi carregado")
	}
	if _, ok := reg.ByBin("railway"); !ok {
		t.Error("providers embutidos sumiram ao carregar o arquivo do usuário")
	}
}

func TestLoadMissingUserFileIsNotAnError(t *testing.T) {
	if _, err := provider.Load(filepath.Join(t.TempDir(), "nope.toml")); err != nil {
		t.Fatalf("arquivo do usuário ausente deve ser silencioso, deu: %v", err)
	}
}

func TestIsPassthrough(t *testing.T) {
	reg, err := provider.Load("")
	if err != nil {
		t.Fatal(err)
	}
	p, _ := reg.ByName("railway")

	cases := []struct {
		name string
		args []string
		want bool
	}{
		{"sem args mostra ajuda", nil, true},
		{"login", []string{"login"}, true},
		{"up precisa de conta", []string{"up"}, false},
		{"flag antes do subcomando", []string{"--json", "up"}, false},
		{"subcomando não listado", []string{"status"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := p.IsPassthrough(tc.args); got != tc.want {
				t.Errorf("IsPassthrough(%v) = %v, quero %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestFields(t *testing.T) {
	reg, _ := provider.Load("")
	p, _ := reg.ByName("resend")
	if got := p.Fields(); !slices.Equal(got, []string{"api_key"}) {
		t.Errorf("Fields() = %v, quero [api_key]", got)
	}
}
