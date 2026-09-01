# Git hooks & conventions

Git-native hooks (no Node, no extra binaries) that enforce the repo's commit and
branch rules locally, before code ever reaches CI.

## Enable them

Hooks under version control don't run until git is told where they live. One-time
per clone:

```bash
make git-hooks
```

This sets `core.hooksPath` to this directory and marks the hooks executable.

## Rules enforced

### Conventional Commits — `commit-msg`

```
type(scope)?(!)?: subject
```

- **type** — one of: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`,
  `build`, `ci`, `chore`, `revert`
- **scope** — optional, lower-kebab (e.g. `identity`, `gateway`, `payment`)
- **!** — optional, marks a breaking change
- **subject** — non-empty, no trailing period; full header ≤ 100 chars

```
feat(identity): add refresh-token rotation
fix(gateway): drop stale APISIX upstreams on reload
refactor!: split auth out of the identity module
```

Merge / revert / fixup / squash messages are exempt.

### Branch naming + protected branches — `pre-commit`

- Direct commits to `main`, `master`, `develop` are blocked — work on a branch and
  merge via MR. Emergency override: `ALLOW_DIRECT_COMMIT=1 git commit ...`
- Feature branches must match `service/type/short-description`:

```
service/type/description
```

  - **service** — the owning service, lower-kebab (e.g. `identity`, `gateway`, `payment`)
  - **type** — `feat`, `fix`, `hotfix`, `chore`, `docs`, `refactor`, `perf`,
    `test`, `build`, `ci`, `release`
  - **description** — lower-kebab, may use `/` to group

```
identity/feat/otp-verification
gateway/fix/stale-upstreams
```

## Bypassing

`git commit --no-verify` skips all hooks. Use sparingly — CI is the backstop.
