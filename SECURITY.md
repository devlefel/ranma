# Security Policy

ranma holds credentials for other services. A defect here can expose an account
that has nothing to do with this repository, so security reports get priority
over everything else.

## Reporting a vulnerability

Use GitHub's [private vulnerability reporting](https://github.com/devlefel/ranma/security/advisories/new).
**Do not open a public issue** — the report itself often describes how to extract
a credential.

Include what you did, what leaked or ran, and the version (`ranma --version`).
A proof of concept helps; a real token does not — redact it.

Supported: the latest release. There are no backports yet.

## What counts as a vulnerability

Anything that breaks one of these:

| Guarantee | Broken if |
|---|---|
| A credential never reaches stdout, stderr, a log, or a command line | any output path prints more than the 8-character prefix `ranma ls` shows |
| A command runs on the declared account, or does not run | some input makes ranma inject a different account's credential |
| `accounts.toml` stays `0600` | ranma writes or accepts it more permissively |
| A shim never executes itself | some `PATH` shape makes `ranma exec` recurse |

## Known limits — not vulnerabilities

These are documented on purpose. Reporting them is welcome as an improvement,
but they are not treated as disclosures:

- **Absolute-path invocation bypasses the shim.** `/usr/local/bin/railway up` does
  not go through `PATH`, so nothing intercepts it. A shim guards the name, not the
  inode.
- **The Claude Code hook fails open on wrappers.** `env railway up`, `sh -c "…"`,
  aliases and function bodies are not attributed to a provider by the lexer. The
  shim still catches them; the hook is a second, weaker line.
- **Encoded credentials escape redaction.** `doctor --verify` redacts a credential
  a provider CLI echoes back, including truncated prefixes and case changes — but
  a token printed base64-encoded or url-encoded matches no prefix and passes.
- **`accounts.toml` is plaintext at `0600`.** That is the same posture the native
  CLIs already have. Encrypting it was considered and rejected: unlocking on every
  invocation defeats the point of a tool meant to be invisible.
