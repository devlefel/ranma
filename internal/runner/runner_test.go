package runner_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/provider"
	"github.com/devlefel/ranma/internal/resolve"
	"github.com/devlefel/ranma/internal/runner"
)

func executable(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRealBinarySkipsShimDir(t *testing.T) {
	root := t.TempDir()
	shimDir := filepath.Join(root, "shims")
	realDir := filepath.Join(root, "real")
	executable(t, shimDir, "railway", "#!/bin/sh\nexit 0\n")
	want := executable(t, realDir, "railway", "#!/bin/sh\nexit 0\n")

	pathEnv := strings.Join([]string{shimDir, realDir}, string(os.PathListSeparator))

	got, err := runner.RealBinary("railway", pathEnv, shimDir)
	if err != nil {
		t.Fatalf("RealBinary: %v", err)
	}
	if got != want {
		t.Errorf("RealBinary = %q, quero %q (o shim deve ser pulado)", got, want)
	}
}

func TestRealBinarySkipsShimDirReachedThroughSymlink(t *testing.T) {
	root := t.TempDir()
	shimDir := filepath.Join(root, "shims")
	realDir := filepath.Join(root, "real")
	executable(t, shimDir, "railway", "#!/bin/sh\nexit 0\n")
	want := executable(t, realDir, "railway", "#!/bin/sh\nexit 0\n")

	// A second, lexically different path to the same physical directory —
	// what a symlinked $HOME or a dotfile manager produces.
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(shimDir, alias); err != nil {
		t.Skipf("symlink não suportado neste ambiente: %v", err)
	}

	pathEnv := strings.Join([]string{alias, realDir}, string(os.PathListSeparator))

	got, err := runner.RealBinary("railway", pathEnv, shimDir)
	if err != nil {
		t.Fatalf("RealBinary: %v", err)
	}
	if got != want {
		t.Errorf("RealBinary = %q, quero %q: o alias simbólico do shim dir deve ser pulado", got, want)
	}
}

func TestRealBinaryErrorsWhenOnlyShimExists(t *testing.T) {
	root := t.TempDir()
	shimDir := filepath.Join(root, "shims")
	executable(t, shimDir, "railway", "#!/bin/sh\nexit 0\n")

	_, err := runner.RealBinary("railway", shimDir, shimDir)
	if err == nil {
		t.Fatal("quero erro em vez de recursão infinita no shim")
	}
	if !strings.Contains(err.Error(), "railway") {
		t.Errorf("a mensagem deve nomear o binário, deu: %v", err)
	}
}

func TestBuildEnvInjectsAndClears(t *testing.T) {
	p := &provider.Provider{Name: "railway", Bin: "railway"}
	res := &resolve.Resolution{
		Provider: p,
		Account:  "lefel",
		Env:      map[string]string{"RAILWAY_API_TOKEN": "rw_secret"},
		Clear:    []string{"RAILWAY_TOKEN"},
	}
	base := []string{
		"HOME=/home/x",
		"RAILWAY_TOKEN=herdado_do_shell",
		"RAILWAY_API_TOKEN=antigo",
	}

	got := runner.BuildEnv(base, res)

	if !slices.Contains(got, "HOME=/home/x") {
		t.Error("variáveis não relacionadas devem sobreviver")
	}
	if !slices.Contains(got, "RAILWAY_API_TOKEN=rw_secret") {
		t.Errorf("token injetado ausente: %v", got)
	}
	for _, kv := range got {
		if strings.HasPrefix(kv, "RAILWAY_TOKEN=") {
			t.Errorf("RAILWAY_TOKEN deveria ter sido limpo, achei %q", kv)
		}
		if kv == "RAILWAY_API_TOKEN=antigo" {
			t.Error("o valor antigo não pode sobreviver junto com o injetado")
		}
	}
	if !slices.Contains(got, "RANMA_ACCOUNT=railway/lefel") {
		t.Errorf("quero RANMA_ACCOUNT como marcador de depuração: %v", got)
	}
}

func TestRunPassesArgv(t *testing.T) {
	var gotPath string
	var gotArgv []string
	original := runner.ExecFn
	t.Cleanup(func() { runner.ExecFn = original })
	runner.ExecFn = func(path string, argv, env []string) error {
		gotPath, gotArgv = path, argv
		return nil
	}

	res := &resolve.Resolution{
		Provider: &provider.Provider{Name: "railway", Bin: "railway"},
		Account:  "lefel",
		Env:      map[string]string{"RAILWAY_API_TOKEN": "rw_secret"},
	}
	if err := runner.Run(res, "/usr/bin/railway", []string{"up", "--detach"}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if gotPath != "/usr/bin/railway" {
		t.Errorf("path = %q", gotPath)
	}
	if !slices.Equal(gotArgv, []string{"railway", "up", "--detach"}) {
		t.Errorf("argv = %v, quero [railway up --detach]", gotArgv)
	}
}
