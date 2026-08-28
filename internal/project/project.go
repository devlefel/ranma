// Package project reads the per-project .ranma.toml declaration.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the per-project declaration file.
const FileName = ".ranma.toml"

// Project is one .ranma.toml: which account each provider should use.
type Project struct {
	Path string            `toml:"-"`
	Use  map[string]string `toml:"use"`
}

// Find walks up from startDir looking for FileName. The nearest file wins
// outright — declarations are never merged across directory levels, so what
// a project declares is exactly what you read in its own file.
// A missing file is not an error: it returns (nil, nil).
func Find(startDir string) (*Project, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		candidate := filepath.Join(dir, FileName)
		if _, err := os.Stat(candidate); err == nil {
			return Read(candidate)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

// Read parses one .ranma.toml.
func Read(path string) (*Project, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := &Project{Path: path, Use: map[string]string{}}
	if _, err := toml.Decode(string(raw), p); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if p.Use == nil {
		p.Use = map[string]string{}
	}
	p.Path = path
	return p, nil
}

// Write renders .ranma.toml into dir. Output is sorted so the file stays
// diff-stable when it is committed.
func Write(dir string, use map[string]string) error {
	var b strings.Builder
	b.WriteString("[use]\n")
	for _, name := range slices.Sorted(maps.Keys(use)) {
		fmt.Fprintf(&b, "%s = %q\n", name, use[name])
	}
	return os.WriteFile(filepath.Join(dir, FileName), []byte(b.String()), 0o644)
}
