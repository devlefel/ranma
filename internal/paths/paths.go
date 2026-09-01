// Package paths centralises every location ranma reads or writes.
package paths

import (
	"os"
	"path/filepath"
)

func home() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return os.Getenv("HOME")
}

// ConfigDir is ~/.config/ranma, honouring XDG_CONFIG_HOME.
func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "ranma")
	}
	return filepath.Join(home(), ".config", "ranma")
}

// AccountsFile holds credentials. Mode 0600, always.
func AccountsFile() string { return filepath.Join(ConfigDir(), "accounts.toml") }

// ProvidersFile is the optional user override of the builtin providers.
func ProvidersFile() string { return filepath.Join(ConfigDir(), "providers.toml") }

// ShimDir is where the PATH interceptors live.
func ShimDir() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "ranma", "bin")
	}
	return filepath.Join(home(), ".local", "share", "ranma", "bin")
}
