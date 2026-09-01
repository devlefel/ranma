package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/cli"
	"github.com/devlefel/ranma/internal/project"
)

func findCheck(t *testing.T, checks []cli.Check, substr string) cli.Check {
	t.Helper()
	for _, c := range checks {
		if strings.Contains(c.Name, substr) {
			return c
		}
	}
	t.Fatalf("check %q não encontrado em %+v", substr, checks)
	return cli.Check{}
}

func TestDoctorFlagsShimDirMissingFromPath(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	shimDir := t.TempDir()

	checks := cli.Doctor(reg, st, t.TempDir(), "/usr/bin:/bin", shimDir, "/tmp/accounts.toml")

	c := findCheck(t, checks, "PATH")
	if c.OK {
		t.Errorf("o check de PATH deveria falhar quando o shim dir não está nele: %+v", c)
	}
	if !strings.Contains(c.Detail, shimDir) {
		t.Errorf("o detalhe deve mostrar o diretório esperado: %q", c.Detail)
	}
}

func TestDoctorFlagsShimDirNotFirst(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	shimDir := t.TempDir()
	pathEnv := "/usr/bin" + string(os.PathListSeparator) + shimDir

	checks := cli.Doctor(reg, st, t.TempDir(), pathEnv, shimDir, "/tmp/accounts.toml")

	c := findCheck(t, checks, "PATH")
	if c.OK {
		t.Error("estar no PATH depois dos CLIs reais não garante interceptação; deveria falhar")
	}
}

func TestDoctorPassesWhenShimDirIsFirst(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	shimDir := t.TempDir()
	pathEnv := shimDir + string(os.PathListSeparator) + "/usr/bin"

	checks := cli.Doctor(reg, st, t.TempDir(), pathEnv, shimDir, "/tmp/accounts.toml")

	if c := findCheck(t, checks, "PATH"); !c.OK {
		t.Errorf("quero PATH ok: %+v", c)
	}
}

func TestDoctorReportsProjectResolution(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_SEGREDO\"\n")
	cwd := t.TempDir()
	if err := project.Write(cwd, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}

	// Doctor reports "binário ausente" before it reports resolution, so the
	// PATH it is handed must actually contain a railway to intercept.
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "railway"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	checks := cli.Doctor(reg, st, cwd, binDir, t.TempDir(), "/tmp/accounts.toml")

	c := findCheck(t, checks, "railway")
	if !c.OK || !strings.Contains(c.Detail, "lefel") {
		t.Errorf("quero railway resolvendo para lefel: %+v", c)
	}
	for _, check := range checks {
		if strings.Contains(check.Detail, "rw_SEGREDO") {
			t.Fatalf("credencial vazou em %+v", check)
		}
	}
}

func TestDoctorChecksAccountsPermissions(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	loose := filepath.Join(t.TempDir(), "accounts.toml")
	if err := os.WriteFile(loose, []byte("[railway.lefel]\ntoken = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	checks := cli.Doctor(reg, st, t.TempDir(), "/usr/bin", t.TempDir(), loose)

	if c := findCheck(t, checks, "permiss"); c.OK {
		t.Errorf("0644 deveria falhar o check de permissão: %+v", c)
	}
}

func TestPrintChecksReportsFailure(t *testing.T) {
	var buf bytes.Buffer
	ok := cli.PrintChecks(&buf, []cli.Check{
		{Name: "a", OK: true, Detail: "tudo certo"},
		{Name: "b", OK: false, Detail: "problema"},
	})
	if ok {
		t.Error("PrintChecks deveria retornar false quando há falha")
	}
	out := buf.String()
	if !strings.Contains(out, "✓") || !strings.Contains(out, "✗") {
		t.Errorf("quero marcadores visuais:\n%s", out)
	}
}

func TestVerifyAccountsSkipsProvidersWithoutBinary(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")

	checks := cli.VerifyAccounts(reg, st, t.TempDir(), t.TempDir())

	c := findCheck(t, checks, "railway")
	if c.OK {
		t.Error("sem o binário no PATH não dá para verificar; deve reportar falha")
	}
	if !strings.Contains(c.Detail, "railway") {
		t.Errorf("o detalhe deve nomear o binário ausente: %q", c.Detail)
	}
}

func TestVerifyAccountsRunsVerifyArgs(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_SEGREDO\"\n")

	binDir := t.TempDir()
	// Passes only if the injected token arrives; fails when it is empty.
	fake := "#!/bin/sh\n[ \"$RAILWAY_API_TOKEN\" = \"rw_SEGREDO\" ] || exit 1\necho ok\n"
	if err := os.WriteFile(filepath.Join(binDir, "railway"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	checks := cli.VerifyAccounts(reg, st, binDir, t.TempDir())

	c := findCheck(t, checks, "railway/lefel")
	if !c.OK {
		t.Errorf("quero a conta aprovada: %+v", c)
	}
	for _, check := range checks {
		if strings.Contains(check.Detail, "rw_SEGREDO") {
			t.Fatalf("credencial vazou em %+v", check)
		}
	}
}
