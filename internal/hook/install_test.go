package hook_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/hook"
)

func TestInstallSettingsPreservesExistingHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := `{
  "statusLine": {"type": "command", "command": "meu-status.sh"},
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "/home/x/.claude/hooks/rtk-rewrite.sh"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := hook.InstallSettings(path, "/usr/local/bin/ranma"); err != nil {
		t.Fatalf("InstallSettings: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := string(raw)
	if !strings.Contains(body, "rtk-rewrite.sh") {
		t.Error("o hook existente foi apagado")
	}
	if !strings.Contains(body, "/usr/local/bin/ranma") {
		t.Error("o hook do ranma não foi adicionado")
	}
	if !strings.Contains(body, "meu-status.sh") {
		t.Error("configurações não relacionadas foram perdidas")
	}

	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("settings.json ficou inválido: %v", err)
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("backup ausente: %v", err)
	}
}

func TestInstallSettingsIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if err := hook.InstallSettings(path, "/x/ranma"); err != nil {
			t.Fatalf("InstallSettings #%d: %v", i, err)
		}
	}

	raw, _ := os.ReadFile(path)
	if n := strings.Count(string(raw), "/x/ranma"); n != 1 {
		t.Errorf("hook do ranma aparece %d vezes, quero 1", n)
	}
}

func TestUninstallSettingsRemovesOnlyRanma(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"rtk.sh"}]}]}}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := hook.InstallSettings(path, "/x/ranma"); err != nil {
		t.Fatal(err)
	}

	if err := hook.UninstallSettings(path); err != nil {
		t.Fatalf("UninstallSettings: %v", err)
	}

	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "/x/ranma") {
		t.Error("hook do ranma continua registrado")
	}
	if !strings.Contains(string(raw), "rtk.sh") {
		t.Error("hook alheio foi removido junto")
	}
}
