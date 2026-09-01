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

func TestVerifyAccountsRedactsEchoedCredential(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_SEGREDO_ECOADO\"\n")

	binDir := t.TempDir()
	// A provider CLI that echoes its own credential, the way a verbose auth
	// error or a debug flag does.
	fake := "#!/bin/sh\necho \"auth error: token was: $RAILWAY_API_TOKEN\"\nexit 1\n"
	if err := os.WriteFile(filepath.Join(binDir, "railway"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	checks := cli.VerifyAccounts(reg, st, binDir, t.TempDir())

	for _, c := range checks {
		if strings.Contains(c.Detail, "rw_SEGREDO_ECOADO") {
			t.Fatalf("credencial vazou no relatório do doctor: %+v", c)
		}
	}
	if c := findCheck(t, checks, "railway/lefel"); !strings.Contains(c.Detail, "[REDACTED]") {
		t.Errorf("quero o marcador de redação no lugar do token, deu: %q", c.Detail)
	}
}

func TestVerifyAccountsRedactsPartialAndAlteredEchoes(t *testing.T) {
	const secret = "rw_SEGREDO_ECOADO_1234567890"
	reg, st := fixture(t, "[railway.lefel]\ntoken = \""+secret+"\"\n")

	cases := []struct {
		name string
		echo string
	}{
		{"valor inteiro", `echo "token: $RAILWAY_API_TOKEN"`},
		{"na segunda linha", `echo primeira; echo "token: $RAILWAY_API_TOKEN"`},
		{"prefixo truncado", `echo "invalid token, got: $(echo $RAILWAY_API_TOKEN | cut -c1-20)"`},
		{"em maiúsculas", `echo "token: $(echo $RAILWAY_API_TOKEN | tr a-z A-Z)"`},
		{"dentro de JSON", `echo "{\"token\":\"$RAILWAY_API_TOKEN\"}"`},
		{"caminho de sucesso", `echo "ok, autenticado com $RAILWAY_API_TOKEN"; exit 0`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			fake := "#!/bin/sh\n" + tc.echo + "\n"
			if err := os.WriteFile(filepath.Join(binDir, "railway"), []byte(fake), 0o755); err != nil {
				t.Fatal(err)
			}

			checks := cli.VerifyAccounts(reg, st, binDir, t.TempDir())

			for _, c := range checks {
				// Nothing longer than what `ranma ls` itself prints may survive.
				if strings.Contains(strings.ToLower(c.Detail), strings.ToLower(secret[:8])) {
					t.Errorf("credencial vazou (%s): %q", tc.name, c.Detail)
				}
			}
		})
	}
}

func TestVerifyAccountsRedactsRawFieldWrappedInTemplate(t *testing.T) {
	const secret = "rw_RASTREAVEL_BEARER_5555"

	dir := t.TempDir()
	providersPath := filepath.Join(dir, "providers.toml")
	providersBody := "[bearer]\nbin = \"bearerctl\"\nenv = { AUTHZ = \"Bearer {{token}}\" }\nverify = [\"whoami\"]\n"
	if err := os.WriteFile(providersPath, []byte(providersBody), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := provider.Load(providersPath)
	if err != nil {
		t.Fatal(err)
	}

	accountsPath := filepath.Join(dir, "accounts.toml")
	accountsBody := "[bearer.acct]\ntoken = \"" + secret + "\"\n"
	if err := os.WriteFile(accountsPath, []byte(accountsBody), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := account.Load(accountsPath)
	if err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	// The env value is "Bearer <token>", never the raw token alone. A CLI
	// that strips the "Bearer " wrapper before echoing leaks the raw token,
	// which does not match any rendered env value as a prefix.
	fake := "#!/bin/sh\necho \"${AUTHZ#Bearer }\"\n"
	if err := os.WriteFile(filepath.Join(binDir, "bearerctl"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	checks := cli.VerifyAccounts(reg, st, binDir, t.TempDir())

	c := findCheck(t, checks, "bearer/acct")
	if strings.Contains(strings.ToLower(c.Detail), strings.ToLower(secret[:8])) {
		t.Fatalf("credencial crua vazou por trás do template: %q", c.Detail)
	}
}

func TestRedactLeavesInnocentOutputAlone(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_SEGREDO_ECOADO_1234567890\"\n")

	binDir := t.TempDir()
	fake := "#!/bin/sh\necho 'lefel@example.com — plano Pro, 3 projetos'\n"
	if err := os.WriteFile(filepath.Join(binDir, "railway"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	checks := cli.VerifyAccounts(reg, st, binDir, t.TempDir())

	c := findCheck(t, checks, "railway/lefel")
	if !strings.Contains(c.Detail, "lefel@example.com") {
		t.Errorf("saída legítima foi mutilada pela redação: %q", c.Detail)
	}
	if strings.Contains(c.Detail, "[REDACTED]") {
		t.Errorf("redação disparou em saída sem segredo: %q", c.Detail)
	}
}

func TestDoctorAcceptsShimDirReachedThroughSymlink(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")

	root := t.TempDir()
	shimDir := filepath.Join(root, "shims")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(shimDir, alias); err != nil {
		t.Skipf("symlink não suportado neste ambiente: %v", err)
	}

	// runner.RealBinary resolves this alias and intercepts correctly, so
	// doctor must agree instead of telling the user to fix a working install.
	checks := cli.Doctor(reg, st, t.TempDir(), alias, shimDir, "/tmp/accounts.toml")

	if c := findCheck(t, checks, "PATH"); !c.OK {
		t.Errorf("quero PATH ok quando o shim dir é alcançado por symlink: %+v", c)
	}
}

func TestDoctorDoesNotFailForProviderTheProjectDoesNotUse(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")

	cwd := t.TempDir()
	if err := project.Write(cwd, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}

	binDir := t.TempDir()
	for _, bin := range []string{"railway", "gh", "resend"} {
		if err := os.WriteFile(filepath.Join(binDir, bin), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	shimDir := t.TempDir()
	pathEnv := shimDir + string(os.PathListSeparator) + binDir

	accountsPath := filepath.Join(t.TempDir(), "accounts.toml")
	if err := os.WriteFile(accountsPath, []byte("[railway.lefel]\ntoken = \"rw_x\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	checks := cli.Doctor(reg, st, cwd, pathEnv, shimDir, accountsPath)

	var buf bytes.Buffer
	if ok := cli.PrintChecks(&buf, checks); !ok {
		t.Errorf("providers não usados pelo projeto não podem falhar o doctor:\n%s", buf.String())
	}
}

func TestDoctorFlagsUnknownProviderInDeclaration(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")

	cwd := t.TempDir()
	if err := project.Write(cwd, map[string]string{"railwey": "lefel"}); err != nil {
		t.Fatal(err)
	}

	checks := cli.Doctor(reg, st, cwd, "/usr/bin", t.TempDir(), "/tmp/accounts.toml")

	c := findCheck(t, checks, "railwey")
	if c.OK || c.Note {
		t.Errorf("chave desconhecida no .ranma.toml deve falhar, não ser nota: %+v", c)
	}
	if !strings.Contains(c.Detail, filepath.Join(cwd, project.FileName)) {
		t.Errorf("detalhe deve nomear o .ranma.toml culpado: %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "railwey") {
		t.Errorf("detalhe deve nomear a chave errada: %q", c.Detail)
	}
	if !strings.Contains(c.Detail, "railway") {
		t.Errorf("detalhe deve listar os providers conhecidos: %q", c.Detail)
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
