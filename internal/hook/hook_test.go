package hook_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/hook"
	"github.com/devlefel/ranma/internal/project"
	"github.com/devlefel/ranma/internal/provider"
)

func fixture(t *testing.T, accounts string) (*provider.Registry, *account.Store) {
	t.Helper()
	reg, err := provider.Load("")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "accounts.toml")
	if err := os.WriteFile(path, []byte(accounts), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := account.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return reg, st
}

func TestCommandsFindsEveryCallSite(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   []string
	}{
		{"simples", "railway up", []string{"railway"}},
		{"encadeado", "cd /tmp && railway up", []string{"cd", "railway"}},
		{"pipe", "gh pr list | head -5", []string{"gh", "head"}},
		{"subshell", "(cd x; railway status)", []string{"cd", "railway"}},
		{"prefixo de env", "FOO=1 railway up", []string{"railway"}},
		{"caminho absoluto vira base", "/usr/bin/railway up", []string{"railway"}},
		{"argumento não é comando", "echo railway", []string{"echo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmds, err := hook.Commands(tc.script)
			if err != nil {
				t.Fatalf("Commands: %v", err)
			}
			var got []string
			for _, c := range cmds {
				got = append(got, c.Bin)
			}
			for _, want := range tc.want {
				if !contains(got, want) {
					t.Errorf("Commands(%q) = %v, quero conter %q", tc.script, got, want)
				}
			}
		})
	}
}

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

func TestCommandsKeepsArgsForPassthrough(t *testing.T) {
	cmds, err := hook.Commands("railway login")
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 1 || cmds[0].Bin != "railway" {
		t.Fatalf("Commands = %+v", cmds)
	}
	if len(cmds[0].Args) != 1 || cmds[0].Args[0] != "login" {
		t.Errorf("Args = %v, quero [login]", cmds[0].Args)
	}
}

func TestDecide(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_SEGREDO\"\n")

	declared := t.TempDir()
	if err := project.Write(declared, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}
	bare := t.TempDir()

	cases := []struct {
		name     string
		script   string
		cwd      string
		wantDeny bool
		wantMsg  string
	}{
		{"declarado passa", "railway up", declared, false, ""},
		{"não declarado nega", "railway up", bare, true, "ranma link railway"},
		{"passthrough passa", "railway login", bare, false, ""},
		{"provider desconhecido passa", "ls -la", bare, false, ""},
		{"menção em argumento passa", "echo railway up", bare, false, ""},
		{"script inválido passa", "railway up && (", bare, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reason, deny := hook.Decide(reg, st, tc.script, tc.cwd)
			if deny != tc.wantDeny {
				t.Fatalf("deny = %v, quero %v (motivo: %s)", deny, tc.wantDeny, reason)
			}
			if tc.wantMsg != "" && !strings.Contains(reason, tc.wantMsg) {
				t.Errorf("motivo não contém %q:\n%s", tc.wantMsg, reason)
			}
			if strings.Contains(reason, "rw_SEGREDO") {
				t.Error("o motivo vazou a credencial")
			}
		})
	}
}

func TestRunEmitsDenyJSON(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_SEGREDO\"\n")
	cwd := t.TempDir()

	in := strings.NewReader(`{"cwd":"` + cwd + `","tool_input":{"command":"railway up"}}`)
	var out bytes.Buffer
	if err := hook.Run(in, &out, reg, st); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var payload struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("saída não é JSON válido: %v\n%s", err, out.String())
	}
	if payload.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q", payload.HookSpecificOutput.HookEventName)
	}
	if payload.HookSpecificOutput.PermissionDecision != "deny" {
		t.Errorf("permissionDecision = %q, quero deny", payload.HookSpecificOutput.PermissionDecision)
	}
	if !strings.Contains(payload.HookSpecificOutput.PermissionDecisionReason, "ranma link railway") {
		t.Errorf("motivo sem instrução:\n%s", payload.HookSpecificOutput.PermissionDecisionReason)
	}
	if strings.Contains(out.String(), "updatedInput") {
		t.Error("o hook não pode emitir updatedInput: isso conflita com hooks de reescrita")
	}
}

func TestRunStaysSilentWhenAllowed(t *testing.T) {
	reg, st := fixture(t, "[railway.lefel]\ntoken = \"rw_x\"\n")
	cwd := t.TempDir()
	if err := project.Write(cwd, map[string]string{"railway": "lefel"}); err != nil {
		t.Fatal(err)
	}

	in := strings.NewReader(`{"cwd":"` + cwd + `","tool_input":{"command":"railway up"}}`)
	var out bytes.Buffer
	if err := hook.Run(in, &out, reg, st); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("quero saída vazia quando permitido, deu:\n%s", out.String())
	}
}
