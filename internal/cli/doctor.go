package cli

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

	// Note marks an informational line: there is nothing to fix. A provider
	// this project does not use, or does not have installed, is normal.
	Note bool
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
			// Not having a provider's CLI installed is the normal case for a
			// machine that never touches it: nothing here needs fixing.
			checks = append(checks, Check{
				Name:   name + ": binário",
				Detail: fmt.Sprintf("%s não está instalado; o ranma não tem o que interceptar", p.Bin),
				Note:   true,
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
		note := false
		if errors.As(err, &rerr) {
			switch rerr.Reason {
			case resolve.ReasonNoProjectFile:
				// No .ranma.toml at all is the default state of most
				// directories, not a defect to flag.
				detail = "nenhum " + project.FileName + " encontrado a partir deste diretório"
				note = true
			case resolve.ReasonProviderNotDeclared:
				// A project that simply does not use this provider is the
				// normal case, not a failure.
				detail = "não declarado neste projeto"
				note = true
			case resolve.ReasonAccountNotFound:
				detail = fmt.Sprintf("conta %q declarada mas não cadastrada", rerr.Account)
			case resolve.ReasonMissingField:
				detail = fmt.Sprintf("conta %q sem o campo %q", rerr.Account, rerr.Field)
			}
		}
		checks = append(checks, Check{Name: name, Detail: detail, Note: note})
	}

	checks = append(checks, checkUnknownProviders(reg, cwd)...)

	return checks
}

// checkUnknownProviders flags every key in the project's .ranma.toml that
// names no provider in the registry — a typo like "railwey" instead of
// "railway" — pointing at the exact file and the offending key so it is not
// left as silent "não declarado" noise.
func checkUnknownProviders(reg *provider.Registry, cwd string) []Check {
	proj, err := project.Find(cwd)
	if err != nil || proj == nil {
		return nil
	}

	var checks []Check
	for _, key := range slices.Sorted(maps.Keys(proj.Use)) {
		if _, ok := reg.ByName(key); ok {
			continue
		}
		checks = append(checks, Check{
			Name: key + ": provider desconhecido",
			Detail: fmt.Sprintf(
				"%s declara %q, que não é um provider conhecido; providers conhecidos: %s",
				proj.Path, key, strings.Join(reg.Names(), ", ")),
		})
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
// A Note never fails the run: ✗ and the exit code are reserved for what the
// user actually needs to fix.
func PrintChecks(w io.Writer, checks []Check) bool {
	all := true
	for _, c := range checks {
		mark := "✓"
		switch {
		case c.Note:
			mark = "·"
		case !c.OK:
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
			safe := redact(string(out), res, fields)
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

// minRedact is how much of a secret may never reach the report. It matches
// what account.Mask already shows, so `doctor --verify` can never print more
// of a credential than `ranma ls` does.
const minRedact = 8

// redact removes every injected secret from a provider CLI's output. A CLI
// that echoes its own credential — a verbose auth error, a debug flag — must
// never leak it into the doctor report.
//
// Matching is deliberately generous, because an exact whole-value match is not
// how credentials actually escape: CLIs print a truncated prefix, uppercase the
// value, or wrap it across lines. So any run matching a leading portion of a
// secret, at least minRedact bytes long, is replaced, case-insensitively.
//
// The rendered env values are not the only secret shape: a template can wrap
// a raw account field in surrounding text (env = { AUTHZ = "Bearer
// {{token}}" }), and a CLI that strips that wrapper before echoing leaks the
// raw field, which never appears as a prefix of the rendered value. So the
// raw account fields are redacted too, not just the rendered env.
//
// Values are redacted longest first, so a long secret is never left partially
// exposed because a shorter one (e.g. a field also equal to a prefix of it)
// consumed the match first.
//
// What this does not defend against: a credential the CLI prints encoded —
// base64, URL-encoded, JSON-escaped — matches no prefix of the raw secret and
// passes through untouched. Redaction here is prefix matching on the raw
// bytes, not a decoder for every encoding a CLI might choose.
func redact(out string, res *resolve.Resolution, fields map[string]string) string {
	values := make([]string, 0, len(res.Env)+len(fields))
	for _, value := range res.Env {
		values = append(values, value)
	}
	for _, value := range fields {
		values = append(values, value)
	}
	slices.SortFunc(values, func(a, b string) int {
		return len(b) - len(a)
	})
	for _, value := range values {
		if len(value) < minRedact {
			continue
		}
		out = redactPrefixes(out, value)
	}
	return out
}

// redactPrefixes replaces every run in out matching a leading portion of
// secret, at least minRedact bytes long.
func redactPrefixes(out, secret string) string {
	var b strings.Builder
	for i := 0; i < len(out); {
		n := 0
		for n < len(secret) && i+n < len(out) && foldEqual(out[i+n], secret[n]) {
			n++
		}
		if n >= minRedact {
			b.WriteString("[REDACTED]")
			i += n
			continue
		}
		b.WriteByte(out[i])
		i++
	}
	return b.String()
}

// foldEqual compares two bytes ignoring ASCII case.
func foldEqual(a, b byte) bool {
	if 'A' <= a && a <= 'Z' {
		a += 'a' - 'A'
	}
	if 'A' <= b && b <= 'Z' {
		b += 'a' - 'A'
	}
	return a == b
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
