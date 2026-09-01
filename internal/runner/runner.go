// Package runner locates the real provider binary and execs it with the
// resolved credentials in its environment.
package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/devlefel/ranma/internal/resolve"
)

// ExecFn is the syscall used to replace this process. Tests substitute it.
var ExecFn = syscall.Exec

// RealBinary finds bin on pathEnv, skipping shimDir so a shim never
// re-invokes itself.
func RealBinary(bin, pathEnv, shimDir string) (string, error) {
	skip, err := filepath.Abs(shimDir)
	if err != nil {
		skip = shimDir
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if abs == skip {
			continue
		}

		candidate := filepath.Join(abs, bin)
		fi, err := os.Stat(candidate)
		if err != nil || fi.IsDir() {
			continue
		}
		if fi.Mode().Perm()&0o111 == 0 {
			continue
		}
		return candidate, nil
	}

	return "", fmt.Errorf(
		"ranma: executável %q não encontrado no PATH fora de %s; o CLI está instalado?",
		bin, shimDir)
}

// BuildEnv renders the child environment: base minus the provider's clear
// list and minus any stale copy of an injected key, plus the resolved values.
func BuildEnv(base []string, res *resolve.Resolution) []string {
	drop := map[string]bool{"RANMA_ACCOUNT": true}
	for _, name := range res.Clear {
		drop[name] = true
	}
	for name := range res.Env {
		drop[name] = true
	}

	out := make([]string, 0, len(base)+len(res.Env)+1)
	for _, kv := range base {
		name, _, ok := strings.Cut(kv, "=")
		if ok && drop[name] {
			continue
		}
		out = append(out, kv)
	}
	for name, value := range res.Env {
		out = append(out, name+"="+value)
	}
	// Debug marker: says which account ran, never what the credential is.
	out = append(out, "RANMA_ACCOUNT="+res.Provider.Name+"/"+res.Account)
	return out
}

// Run replaces this process with the real binary.
func Run(res *resolve.Resolution, realBin string, args []string) error {
	argv := append([]string{res.Provider.Bin}, args...)
	return ExecFn(realBin, argv, BuildEnv(os.Environ(), res))
}

// Passthrough execs the real binary untouched, for login/help style commands.
func Passthrough(bin, realBin string, args []string) error {
	return ExecFn(realBin, append([]string{bin}, args...), os.Environ())
}
