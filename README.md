# ranma

**The right CLI account for every project — enforced, not remembered.**

`gh auth switch` is global. So is `railway login`. If you juggle a personal
account, a company account and a client account, the active one is whatever
you switched to last — and a `railway up` in the wrong directory deploys to
the wrong place. AI coding agents make this worse: they have no memory of
which account is active.

ranma makes the account a function of the directory.

```console
$ cd ~/work/client-api && railway up
# → deploys with the client account

$ cd ~/side/blog && railway up
# → deploys with your personal account

$ cd ~/scratch && railway up
✗ ranma: projeto sem conta railway declarada
  Declare com:
    ranma link railway <conta>
```

No global state is mutated. Two projects can run in parallel. An undeclared
project is blocked, never guessed.

## Install

```console
curl -sSL https://raw.githubusercontent.com/devlefel/ranma/main/install.sh | sh
```

Or `go install github.com/devlefel/ranma/cmd/ranma@latest`.

Then put the shims in front of your PATH:

```console
ranma shim install
# follow the printed export line, then restart your shell
ranma doctor
```

## Use

```console
gh auth login                      # log in with the native CLI, as usual
ranma add --import gh devlefel     # lift the credential into ranma
ranma link gh devlefel             # declare it for this project
```

`ranma link` writes `.ranma.toml`:

```toml
[use]
gh = "devlefel"
railway = "client-prod"
```

That file holds **account names only, never credentials** — commit it.

| Command | |
|---|---|
| `ranma exec <bin> [args...]` | run a provider CLI, resolving the account for this directory |
| `ranma ls [provider]` | registered accounts, credentials masked |
| `ranma whoami` | what this directory resolves to |
| `ranma link <provider> <account>` | declare the account for this project, in `.ranma.toml` |
| `ranma add [--import] <provider> <account>` | register a credential |
| `ranma rm <provider> <account>` | remove a registered credential |
| `ranma shim install\|uninstall` | install or remove the PATH interceptors |
| `ranma hook install\|uninstall` | register or remove the Claude Code hook |
| `ranma doctor` | diagnose PATH, permissions, resolution |
| `ranma doctor --verify` | additionally ask each provider whether the credential still works |
| `ranma --version` | print the installed version |

Flags come before positional arguments (`ranma add --import gh devlefel`).

## Built-in providers

`railway` · `gh` · `resend`

## Add a provider

Drop a file in `internal/provider/builtin/` and send a PR:

```toml
[vercel]
bin = "vercel"
env = { VERCEL_TOKEN = "{{token}}" }
passthrough = ["login", "logout", "help"]
verify = ["whoami"]
```

Users can also override or add providers locally in
`~/.config/ranma/providers.toml` — same format, no rebuild.

## AI agents

Point the agent at the project and it just works: the shims intercept any
invocation by name — `railway up`, whether a person or an agent typed it —
because that is how PATH lookup, and so command resolution, works. For
Claude Code, `ranma hook install` additionally surfaces the reason inline so
the agent reads the fix without burning a run. The hook only ever denies with
an explanation — it never rewrites commands, so it composes with rewrite
hooks you already have.

See [Security](#security) for what this does not cover.

## Security

- `~/.config/ranma/accounts.toml` is `0600`; ranma refuses to read it otherwise.
- A credential reaches a provider only through the child process environment at
  `exec` time — never a command line, so it never shows up in `ps` or in shell
  history. It is never logged. The most ranma will ever print is an 8-character
  prefix, so you can tell two accounts apart — `ranma ls` shows it, and `ranma
  add` echoes it back to confirm what it stored. `doctor --verify` redacts any
  longer run a provider CLI echoes back at it.
- ranma itself makes no network calls: no telemetry, no update check, no remote
  provider registry — definitions ship in the binary. The provider CLI it hands
  off to is of course still talking to its own API, and `doctor --verify` asks
  it to.

**What the shim does not cover.** Interception works by putting a shim
earlier on PATH than the real binary, so it catches any invocation resolved
by name — `railway up`, `env railway up`, `sh -c 'railway up'` all go through
PATH lookup and hit the shim. Calling the real CLI by **absolute path**
(`/usr/local/bin/railway up`) is the one thing that escapes it: PATH lookup
never happens, so the command reaches the native CLI directly and runs under
whatever account is active globally for it. `ranma doctor` cannot detect this
— nothing before `exec` time can tell an agent chose the absolute path on
purpose.

The optional Claude Code hook (`ranma hook install`) is a separate, weaker
line of defense: an inline warning that reads the Bash command *before* it
runs, so it can explain a block instead of just failing. It fails open on a
provider invoked through a wrapper, an alias, or a shell function — those
aren't attributed to the provider by its script parser, so no inline warning
fires. The shim still catches every one of those at actual `exec` time
(wrappers resolve the binary through PATH same as a direct call); only the
early warning is missed, not the block itself.

## Contributing

Most providers are an 8-line TOML file and no Go at all — see
[CONTRIBUTING.md](CONTRIBUTING.md). Bugs and provider requests go to
[Issues](https://github.com/devlefel/ranma/issues); anything that could expose a
credential goes through [SECURITY.md](SECURITY.md) instead. Participation is
covered by the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

MIT
