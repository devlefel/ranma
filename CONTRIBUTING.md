# Contributing

## Adding a provider

Most contributions are one file. Create `internal/provider/builtin/<name>.toml`:

```toml
[<name>]
bin = "<executable>"
env = { <ENV_VAR> = "{{token}}" }
clear = ["<ENV_VAR_THAT_WOULD_OVERRIDE_IT>"]
passthrough = ["login", "logout", "help"]
verify = ["whoami"]
```

| Field | Meaning |
|---|---|
| `bin` | executable ranma intercepts; must be unique across every provider, builtin and user-defined — ranma refuses to load if two collide |
| `env` | env vars to inject; `{{field}}` reads a field from the account |
| `clear` | env vars removed before exec, so an inherited token cannot win |
| `passthrough` | subcommands that run without account resolution |
| `verify` | args `ranma doctor` uses to validate a credential |
| `import` | optional `{ kind, path }` to lift the credential from the native CLI's own config; `kind` must be implemented in `internal/importer` |

Add a case to `TestLoadBuiltins` and open the PR.

## Rules

- No new dependencies without discussion. This tool handles credentials.
- A credential must never reach stdout, stderr, a log, or a command line.
- Every behaviour change ships with a test. `go test ./...` must pass.
- User-facing messages are in Portuguese; code and comments in English.

## Running the tests

```console
go test ./...
```

Every test runs in a temporary `HOME` — nothing touches your real config.
