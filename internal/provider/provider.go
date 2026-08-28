// Package provider carries the definitions of the CLIs ranma intercepts.
package provider

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed builtin/*.toml
var builtinFS embed.FS

// ImportSpec says where a provider's native CLI keeps the credential.
type ImportSpec struct {
	Kind string `toml:"kind"`
	Path string `toml:"path"`
}

// Provider is one interceptable CLI.
type Provider struct {
	Name        string            `toml:"-"`
	Bin         string            `toml:"bin"`
	Env         map[string]string `toml:"env"`
	Clear       []string          `toml:"clear"`
	Passthrough []string          `toml:"passthrough"`
	Verify      []string          `toml:"verify"`
	Import      *ImportSpec       `toml:"import"`
}

var fieldRe = regexp.MustCompile(`\{\{([a-z0-9_]+)\}\}`)

// Fields lists the account fields the env templates reference, sorted.
func (p *Provider) Fields() []string {
	set := map[string]bool{}
	for _, tmpl := range p.Env {
		for _, m := range fieldRe.FindAllStringSubmatch(tmpl, -1) {
			set[m[1]] = true
		}
	}
	return slices.Sorted(maps.Keys(set))
}

// IsPassthrough reports whether these args run without account resolution.
func (p *Provider) IsPassthrough(args []string) bool {
	if len(args) == 0 {
		return true
	}
	return slices.Contains(p.Passthrough, args[0])
}

// Registry holds every known provider, indexed by name and by binary.
type Registry struct {
	byName map[string]*Provider
	byBin  map[string]*Provider
}

// Load reads the embedded providers, then overlays userPath if it exists.
// A user block replaces the builtin block for that name wholesale.
func Load(userPath string) (*Registry, error) {
	defs := map[string]*Provider{}

	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		raw, err := builtinFS.ReadFile("builtin/" + e.Name())
		if err != nil {
			return nil, err
		}
		if err := decodeInto(string(raw), defs); err != nil {
			return nil, fmt.Errorf("provider embutido %s: %w", e.Name(), err)
		}
	}

	if userPath != "" {
		raw, err := os.ReadFile(userPath)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// Sem arquivo do usuário: só os embutidos.
		case err != nil:
			return nil, err
		default:
			if err := decodeInto(string(raw), defs); err != nil {
				return nil, fmt.Errorf("%s: %w", userPath, err)
			}
		}
	}

	reg := &Registry{byName: map[string]*Provider{}, byBin: map[string]*Provider{}}
	for name, p := range defs {
		p.Name = name
		if p.Bin == "" {
			return nil, fmt.Errorf("provider %q: campo bin obrigatório", name)
		}
		if len(p.Env) == 0 {
			return nil, fmt.Errorf("provider %q: campo env obrigatório", name)
		}
		reg.byName[name] = p
		reg.byBin[p.Bin] = p
	}
	return reg, nil
}

func decodeInto(body string, defs map[string]*Provider) error {
	parsed := map[string]*Provider{}
	if _, err := toml.Decode(body, &parsed); err != nil {
		return err
	}
	maps.Copy(defs, parsed)
	return nil
}

// ByName looks a provider up by its registry key.
func (r *Registry) ByName(name string) (*Provider, bool) {
	p, ok := r.byName[name]
	return p, ok
}

// ByBin looks a provider up by the executable it intercepts.
func (r *Registry) ByBin(bin string) (*Provider, bool) {
	p, ok := r.byBin[strings.TrimSpace(bin)]
	return p, ok
}

// Names lists provider names, sorted.
func (r *Registry) Names() []string {
	return slices.Sorted(maps.Keys(r.byName))
}

// Bins lists intercepted executables, sorted.
func (r *Registry) Bins() []string {
	return slices.Sorted(maps.Keys(r.byBin))
}
