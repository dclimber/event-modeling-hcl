# Contributing

Thanks for considering a contribution to `event-modeling-hcl`.

## Development setup

Install Go 1.25 or newer, then build and test from a checkout:

```bash
go build ./...
make verify
```

- `make help` lists individual commands.
- `make verify` runs the same checks as CI (Run it before opening a pull request):
  - `gofmt` formatting,
  - `go mod tidy` drift,
  - `go vet`,
  - unit tests,
  - race tests,
  - `staticcheck`,
  - `govulncheck`,
  - and validation of every example in `examples/`.

## Code conventions

- Follow the responsibilities and change sequence in
  [architecture-guide.md](architecture-guide.md). In particular, entry points
  delegate to `internal/app`, grammar belongs to `internal/syntax`, shared
  source interpretation belongs to `internal/source`, and validation policy
  belongs to `internal/validator`.
- Prefer table-driven tests with `t.Run` subtests, matching the existing style
  in `internal/validator/validator_test.go` and
  `internal/cli/cli_test.go`.
- Diagnostics use `hcl.Diagnostics`: a short, no-period `Summary` and a full
  sentence `Detail`, matching the existing HCL/Terraform ecosystem
  convention.
- Exported identifiers need a doc comment starting with the identifier name.

## Changing the language grammar

`eventmodeling.hclspec.md` is the normative grammar. Any change to what
the validator accepts must update that document, `schema/eventmodeling.hcl.schema.md`
if the source-model mapping changes, and `CHANGELOG.md`.

## Pull requests

- Keep changes focused; unrelated cleanup belongs in a separate PR.
- Update `CHANGELOG.md` for user-visible changes.
- Make sure `make verify` passes before requesting review.
