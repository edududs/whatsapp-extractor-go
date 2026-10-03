# Contributing

Use Go 1.27 or newer. Keep domain and application free of third-party dependencies; the architecture test enforces imports. Define small interfaces where they are consumed, pass contexts explicitly, wrap errors with `%w`, and close resources in the owning scope.

Before submitting:

```sh
make check
go tool govulncheck ./...
```

New store adapters should run `storetest.Contract`. Use table-driven tests for mapping and configuration, fuzz tests for untrusted decoding, and `-race` for concurrent code. Network-dependent WhatsApp tests must be opt-in. Never commit QR codes, session databases, phone credentials or real messages.

Update the decisions document when changing a delivery or persistence contract. Add a versioned migration rather than rewriting an existing schema revision. Keep dependency upgrades explicit and review changes in whatsmeow; the protocol adapter is the boundary for upstream churn.

Use Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`). CI must pass before release tags are pushed. Release binaries are generated from the tag by GitHub Actions, not uploaded from a developer's machine.
