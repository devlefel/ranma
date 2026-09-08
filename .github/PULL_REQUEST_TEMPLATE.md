## What and why

<!-- One or two lines. The diff shows what changed; say why it should. -->

## Checklist

- [ ] `go test ./... -race`, `go vet ./...` and `gofmt -l .` are clean
- [ ] Behaviour change ships with a test that fails without the fix
- [ ] No new dependency (or the PR explains why one is unavoidable)
- [ ] No credential can reach stdout, stderr, a log, or a command line
- [ ] User-facing messages in Portuguese; code and comments in English

## Adding a provider?

- [ ] `bin` does not collide with an existing provider
- [ ] `clear` lists the env vars that would otherwise override the injected one
- [ ] Case added to `TestLoadBuiltins`
