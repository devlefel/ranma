// Package hook implements the optional Claude Code PreToolUse integration.
// It only ever denies with an explanation — it never rewrites the command,
// so it composes with rewrite hooks the user already has installed.
package hook

import (
	"encoding/json"
	"io"
	"path/filepath"
	"strings"

	"mvdan.cc/sh/v3/syntax"

	"github.com/devlefel/ranma/internal/account"
	"github.com/devlefel/ranma/internal/provider"
	"github.com/devlefel/ranma/internal/resolve"
)

// Command is one call site found inside a Bash script.
type Command struct {
	Bin  string
	Args []string
}

// Commands lexes a shell script and returns every command invocation in it,
// so `echo railway` is never mistaken for running railway.
func Commands(script string) ([]Command, error) {
	file, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		return nil, err
	}

	var out []Command
	syntax.Walk(file, func(node syntax.Node) bool {
		call, ok := node.(*syntax.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		name := call.Args[0].Lit()
		if name == "" {
			return true
		}
		cmd := Command{Bin: filepath.Base(name)}
		for _, arg := range call.Args[1:] {
			cmd.Args = append(cmd.Args, arg.Lit())
		}
		out = append(out, cmd)
		return true
	})
	return out, nil
}

// Decide reports whether the script must be denied, and why. Anything it
// cannot understand is allowed: the shims are the actual guarantee, and a
// hook that guesses wrong is worse than a hook that stays quiet.
func Decide(reg *provider.Registry, st *account.Store, script, cwd string) (string, bool) {
	cmds, err := Commands(script)
	if err != nil {
		return "", false
	}

	for _, cmd := range cmds {
		p, known := reg.ByBin(cmd.Bin)
		if !known || p.IsPassthrough(cmd.Args) {
			continue
		}
		if _, err := resolve.Resolve(p, st, cwd); err != nil {
			return err.Error(), true
		}
	}
	return "", false
}

type input struct {
	Cwd       string `json:"cwd"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type output struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// Run reads one PreToolUse payload and writes a deny decision, or nothing.
func Run(r io.Reader, w io.Writer, reg *provider.Registry, st *account.Store) error {
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		// A payload we cannot parse is no reason to block the agent.
		return nil
	}
	if in.ToolInput.Command == "" || in.Cwd == "" {
		return nil
	}

	reason, deny := Decide(reg, st, in.ToolInput.Command, in.Cwd)
	if !deny {
		return nil
	}

	var out output
	out.HookSpecificOutput.HookEventName = "PreToolUse"
	out.HookSpecificOutput.PermissionDecision = "deny"
	out.HookSpecificOutput.PermissionDecisionReason = reason

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
