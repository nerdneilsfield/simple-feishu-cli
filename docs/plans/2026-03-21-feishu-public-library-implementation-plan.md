# Feishu Public Library Implementation Plan

## Objective

Expose the current CLI functionality as reusable public Go packages while keeping the CLI stable.

## Task 1: Create public config package

Promote `internal/config` into a public `config` package at module root.

Requirements:

- preserve `Config`, `LoadOptions`, `Load`, and `DefaultConfigPath`
- preserve current precedence and permission behavior
- migrate tests to the new package path
- do not change runtime behavior

Verification:

- `go test ./config`

## Task 2: Create public feishu package

Promote `internal/feishu` into a public `feishu` package at module root.

Requirements:

- preserve `Client`, message input types, chat types, and typed errors
- preserve `NewClient` behavior and current request handling
- migrate tests to the new package path
- keep current CLI-facing semantics unchanged

Verification:

- `go test ./feishu`

## Task 3: Add SendMarkdown to public feishu client

Expose the Markdown-send capability as a public library method.

Requirements:

- add `MarkdownMessageInput` to the public `feishu` package
- add `(*Client).SendMarkdown(ctx, input)`
- keep existing public seams backward-compatible; expose Markdown sending through a separate `MarkdownSender` seam if needed
- reuse the existing Markdown conversion logic internally
- accept Markdown bytes, not file path
- preserve current post JSON generation behavior

Verification:

- add unit tests for success path
- add unit tests for unsupported Markdown failure path
- add unit tests that send-path API/client errors are preserved by `SendMarkdown`
- `go test ./feishu -run SendMarkdown`

## Task 4: Migrate CLI imports to public packages

Switch `internal/cli` and `cmd/feishu` to depend on the new public packages.

Requirements:

- replace imports of `internal/config` with `config`
- replace imports of `internal/feishu` with `feishu`
- keep CLI output, exit codes, and parameter validation unchanged
- keep `send md` file-reading behavior in CLI

Verification:

- `go test ./internal/cli ./cmd/feishu`

## Task 5: Remove obsolete internal package dependencies

Clean up the repository after the CLI fully depends on public packages.

Requirements:

- remove dead references to `internal/config` and `internal/feishu`
- keep `internal/markdown` as an actual internal package used by the public `feishu` package
- ensure no package still imports the old internal paths

Verification:

- `rg -n "internal/config|internal/feishu"`
- `go test ./...`

## Task 6: Add README library usage docs

Document library consumption in both README files.

Requirements:

- add a short Go example for `config.Load` plus `feishu.NewClient`
- show at least `SendText`, `SendFile`, `SendPost`, `SendMarkdown`, and `ListChats` at a high level
- keep CLI docs intact
- keep English default and Chinese mirror aligned

Verification:

- `git diff --check`

## Task 7: Final verification and review

Run full project verification after all tasks complete.

Required commands:

- `go test ./...`
- `make check`
- `make build`

Final review gates:

1. spec review on public library scope
2. code quality review on API shape, migration safety, and compatibility

## Implementation Notes

- keep commits task-scoped where practical
- do not broaden scope into private chat listing or new APIs
- prefer preserving tests and behavior over refactoring aesthetics
- avoid introducing a second abstraction layer over the public `feishu.Client`
