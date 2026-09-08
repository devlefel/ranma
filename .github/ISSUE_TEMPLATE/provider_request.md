---
name: Provider request
about: Ask for a CLI ranma should intercept
labels: provider
---

**Which CLI**, and a link to its docs.

**How it authenticates** — which env var does it read for a token, and does that
env var win over whatever the CLI stored when you logged in? The whole design
rests on that precedence; if the CLI ignores the env var, ranma cannot help it
without a different strategy.

```console
$ <cli> whoami
$ SOME_TOKEN=invalid <cli> whoami   # should fail, proving the env var wins
```

**Where it stores the credential after login** — the config file path, if you
know it. That is what `--import` reads so nobody has to paste a token.

**Subcommands that must run without an account** — `login`, `help`, `version`.

Most providers are an 8-line TOML file and no Go at all — see
[CONTRIBUTING.md](../CONTRIBUTING.md) if you would rather send the PR yourself.
