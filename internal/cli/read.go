// Package cli implements the ranma subcommands on top of the core packages.
package cli

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/project"
	"github.com/devlefel/ranma/internal/provider"
	"github.com/devlefel/ranma/internal/resolve"
)

// List prints the registered accounts, credentials masked.
func List(w io.Writer, reg *provider.Registry, st *account.Store, filter string) error {
	if filter != "" {
		if _, ok := reg.ByName(filter); !ok {
			return unknownProvider(reg, filter)
		}
	}

	providers := st.Providers()
	if filter != "" {
		providers = slices.DeleteFunc(providers, func(n string) bool { return n != filter })
	}
	if len(providers) == 0 {
		fmt.Fprintln(w, "Nenhuma conta cadastrada. Comece com: ranma add <provider> <conta>")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, name := range providers {
		for _, acct := range st.Accounts(name) {
			fields, _ := st.Get(name, acct)
			var parts []string
			for _, field := range slices.Sorted(maps.Keys(fields)) {
				parts = append(parts, field+"="+account.Mask(fields[field]))
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\n", name, acct, strings.Join(parts, " "))
		}
	}
	return tw.Flush()
}

// Whoami reports, per provider, what this directory would resolve to.
func Whoami(w io.Writer, reg *provider.Registry, st *account.Store, cwd string) error {
	proj, err := project.Find(cwd)
	if err != nil {
		return err
	}
	if proj == nil {
		fmt.Fprintf(w, "cwd:      %s\ndeclarado em: (nenhum %s encontrado)\n\n", cwd, project.FileName)
	} else {
		fmt.Fprintf(w, "cwd:      %s\ndeclarado em: %s\n\n", cwd, proj.Path)
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, name := range reg.Names() {
		p, _ := reg.ByName(name)
		res, err := resolve.Resolve(p, st, cwd)
		if err == nil {
			fmt.Fprintf(tw, "%s\t→ %s\n", name, res.Account)
			continue
		}

		var rerr *resolve.Error
		if !errors.As(err, &rerr) {
			tw.Flush()
			return err
		}
		switch rerr.Reason {
		case resolve.ReasonNoProjectFile, resolve.ReasonProviderNotDeclared:
			fmt.Fprintf(tw, "%s\t✗ não declarado\n", name)
		case resolve.ReasonAccountNotFound:
			fmt.Fprintf(tw, "%s\t✗ conta %q não cadastrada\n", name, rerr.Account)
		case resolve.ReasonMissingField:
			fmt.Fprintf(tw, "%s\t✗ conta %q sem o campo %q\n", name, rerr.Account, rerr.Field)
		}
	}
	return tw.Flush()
}

// Link declares providerName → accountName in cwd's .ranma.toml, merging
// with whatever that same directory already declares.
func Link(w io.Writer, reg *provider.Registry, st *account.Store, cwd, providerName, accountName string) error {
	if _, ok := reg.ByName(providerName); !ok {
		return unknownProvider(reg, providerName)
	}
	if _, ok := st.Get(providerName, accountName); !ok {
		available := "nenhuma"
		if names := st.Accounts(providerName); len(names) > 0 {
			available = strings.Join(names, ", ")
		}
		return fmt.Errorf(
			"ranma: conta %s/%s não cadastrada (disponíveis: %s)\n  Cadastre com: ranma add %s %s",
			providerName, accountName, available, providerName, accountName)
	}

	path := filepath.Join(cwd, project.FileName)
	use := map[string]string{}
	existing, err := project.Read(path)
	switch {
	case err == nil:
		use = existing.Use
	case !errors.Is(err, fs.ErrNotExist):
		// Never overwrite a file we could not read. It is meant to be
		// committed and may carry another provider's declaration; losing
		// that silently is worse than refusing to link.
		return fmt.Errorf("ranma: %s existe mas não pôde ser lido: %w", path, err)
	}
	use[providerName] = accountName

	if err := project.Write(cwd, use); err != nil {
		return err
	}
	fmt.Fprintf(w, "✓ %s → %s (%s)\n", providerName, accountName, path)
	return nil
}

func unknownProvider(reg *provider.Registry, name string) error {
	return fmt.Errorf("ranma: provider desconhecido %q (conhecidos: %s)",
		name, strings.Join(reg.Names(), ", "))
}
