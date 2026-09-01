package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var ranmaBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "ranma-e2e-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	ranmaBin = filepath.Join(dir, "ranma")
	build := exec.Command("go", "build", "-o", ranmaBin, "../cmd/ranma")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// env prepares an isolated HOME with a fake `railway` that prints its
// environment, plus shims installed in front of it on PATH.
func env(t *testing.T) (home, projectDir, shimDir, pathEnv string) {
	t.Helper()

	home = t.TempDir()
	fakeDir := filepath.Join(home, "fakebin")
	if err := os.MkdirAll(fakeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fake := "#!/bin/sh\nprintf 'ARGS:%s\\n' \"$*\"\nenv | grep -E '^(RAILWAY_|RANMA_)' | sort\n"
	if err := os.WriteFile(filepath.Join(fakeDir, "railway"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}

	shimDir = filepath.Join(home, ".local", "share", "ranma", "bin")
	projectDir = filepath.Join(home, "proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}

	pathEnv = strings.Join([]string{shimDir, fakeDir, "/usr/bin", "/bin"}, string(os.PathListSeparator))
	return home, projectDir, shimDir, pathEnv
}

func run(t *testing.T, home, dir, pathEnv string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"PATH="+pathEnv,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
		"RAILWAY_TOKEN=herdado_do_shell",
	)
	// `ranma add` reads the credential from stdin when there is no TTY.
	if len(args) > 1 && args[1] == "add" {
		cmd.Stdin = strings.NewReader("rw_SEGREDO\n")
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestShimInjectsResolvedAccount(t *testing.T) {
	home, projectDir, shimDir, pathEnv := env(t)

	if out, err := run(t, home, projectDir, pathEnv, ranmaBin, "shim", "install"); err != nil {
		t.Fatalf("shim install: %v\n%s", err, out)
	}
	if out, err := run(t, home, projectDir, pathEnv, ranmaBin, "add", "railway", "lefel"); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	} else if !strings.Contains(out, "cadastrada") {
		t.Fatalf("add não confirmou:\n%s", out)
	}
	if out, err := run(t, home, projectDir, pathEnv, ranmaBin, "link", "railway", "lefel"); err != nil {
		t.Fatalf("link: %v\n%s", err, out)
	}

	out, err := run(t, home, projectDir, pathEnv, filepath.Join(shimDir, "railway"), "up", "--detach")
	if err != nil {
		t.Fatalf("shim railway: %v\n%s", err, out)
	}

	if !strings.Contains(out, "ARGS:up --detach") {
		t.Errorf("argumentos não chegaram ao binário real:\n%s", out)
	}
	if !strings.Contains(out, "RAILWAY_API_TOKEN=rw_SEGREDO") {
		t.Errorf("token injetado ausente:\n%s", out)
	}
	if strings.Contains(out, "herdado_do_shell") {
		t.Errorf("RAILWAY_TOKEN herdado do shell não foi limpo:\n%s", out)
	}
	if !strings.Contains(out, "RANMA_ACCOUNT=railway/lefel") {
		t.Errorf("marcador de conta ausente:\n%s", out)
	}
}

func TestShimBlocksUndeclaredProject(t *testing.T) {
	home, projectDir, shimDir, pathEnv := env(t)

	if out, err := run(t, home, projectDir, pathEnv, ranmaBin, "shim", "install"); err != nil {
		t.Fatalf("shim install: %v\n%s", err, out)
	}
	if out, err := run(t, home, projectDir, pathEnv, ranmaBin, "add", "railway", "lefel"); err != nil {
		t.Fatalf("add: %v\n%s", err, out)
	}
	// Deliberadamente sem `ranma link`.

	out, err := run(t, home, projectDir, pathEnv, filepath.Join(shimDir, "railway"), "up")
	if err == nil {
		t.Fatalf("o comando deveria ter sido bloqueado, saída:\n%s", out)
	}
	if !strings.Contains(out, "ranma link railway") {
		t.Errorf("a mensagem deve ensinar a correção:\n%s", out)
	}
	if strings.Contains(out, "ARGS:") {
		t.Errorf("o binário real rodou apesar do bloqueio:\n%s", out)
	}
	if strings.Contains(out, "rw_SEGREDO") {
		t.Errorf("credencial vazou na mensagem de bloqueio:\n%s", out)
	}
}

func TestShimPassesThroughLogin(t *testing.T) {
	home, projectDir, shimDir, pathEnv := env(t)
	if out, err := run(t, home, projectDir, pathEnv, ranmaBin, "shim", "install"); err != nil {
		t.Fatalf("shim install: %v\n%s", err, out)
	}

	out, err := run(t, home, projectDir, pathEnv, filepath.Join(shimDir, "railway"), "login")
	if err != nil {
		t.Fatalf("login deveria passar direto: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ARGS:login") {
		t.Errorf("login não chegou ao binário real:\n%s", out)
	}
	if strings.Contains(out, "RANMA_ACCOUNT=") {
		t.Errorf("passthrough não deve injetar conta:\n%s", out)
	}
}
