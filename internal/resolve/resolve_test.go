package resolve_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/project"
	"github.com/devlefel/ranma/internal/provider"
	"github.com/devlefel/ranma/internal/resolve"
)

func fixture(t *testing.T, accounts string) (*provider.Registry, *account.Store, string) {
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
	return reg, st, t.TempDir()
}

func TestResolveSubstitutesTemplate(t *testing.T) {
	reg, st, cwd := fixture(t, "[railway.lefel]\ntoken = \"rw_secret\"\n")
	if err := project.Write(cwd, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}
	p, _ := reg.ByName("railway")

	res, err := resolve.Resolve(p, st, cwd)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.Account != "lefel" {
		t.Errorf("Account = %q, quero lefel", res.Account)
	}
	if res.Env["RAILWAY_API_TOKEN"] != "rw_secret" {
		t.Errorf("Env = %v, quero RAILWAY_API_TOKEN=rw_secret", res.Env)
	}
	if len(res.Clear) != 1 || res.Clear[0] != "RAILWAY_TOKEN" {
		t.Errorf("Clear = %v, quero [RAILWAY_TOKEN]", res.Clear)
	}
}

func TestResolveErrors(t *testing.T) {
	cases := []struct {
		name       string
		accounts   string
		declare    map[string]string
		wantReason resolve.Reason
		wantInMsg  []string
	}{
		{
			name:       "sem .ranma.toml",
			accounts:   "[railway.lefel]\ntoken = \"rw_secret\"\n",
			declare:    nil,
			wantReason: resolve.ReasonNoProjectFile,
			wantInMsg:  []string{"ranma link railway", "lefel"},
		},
		{
			name:       "provider não declarado",
			accounts:   "[railway.lefel]\ntoken = \"rw_secret\"\n",
			declare:    map[string]string{"gh": "devlefel"},
			wantReason: resolve.ReasonProviderNotDeclared,
			wantInMsg:  []string{"ranma link railway", "lefel"},
		},
		{
			name:       "conta declarada não cadastrada",
			accounts:   "[railway.lefel]\ntoken = \"rw_secret\"\n",
			declare:    map[string]string{"railway": "fantasma"},
			wantReason: resolve.ReasonAccountNotFound,
			wantInMsg:  []string{"fantasma", "ranma add railway fantasma"},
		},
		{
			name:       "conta sem o campo que o provider exige",
			accounts:   "[railway.lefel]\napi_key = \"errado\"\n",
			declare:    map[string]string{"railway": "lefel"},
			wantReason: resolve.ReasonMissingField,
			wantInMsg:  []string{"token"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg, st, cwd := fixture(t, tc.accounts)
			if tc.declare != nil {
				if err := project.Write(cwd, tc.declare); err != nil {
					t.Fatal(err)
				}
			}
			p, _ := reg.ByName("railway")

			_, err := resolve.Resolve(p, st, cwd)
			if err == nil {
				t.Fatal("quero erro")
			}

			var rerr *resolve.Error
			if !errors.As(err, &rerr) {
				t.Fatalf("quero *resolve.Error, deu %T", err)
			}
			if rerr.Reason != tc.wantReason {
				t.Errorf("Reason = %v, quero %v", rerr.Reason, tc.wantReason)
			}
			for _, want := range tc.wantInMsg {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("mensagem não contém %q:\n%s", want, err)
				}
			}
			if strings.Contains(err.Error(), "rw_secret") || strings.Contains(err.Error(), "errado") {
				t.Error("a mensagem de erro vazou uma credencial")
			}
		})
	}
}
