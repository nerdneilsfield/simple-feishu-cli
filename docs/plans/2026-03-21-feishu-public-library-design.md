# Feishu Public Library Design

## Goal

Turn the current repository into a reusable Go library without losing the existing CLI.

The library should expose the same high-level capabilities already implemented by the CLI:

- load credentials from flags/env/config file
- send text messages
- upload and send files
- send native post JSON
- convert Markdown and send it as post
- list chats joined by the bot

The CLI remains in `cmd/feishu`, but becomes a thin wrapper over public packages.

## Non-goals

- split the library into a separate repository
- expose low-level Markdown conversion as a stable public API
- add private-chat listing support
- redesign the message model beyond current feature scope
- add new Feishu features unrelated to the existing CLI

## Public Package Layout

Expose two public packages at module root:

- `github.com/nerdneilsfield/simple-feishu-cli/config`
- `github.com/nerdneilsfield/simple-feishu-cli/feishu`

Keep the CLI in `cmd/feishu`.

Keep Markdown conversion internal to the library implementation. It does not need to become a stable package for external callers yet.

## Public Config API

The public `config` package should expose:

```go
package config

type Config struct {
    AppID     string
    AppSecret string
}

type LoadOptions struct {
    AppID      string
    AppSecret  string
    ConfigPath string
    HomeDir    string
}

func Load(opts LoadOptions) (Config, error)
func DefaultConfigPath(home string) (string, error)
```

Behavior stays exactly aligned with the current CLI contract:

1. explicit options override everything
2. env vars are next
3. config file fills remaining gaps
4. explicit config path fails fast if missing
5. Unix-like config files still require owner-only permissions

## Public Feishu API

The public `feishu` package should expose a high-level client:

```go
package feishu

type Client struct { ... }

func NewClient(cfg config.Config) (*Client, error)

type MessageResult struct {
    MessageID     string
    MsgType       string
    ReceiveID     string
    ReceiveIDType string
}

type TextMessageInput struct {
    ReceiveIDType string
    ReceiveID     string
    Text          string
}

type FileMessageInput struct {
    ReceiveIDType string
    ReceiveID     string
    FilePath      string
}

type PostMessageInput struct {
    ReceiveIDType string
    ReceiveID     string
    Post          json.RawMessage
}

type MarkdownMessageInput struct {
    ReceiveIDType string
    ReceiveID     string
    Markdown      []byte
}

type ChatOwner struct {
    OpenID  string
    UnionID string
}

type ChatSummary struct {
    ChatID string
    Name   string
    Owner  ChatOwner
}

func (c *Client) SendText(ctx context.Context, input TextMessageInput) (MessageResult, error)
func (c *Client) SendFile(ctx context.Context, input FileMessageInput) (MessageResult, error)
func (c *Client) SendPost(ctx context.Context, input PostMessageInput) (MessageResult, error)
func (c *Client) SendMarkdown(ctx context.Context, input MarkdownMessageInput) (MessageResult, error)
func (c *Client) ListChats(ctx context.Context) ([]ChatSummary, error)

// The existing narrow capability seams should remain available for testing and composition.
type Messenger interface {
    SendText(ctx context.Context, input TextMessageInput) (MessageResult, error)
    SendFile(ctx context.Context, input FileMessageInput) (MessageResult, error)
}

type PostSender interface {
    SendPost(ctx context.Context, input PostMessageInput) (MessageResult, error)
    SendMarkdown(ctx context.Context, input MarkdownMessageInput) (MessageResult, error)
}

type ChatLister interface {
    ListChats(ctx context.Context) ([]ChatSummary, error)
}
```

## Library Boundaries

### SendMarkdown

`SendMarkdown` belongs in the public library API because external callers want a full capability, not a partial conversion primitive.

It should accept raw Markdown bytes, not a file path. File IO remains a CLI concern.

### Markdown Conversion Package

The current converter must remain in a real internal package so it is not importable by external callers. The simplest option is to keep the existing `internal/markdown` package and have the public `feishu` package call into it.

The important contract is `SendMarkdown`, not the converter itself.

### Error Model

Preserve current typed errors where useful:

- `APIError`
- `ClientError`
- `LocalFileError`

These are already meaningful for external callers and should remain public on the `feishu` side. `SendMarkdown` should preserve downstream `APIError` and `ClientError` behavior from the post-send path, while still returning conversion failures directly.

Config loading errors continue as normal Go errors from `config.Load`.

## Migration Strategy

Minimize risk by moving in place rather than rewriting:

1. create public `config` package by promoting current `internal/config`
2. create public `feishu` package by promoting current `internal/feishu`
3. add `SendMarkdown` to the public `feishu.Client`
4. switch CLI imports from `internal/config` and `internal/feishu` to public packages
5. keep behavior and output contracts unchanged
6. remove old internal package references only after tests are green

This keeps the CLI stable while immediately making the library importable.

## CLI Impact

`internal/cli` remains internal. It should depend only on public `config` and `feishu` packages after the migration.

The CLI should not own any business logic beyond:

- flag parsing
- file reading for `send post` and `send md`
- output formatting
- exit-code mapping

## Testing Strategy

Keep existing behavior covered while adding public package coverage.

Required coverage:

- public `config` package tests migrated intact
- public `feishu` package tests migrated intact
- new `SendMarkdown` tests in public package
- CLI tests updated to depend on public packages without behavior change
- full `go test ./...`
- `make check`

## Documentation Updates

Update README and README_ZH after implementation to show both use cases:

- CLI usage
- library usage from Go code

Library example should be minimal and high signal.

## Risks

### Import churn

Moving packages from `internal/*` to public root packages changes many imports. The migration must be done in a controlled order to avoid leaving the tree uncompilable for long.

### API freeze too early

Exposing too many low-level helpers would create unnecessary compatibility burden. Keep the first public API high-level and narrow.

### CLI behavior drift

The migration must not accidentally change exit codes, output fields, or config precedence. Existing CLI tests remain the guardrail.
