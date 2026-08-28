// Package account persists provider credentials in ~/.config/ranma/accounts.toml.
package account

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/BurntSushi/toml"
)

// Store is the in-memory view of accounts.toml: provider → account → field → value.
type Store struct {
	path string
	data map[string]map[string]map[string]string
}

// Load reads accounts.toml. A missing file yields an empty store; a file
// readable by anyone but the owner is refused outright.
func Load(path string) (*Store, error) {
	st := &Store{path: path, data: map[string]map[string]map[string]string{}}

	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	if perm := fi.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf(
			"ranma: %s tem permissão %04o e guarda credenciais; corrija com: chmod 600 %s",
			path, perm, path)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if _, err := toml.Decode(string(raw), &st.data); err != nil {
		// Never wrap file content into the error message.
		return nil, fmt.Errorf("ranma: %s não é um TOML válido", path)
	}
	return st, nil
}

// Get returns the fields of one account.
func (s *Store) Get(provider, name string) (map[string]string, bool) {
	fields, ok := s.data[provider][name]
	return fields, ok
}

// Set stores or replaces one account's fields.
func (s *Store) Set(provider, name string, fields map[string]string) {
	if s.data[provider] == nil {
		s.data[provider] = map[string]map[string]string{}
	}
	s.data[provider][name] = fields
}

// Remove drops one account, reporting whether it existed. Providers left
// without accounts are dropped too, so listings stay honest.
func (s *Store) Remove(provider, name string) bool {
	if _, ok := s.data[provider][name]; !ok {
		return false
	}
	delete(s.data[provider], name)
	if len(s.data[provider]) == 0 {
		delete(s.data, provider)
	}
	return true
}

// Accounts lists the account names of one provider, sorted.
func (s *Store) Accounts(provider string) []string {
	return slices.Sorted(maps.Keys(s.data[provider]))
}

// Providers lists providers that have at least one account, sorted.
func (s *Store) Providers() []string {
	return slices.Sorted(maps.Keys(s.data))
}

// Save writes the store atomically with mode 0600.
func (s *Store) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".accounts-*.toml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if err := toml.NewEncoder(tmp).Encode(s.data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path)
}

// Mask renders a credential safe to print.
func Mask(v string) string {
	const keep = 8
	if len([]rune(v)) <= keep {
		return "…"
	}
	return string([]rune(v)[:keep]) + "…"
}
