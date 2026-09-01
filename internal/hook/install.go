package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InstallSettings registers `ranma hook` as a PreToolUse Bash hook in the
// given Claude Code settings.json, preserving every other setting and hook.
func InstallSettings(path, ranmaBin string) error {
	settings, err := readSettings(path)
	if err != nil {
		return err
	}
	if err := backup(path); err != nil {
		return err
	}

	entry := map[string]any{"type": "command", "command": ranmaBin + " hook"}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	pre, _ := hooks["PreToolUse"].([]any)

	matched := false
	for _, raw := range pre {
		matcher, ok := raw.(map[string]any)
		if !ok || matcher["matcher"] != "Bash" {
			continue
		}
		matched = true
		list, _ := matcher["hooks"].([]any)
		if hasRanma(list, ranmaBin) {
			return writeSettings(path, settings)
		}
		matcher["hooks"] = append(list, entry)
	}
	if !matched {
		pre = append(pre, map[string]any{
			"matcher": "Bash",
			"hooks":   []any{entry},
		})
	}

	hooks["PreToolUse"] = pre
	settings["hooks"] = hooks
	return writeSettings(path, settings)
}

// UninstallSettings removes only ranma's hook entries.
func UninstallSettings(path string) error {
	settings, err := readSettings(path)
	if err != nil {
		return err
	}
	if err := backup(path); err != nil {
		return err
	}

	hooks, _ := settings["hooks"].(map[string]any)
	pre, _ := hooks["PreToolUse"].([]any)
	for _, raw := range pre {
		matcher, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		list, _ := matcher["hooks"].([]any)
		kept := make([]any, 0, len(list))
		for _, h := range list {
			entry, ok := h.(map[string]any)
			if ok {
				if cmd, _ := entry["command"].(string); strings.HasSuffix(cmd, " hook") &&
					strings.Contains(cmd, "ranma") {
					continue
				}
			}
			kept = append(kept, h)
		}
		matcher["hooks"] = kept
	}
	return writeSettings(path, settings)
}

func hasRanma(list []any, ranmaBin string) bool {
	for _, h := range list {
		entry, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if cmd, _ := entry["command"].(string); cmd == ranmaBin+" hook" {
			return true
		}
	}
	return false
}

func readSettings(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	settings := map[string]any{}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return settings, nil
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return nil, fmt.Errorf("ranma: %s não é um JSON válido; corrija antes de instalar o hook", path)
	}
	return settings, nil
}

func backup(path string) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path+".bak", raw, 0o600)
}

func writeSettings(path string, settings map[string]any) error {
	raw, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	// A fresh machine may not have ~/.claude yet: installing the hook is a
	// reasonable first thing to do, and must not fail for a missing directory.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
