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

### When the provider needs `--import`

`import` lets `ranma add --import` lift a credential out of the native CLI's own
config file, so nobody has to paste a token. It is the one contribution that
needs Go, because every CLI stores its credential differently.

Declare where the file lives and which reader parses it:

```toml
import = { kind = "vercel-auth", path = ".local/share/com.vercel.cli/auth.json" }
```

`path` is relative to `$HOME`. Then implement `kind` in `internal/importer`:

1. Add a `case "vercel-auth":` to the switch in `Import`.
2. Write the reader. It receives the file bytes, the path (for error messages)
   and the account name, and returns the account fields — the same field names
   the provider's `env` templates reference.
3. Return `map[string]string{"token": …}` on success.

Two rules the existing readers follow, both enforced by tests:

- **Never put file content in an error.** A malformed config may contain the very
  credential you are reading. Say `"%s não é um JSON válido"` with the path, not
  the parse error from the library.
- **When the format holds several accounts** (`gh` does), select by `accountName`
  and, if it is missing, list the **names** available — never the tokens.

Copy `importGhHosts` for a multi-account format, `importRailway` for a
single-token one. Add tests for the happy path, the missing file, the unknown
account, and the empty credential.

## Commits and branches

Conventional commits, scope is the package or the area:

```
feat(shim): interceptadores de PATH com cobertura end-to-end
fix(doctor): redige credencial ecoada em --verify
docs(security): escopa a promessa de rede
```

Branches: `feat/<assunto>`, `fix/<assunto>`, `docs/<assunto>`. Open the PR against
`master`; CI runs `go vet`, `go test -race` and `gofmt` on linux and macos.

## Rules

- No new dependencies without discussion. This tool handles credentials.
- A credential must never reach stdout, stderr, a log, or a command line.
- Every behaviour change ships with a test. `go test ./...` must pass.
- User-facing messages are in Portuguese; code and comments in English.
- Behaviour that protects a credential ships with a test that **fails without the
  fix**. Several of the tests in this repo exist because a reviewer reproduced a
  leak first; that is the standard.

## Running the tests

```console
go test ./...
```

Every test runs in a temporary `HOME` — nothing touches your real config.

## Reporting problems

Bugs and provider requests go to [Issues](https://github.com/devlefel/ranma/issues).
Anything that could expose a credential goes through [SECURITY.md](SECURITY.md)
instead — please do not open a public issue for it.
