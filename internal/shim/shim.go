// Package shim generates the PATH interceptors that route provider CLIs
// through ranma, whatever tool or shell invokes them.
package shim

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MarkerFile proves a directory is ranma's to manage, so Uninstall can never
// delete a directory a user pointed at by mistake.
const MarkerFile = ".ranma-shims"

const template = `#!/bin/sh
# Gerado por ranma. Não edite: ` + "`ranma shim install`" + ` regenera este arquivo.
exec %q exec %s "$@"
`

// Install regenerates dir with one shim per binary. It replaces the whole
// directory so a provider you removed does not leave a stale shim behind.
func Install(dir, ranmaBin string, bins []string) error {
	// Uninstall refuses directories ranma does not manage, so this both
	// clears stale shims and protects a directory the user chose by mistake.
	if err := Uninstall(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, MarkerFile), []byte("managed by ranma\n"), 0o644); err != nil {
		return err
	}
	for _, bin := range bins {
		body := fmt.Sprintf(template, ranmaBin, bin)
		if err := os.WriteFile(filepath.Join(dir, bin), []byte(body), 0o755); err != nil {
			return err
		}
	}
	return nil
}

// Uninstall removes the shim directory, but only if ranma created it.
func Uninstall(dir string) error {
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, MarkerFile)); err != nil {
		return fmt.Errorf(
			"ranma: %s não tem %s e não foi criado pelo ranma; remova manualmente se for o caso",
			dir, MarkerFile)
	}
	return os.RemoveAll(dir)
}

// PathHint is the line the operator must add to their shell profile.
func PathHint(dir string) string {
	return fmt.Sprintf("export PATH=%q:$PATH", dir)
}
