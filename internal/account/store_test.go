package account_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/account"
)

func write(t *testing.T, body string, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "accounts.toml")
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndGet(t *testing.T) {
	path := write(t, "[railway.lefel]\ntoken = \"rw_secret\"\n\n[resend.dev]\napi_key = \"re_secret\"\n", 0o600)

	st, err := account.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	fields, ok := st.Get("railway", "lefel")
	if !ok {
		t.Fatal("conta railway/lefel não encontrada")
	}
	if fields["token"] != "rw_secret" {
		t.Errorf("token = %q, quero rw_secret", fields["token"])
	}
	if _, ok := st.Get("railway", "nope"); ok {
		t.Error("conta inexistente retornou ok")
	}
	if got := st.Providers(); !slices.Equal(got, []string{"railway", "resend"}) {
		t.Errorf("Providers() = %v, quero [railway resend]", got)
	}
}

func TestLoadRejectsLoosePermissions(t *testing.T) {
	path := write(t, "[railway.lefel]\ntoken = \"x\"\n", 0o644)

	_, err := account.Load(path)
	if err == nil {
		t.Fatal("quero erro para arquivo 0644")
	}
	if !strings.Contains(err.Error(), "chmod 600") {
		t.Errorf("a mensagem deve ensinar a correção, deu: %v", err)
	}
	if strings.Contains(err.Error(), "x") && strings.Contains(err.Error(), "token = ") {
		t.Error("a mensagem de erro não pode conter conteúdo do arquivo")
	}
}

func TestLoadMissingFileGivesEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.toml")

	st, err := account.Load(path)
	if err != nil {
		t.Fatalf("arquivo ausente deve dar store vazio, deu: %v", err)
	}
	if len(st.Providers()) != 0 {
		t.Errorf("store deveria estar vazio, tem %v", st.Providers())
	}
}

func TestSaveRoundTripsWith0600(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "accounts.toml")

	st, err := account.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	st.Set("railway", "lefel", map[string]string{"token": "rw_secret"})
	if err := st.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("modo = %04o, quero 0600", fi.Mode().Perm())
	}

	again, err := account.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	fields, ok := again.Get("railway", "lefel")
	if !ok || fields["token"] != "rw_secret" {
		t.Errorf("round trip perdeu o valor: %v %v", fields, ok)
	}
}

func TestRemove(t *testing.T) {
	path := write(t, "[railway.lefel]\ntoken = \"x\"\n[railway.other]\ntoken = \"y\"\n", 0o600)
	st, _ := account.Load(path)

	if !st.Remove("railway", "lefel") {
		t.Error("Remove deveria achar a conta")
	}
	if st.Remove("railway", "lefel") {
		t.Error("Remove duas vezes deveria retornar false")
	}
	if got := st.Accounts("railway"); !slices.Equal(got, []string{"other"}) {
		t.Errorf("Accounts = %v, quero [other]", got)
	}
}

func TestRemoveLastAccountDropsProvider(t *testing.T) {
	path := write(t, "[railway.lefel]\ntoken = \"x\"\n", 0o600)
	st, _ := account.Load(path)

	st.Remove("railway", "lefel")
	if got := st.Providers(); len(got) != 0 {
		t.Errorf("Providers() = %v, quero vazio depois de remover a última conta", got)
	}
}

func TestMask(t *testing.T) {
	cases := map[string]string{
		"rw_Fe26.2xxxxxxxxxxxxxxx": "rw_Fe26.…",
		"short":                    "…",
		"":                         "…",
	}
	for in, want := range cases {
		if got := account.Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, quero %q", in, got, want)
		}
	}
}
