// Package resolve turns a working directory into the credentials a provider needs.
package resolve

import (
	"errors"
	"fmt"
	"strings"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/project"
	"github.com/devlefel/ranma/internal/provider"
)

// Reason distinguishes why resolution failed. Every reason is actionable:
// there is always exactly one command that fixes it.
type Reason int

const (
	ReasonNoProjectFile Reason = iota
	ReasonProviderNotDeclared
	ReasonAccountNotFound
	ReasonMissingField
)

// Error is a resolution failure. Its message never contains a credential.
type Error struct {
	Reason    Reason
	Provider  string
	Account   string
	Field     string
	Cwd       string
	Available []string
}

func (e *Error) Error() string {
	var b strings.Builder
	avail := "nenhuma cadastrada"
	if len(e.Available) > 0 {
		avail = strings.Join(e.Available, ", ")
	}

	switch e.Reason {
	case ReasonNoProjectFile:
		fmt.Fprintf(&b, "✗ ranma: nenhum %s encontrado para %s\n", project.FileName, e.Provider)
	case ReasonProviderNotDeclared:
		fmt.Fprintf(&b, "✗ ranma: projeto sem conta %s declarada\n", e.Provider)
	case ReasonAccountNotFound:
		fmt.Fprintf(&b, "✗ ranma: conta %s/%s declarada mas não cadastrada\n", e.Provider, e.Account)
	case ReasonMissingField:
		fmt.Fprintf(&b, "✗ ranma: conta %s/%s não tem o campo %q\n", e.Provider, e.Account, e.Field)
	}

	fmt.Fprintf(&b, "\n  cwd:         %s\n", e.Cwd)
	fmt.Fprintf(&b, "  disponíveis: %s\n\n", avail)

	switch e.Reason {
	case ReasonNoProjectFile, ReasonProviderNotDeclared:
		fmt.Fprintf(&b, "  Declare com:\n    ranma link %s <conta>\n", e.Provider)
	case ReasonAccountNotFound:
		fmt.Fprintf(&b, "  Cadastre com:\n    ranma add %s %s\n", e.Provider, e.Account)
	case ReasonMissingField:
		fmt.Fprintf(&b, "  Recadastre com:\n    ranma add %s %s\n", e.Provider, e.Account)
	}
	return b.String()
}

// Resolution is everything exec needs to run a command on the right account.
type Resolution struct {
	Provider *provider.Provider
	Account  string
	Env      map[string]string
	Clear    []string
}

// Resolve finds the account declared for p at cwd and renders its env vars.
func Resolve(p *provider.Provider, st *account.Store, cwd string) (*Resolution, error) {
	available := st.Accounts(p.Name)

	proj, err := project.Find(cwd)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, &Error{Reason: ReasonNoProjectFile, Provider: p.Name, Cwd: cwd, Available: available}
	}

	name, ok := proj.Use[p.Name]
	if !ok || name == "" {
		return nil, &Error{Reason: ReasonProviderNotDeclared, Provider: p.Name, Cwd: cwd, Available: available}
	}

	fields, ok := st.Get(p.Name, name)
	if !ok {
		return nil, &Error{Reason: ReasonAccountNotFound, Provider: p.Name, Account: name, Cwd: cwd, Available: available}
	}

	res, err := Render(p, name, fields)
	if err != nil {
		// Render knows the provider and the field, but not the directory it
		// was asked about; fill that in so the message stays actionable.
		var rerr *Error
		if errors.As(err, &rerr) {
			rerr.Cwd, rerr.Available = cwd, available
		}
		return nil, err
	}
	return res, nil
}

// Render substitutes an account's fields into the provider's env templates.
// It touches no filesystem: Resolve calls it after the directory lookup, and
// `doctor --verify` calls it directly, because verifying a credential is not
// a question about any project.
func Render(p *provider.Provider, accountName string, fields map[string]string) (*Resolution, error) {
	env := make(map[string]string, len(p.Env))
	for key, tmpl := range p.Env {
		rendered := tmpl
		for _, field := range p.Fields() {
			value, ok := fields[field]
			if !ok {
				return nil, &Error{
					Reason: ReasonMissingField, Provider: p.Name,
					Account: accountName, Field: field,
				}
			}
			rendered = strings.ReplaceAll(rendered, "{{"+field+"}}", value)
		}
		env[key] = rendered
	}
	return &Resolution{Provider: p, Account: accountName, Env: env, Clear: p.Clear}, nil
}
