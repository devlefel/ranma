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
| `ranma ls` | registered accounts, credentials masked |
| `ranma whoami` | what this directory resolves to |
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

Point the agent at the project and it just works: the shims intercept every
CLI call regardless of which tool made it. For Claude Code, `ranma hook install`
additionally surfaces the reason inline so the agent reads the fix without
burning a run. The hook only ever denies with an explanation — it never
rewrites commands, so it composes with rewrite hooks you already have.

## Security

- `~/.config/ranma/accounts.toml` is `0600`; ranma refuses to read it otherwise.
- Credentials are never printed, logged, or passed on a command line — only
  through the child process environment at `exec` time.
- No network calls, ever. Provider definitions ship in the binary.

## License

MIT
