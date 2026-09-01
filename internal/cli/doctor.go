package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/project"
	"github.com/devlefel/ranma/internal/provider"
	"github.com/devlefel/ranma/internal/resolve"
	"github.com/devlefel/ranma/internal/runner"
)

// Check is one diagnostic result.
type Check struct {
	Name   string
	Detail string
	OK     bool
}

// Doctor diagnoses the installation and the current directory. It never
// executes a provider CLI, so it is safe to run anywhere.
func Doctor(reg *provider.Registry, st *account.Store, cwd, pathEnv, shimDir, accountsPath string) []Check {
	var checks []Check

	checks = append(checks, checkPath(pathEnv, shimDir))
	checks = append(checks, checkPermissions(accountsPath))

	for _, name := range reg.Names() {
		p, _ := reg.ByName(name)

		if _, err := runner.RealBinary(p.Bin, pathEnv, shimDir); err != nil {
			checks = append(checks, Check{
				Name:   name + ": binário",
				Detail: fmt.Sprintf("%s não está instalado; o ranma não tem o que interceptar", p.Bin),
			})
			continue
		}

		res, err := resolve.Resolve(p, st, cwd)
		if err == nil {
			checks = append(checks, Check{
				Name: name, OK: true,
				Detail: "este diretório resolve para a conta " + res.Account,
			})
			continue
		}

		// Read the error's fields rather than re-parsing its rendered text:
		// VerifyAccounts already does that, and string surgery on Error() breaks
		// the moment the message format changes.
		var rerr *resolve.Error
		detail := err.Error()
		if errors.As(err, &rerr) {
			switch rerr.Reason {
			case resolve.ReasonNoProjectFile:
				detail = "nenhum " + project.FileName + " encontrado a partir deste diretório"
			case resolve.ReasonProviderNotDeclared:
				detail = "não declarado neste projeto"
			case resolve.ReasonAccountNotFound:
				detail = fmt.Sprintf("conta %q declarada mas não cadastrada", rerr.Account)
			case resolve.ReasonMissingField:
				detail = fmt.Sprintf("conta %q sem o campo %q", rerr.Account, rerr.Field)
			}
		}
		checks = append(checks, Check{Name: name, Detail: detail})
	}

	return checks
}

func checkPath(pathEnv, shimDir string) Check {
	// Judge the PATH by the same rule interception uses, symlinks included:
	// telling the user to fix an installation that already works is worse
	// than saying nothing.
	want := runner.Canonical(shimDir)

	entries := filepath.SplitList(pathEnv)
	for i, dir := range entries {
		if runner.Canonical(dir) != want {
			continue
		}
		if i == 0 {
			return Check{Name: "PATH", OK: true, Detail: shimDir + " está no início do PATH"}
		}
		return Check{
			Name: "PATH",
			Detail: fmt.Sprintf(
				"%s está no PATH mas na posição %d; os CLIs reais vêm antes e não serão interceptados\n"+
					"    Corrija com: export PATH=%q:$PATH", shimDir, i+1, shimDir),
		}
	}

	return Check{
		Name:   "PATH",
		Detail: fmt.Sprintf("%s não está no PATH\n    Corrija com: export PATH=%q:$PATH", shimDir, shimDir),
	}
}

func checkPermissions(accountsPath string) Check {
	fi, err := os.Stat(accountsPath)
	if os.IsNotExist(err) {
		return Check{
			Name: "permissões", OK: true,
			Detail: "nenhuma credencial cadastrada ainda",
		}
	}
	if err != nil {
		return Check{Name: "permissões", Detail: err.Error()}
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return Check{
			Name:   "permissões",
			Detail: fmt.Sprintf("%s está %04o\n    Corrija com: chmod 600 %s", accountsPath, perm, accountsPath),
		}
	}
	return Check{Name: "permissões", OK: true, Detail: accountsPath + " está 0600"}
}

// PrintChecks renders the diagnosis, reporting whether everything passed.
func PrintChecks(w io.Writer, checks []Check) bool {
	all := true
	for _, c := range checks {
		mark := "✓"
		if !c.OK {
			mark, all = "✗", false
		}
		fmt.Fprintf(w, "%s %s\n    %s\n", mark, c.Name, c.Detail)
	}
	return all
}

// VerifyAccounts runs each provider's verify command once per registered
// account, with that account's credentials injected. It is opt-in because it
// makes the provider CLI hit the network.
func VerifyAccounts(reg *provider.Registry, st *account.Store, pathEnv, shimDir string) []Check {
	var checks []Check

	for _, name := range reg.Names() {
		p, _ := reg.ByName(name)
		accounts := st.Accounts(name)
		if len(accounts) == 0 {
			continue
		}
		if len(p.Verify) == 0 {
			checks = append(checks, Check{
				Name: name, OK: true,
				Detail: "provider sem comando de verificação declarado",
			})
			continue
		}

		realBin, err := runner.RealBinary(p.Bin, pathEnv, shimDir)
		if err != nil {
			checks = append(checks, Check{
				Name:   name,
				Detail: fmt.Sprintf("%s não está no PATH; não dá para verificar", p.Bin),
			})
			continue
		}

		for _, acct := range accounts {
			fields, _ := st.Get(name, acct)
			res, err := resolve.Render(p, acct, fields)
			if err != nil {
				// Render knows nothing about directories, so its full message
				// would claim "disponíveis: nenhuma cadastrada" here. Verifying
				// a credential is not a question about any project: report the
				// missing field alone.
				detail := err.Error()
				var rerr *resolve.Error
				if errors.As(err, &rerr) && rerr.Reason == resolve.ReasonMissingField {
					detail = fmt.Sprintf("conta sem o campo %q", rerr.Field)
				}
				checks = append(checks, Check{Name: name + "/" + acct, Detail: detail})
				continue
			}

			cmd := exec.Command(realBin, p.Verify...)
			cmd.Env = runner.BuildEnv(os.Environ(), res)
			out, err := cmd.CombinedOutput()
			safe := redact(string(out), res)
			if err != nil {
				checks = append(checks, Check{
					Name:   name + "/" + acct,
					Detail: fmt.Sprintf("%s %s falhou: %s", p.Bin, strings.Join(p.Verify, " "), firstLine(safe)),
				})
				continue
			}
			checks = append(checks, Check{
				Name: name + "/" + acct, OK: true,
				Detail: "credencial aceita: " + firstLine(safe),
			})
		}
	}
	return checks
}

// redact removes every injected secret from a provider CLI's output. A CLI
// that echoes its own credential — a verbose auth error, a debug flag — must
// never leak it into the doctor report.
func redact(out string, res *resolve.Resolution) string {
	for _, value := range res.Env {
		if len(value) >= 4 {
			out = strings.ReplaceAll(out, value, "[REDACTED]")
		}
	}
	return out
}

func firstLine(out string) string {
	line := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	if line == "" {
		return "(sem saída)"
	}
	if len([]rune(line)) > 120 {
		line = string([]rune(line)[:120]) + "…"
	}
	return line
}
