---
name: Bug report
about: Something ranma did that it should not have
labels: bug
---

**What happened, and what should have happened instead**

**Steps to reproduce**

**Output of `ranma doctor`** — it reports PATH, permissions and what the current
directory resolves to, which answers most of the questions a report raises. It
prints no credential, only masked prefixes.

```console
$ ranma doctor
```

**Version, OS and shell** — `ranma --version`, and how you installed it.

> If the bug is that a credential appeared somewhere it should not, stop here and
> use [private reporting](https://github.com/devlefel/ranma/security/advisories/new)
> instead. Do not paste the leak into a public issue.
