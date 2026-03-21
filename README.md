# simple-feishu-cli

[English](README.md) | [简体中文](README_ZH.md)

[![CI](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml)
[![Release](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml)
[![Go Version](https://img.shields.io/badge/go-1.24.0-00ADD8?logo=go)](go.mod)

`feishu` is a small CLI for self-built Feishu apps. It sends plain-text, post, and Markdown-converted rich-text messages to users or chats, uploads a local file before sending it as a file message, and lists joined chats.

Current scope:

- `./feishu send text`
- `./feishu send file`
- `./feishu send post`
- `./feishu send md`
- `./feishu list chats`
- credential precedence: CLI flags > environment variables > config file
- stable success fields for message sends: `message_id`, `msg_type`, `receive_id`, `receive_id_type`
- fixed exit codes: `0`, `2`, `3`, `4`, `10`

Out of scope for now:

- user/chat lookup
- cards, images, or other message types beyond `text`, `file`, and `post`
- multi-profile or multi-tenant config
- local token caching

## Quickstart

Requires Go `1.24.0` or a compatible `1.24.x` release.

Build from source:

```bash
go build -o feishu ./cmd/feishu
./feishu --help
```

Set credentials with env vars:

```bash
export FEISHU_APP_ID='cli_xxx'
export FEISHU_APP_SECRET='secret_xxx'
```

Send a text message:

```bash
./feishu send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello from cli"
```

Upload and send a file:

```bash
./feishu send file \
  --to-type chat_id \
  --to oc_xxx \
  --path ./report.pdf
```

Send a Feishu `post` payload from local JSON:

```bash
./feishu send post \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-basic.json
```

Convert Markdown and send it as Feishu `post`:

```bash
./feishu send md \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-from-markdown.md
```

Successful output for all send commands keeps the same field names. `msg_type` matches the command, for example `text`, `file`, or `post`:

```text
message_id=om_xxx
msg_type=post
receive_id=oc_xxx
receive_id_type=chat_id
```

<details>
<summary>Configuration and credential precedence</summary>

Credential sources are resolved in this order:

1. `--app-id` / `--app-secret`
2. `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
3. `~/.config/feishu/config.yaml`

Example config file in this repo:

```bash
mkdir -p ~/.config/feishu
cp config.example.yaml ~/.config/feishu/config.yaml
chmod 600 ~/.config/feishu/config.yaml
```

See `config.example.yaml` for the example schema.

Use a custom config path when needed:

```bash
./feishu \
  --config ./feishu-prod.yaml \
  send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello"
```

Notes:

- On Unix-like systems, config files must be owner-only, for example `0600`.
- If the config path is explicit and missing, the CLI fails fast.
- `config.example.yaml` is for documentation only. Keep real local config files separate.
- Repository-local files such as `config.toml`, `config.yaml`, build outputs, and `dist/` are ignored by git.
- Passing `--app-secret` is supported, but env vars or a protected config file are safer outside CI/CD.

</details>

<details>
<summary>Prerequisites and Feishu permissions</summary>

At the Feishu side, the minimum requirements are:

- use a self-built app
- enable bot capability for the app
- when sending to a user, that user must be inside the bot's availability scope
- when sending to a chat, the bot must already be in the chat and allowed to speak

Official docs:

- send message: <https://open.feishu.cn/document/server-docs/im-v1/message/create.md>
- upload file: <https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create.md>
- self-built app tenant token: <https://open.feishu.cn/document/server-docs/authentication-management/access-token/tenant_access_token_internal.md>

The `tenant_access_token` endpoint itself does not require extra permission.

Any one of these permissions is enough for message sending:

- `im:message`
- `im:message:send_as_bot`
- `im:message:send`

Any one of these permissions is enough for file upload:

- `im:resource`
- `im:resource:upload`

Any one of these permissions is enough for chat listing:

- `im:chat`
- `im:chat:read`
- `im:chat:readonly`

Additional notes:

- both message sending and file upload require bot capability
- uploaded files must not be empty
- the documented size limit for uploaded files is 30 MB
- `send post` and `send md` use the same message-send permissions as `send text`

</details>

<details>
<summary>Supported target IDs</summary>

Supported `--to-type` values:

- `open_id`
- `user_id`
- `union_id`
- `chat_id`

Official entry points for obtaining IDs:

- Open ID: <https://open.feishu.cn/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-openid>
- Union ID: <https://open.feishu.cn/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-union-id>
- User ID: <https://open.feishu.cn/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-user-id>
- Chat ID: <https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/chat-id-description>

Practical defaults:

- for a single user, prefer `open_id`
- for a group, use `chat_id`; groups do not use `open_id` as the send target
- if another system already gives you `user_id` or `union_id`, the CLI accepts those directly

The official message API docs also list these `receive_id_type` values.

</details>

## Use as a Go library

The module now exposes public `config` and `feishu` packages, so you can reuse the same behavior from Go without shelling out to the CLI.

Minimal example:

```go
package main

import (
    "context"
    "log"

    "github.com/nerdneilsfield/simple-feishu-cli/config"
    "github.com/nerdneilsfield/simple-feishu-cli/feishu"
)

func main() {
    cfg, err := config.Load(config.LoadOptions{})
    if err != nil {
        log.Fatal(err)
    }

    client, err := feishu.NewClient(cfg)
    if err != nil {
        log.Fatal(err)
    }

    result, err := client.SendText(context.Background(), feishu.TextMessageInput{
        ReceiveIDType: "open_id",
        ReceiveID:     "ou_xxx",
        Text:          "hello from Go",
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("sent %s as %s", result.MessageID, result.MsgType)
}
```

<details>
<summary>Other public library entry points</summary>

Use the same `client` value for the rest of the public surface. Assume `ctx := context.Background()` and import `encoding/json` for the post example:

```go
_, err = client.SendFile(ctx, feishu.FileMessageInput{
    ReceiveIDType: "chat_id",
    ReceiveID:     "oc_xxx",
    FilePath:      "./artifacts/report.pdf",
})

_, err = client.SendPost(ctx, feishu.PostMessageInput{
    ReceiveIDType: "chat_id",
    ReceiveID:     "oc_xxx",
    Post:          json.RawMessage(`{"zh_cn":{"title":"Notice","content":[[{"tag":"text","text":"hello"}]]}}`),
})

_, err = client.SendMarkdown(ctx, feishu.MarkdownMessageInput{
    ReceiveIDType: "chat_id",
    ReceiveID:     "oc_xxx",
    Markdown:      []byte("# Notice\n\nhello from markdown\n"),
})

chats, err := client.ListChats(ctx)
for _, chat := range chats {
    log.Printf("%s %s %s %s", chat.ChatID, chat.Name, chat.Owner.OpenID, chat.Owner.UnionID)
}
```

The public config loader keeps the same precedence as the CLI: explicit options, then env vars, then the config file. `SendMarkdown` uses the same restricted Markdown subset as the CLI `send md` command and returns an error for unsupported structures.

</details>

## More examples

List joined chats:

```bash
./feishu list chats
```

List joined chats as JSON:

```bash
./feishu list chats --format json
```

CI/CD with explicit flags:

```bash
./feishu \
  --app-id "${FEISHU_APP_ID}" \
  --app-secret "${FEISHU_APP_SECRET}" \
  send file \
  --to-type chat_id \
  --to "${FEISHU_CHAT_ID}" \
  --path ./artifacts/report.pdf
```

If the binary is installed into `PATH`, drop the `./` prefix.

<details>
<summary>Install from GitHub Releases in CI/CD</summary>

These examples pin the release to `0.0.2`, which is the version you said you are about to publish. The Linux GitHub-hosted and GitLab Linux runners use this asset:

```text
https://github.com/nerdneilsfield/simple-feishu-cli/releases/download/v0.0.2/feishu_0.0.2_linux_amd64.tar.gz
```

GitHub Actions example:

```yaml
name: Notify with feishu

on:
  workflow_dispatch:
  push:
    branches: [main]

jobs:
  notify:
    runs-on: ubuntu-latest
    env:
      FEISHU_VERSION: 0.0.2
      FEISHU_APP_ID: ${{ secrets.FEISHU_APP_ID }}
      FEISHU_APP_SECRET: ${{ secrets.FEISHU_APP_SECRET }}
      FEISHU_OPEN_ID: ${{ secrets.FEISHU_OPEN_ID }}
      FEISHU_CHAT_ID: ${{ secrets.FEISHU_CHAT_ID }}
    steps:
      - uses: actions/checkout@v4

      - name: Install feishu CLI
        run: |
          curl -fsSL -o /tmp/feishu.tar.gz \
            "https://github.com/nerdneilsfield/simple-feishu-cli/releases/download/v${FEISHU_VERSION}/feishu_${FEISHU_VERSION}_linux_amd64.tar.gz"
          tar -xzf /tmp/feishu.tar.gz -C /tmp
          install /tmp/feishu /usr/local/bin/feishu
          feishu --version || true

      - name: List joined chats
        run: feishu list chats --format json

      - name: Send text
        run: |
          feishu send text \
            --to-type open_id \
            --to "$FEISHU_OPEN_ID" \
            --text "build ${GITHUB_SHA} finished"

      - name: Send file
        run: |
          printf 'build ok\n' > report.txt
          feishu send file \
            --to-type chat_id \
            --to "$FEISHU_CHAT_ID" \
            --path ./report.txt

      - name: Send native post JSON
        run: |
          feishu send post \
            --to-type chat_id \
            --to "$FEISHU_CHAT_ID" \
            --file ./examples/post-basic.json

      - name: Send Markdown as post
        run: |
          feishu send md \
            --to-type chat_id \
            --to "$FEISHU_CHAT_ID" \
            --file ./examples/post-from-markdown.md
```

GitLab Runner example:

```yaml
stages:
  - notify

variables:
  FEISHU_VERSION: "0.0.2"

default:
  image: alpine:3.20
  before_script:
    - apk add --no-cache curl tar
    - curl -fsSL -o /tmp/feishu.tar.gz "https://github.com/nerdneilsfield/simple-feishu-cli/releases/download/v${FEISHU_VERSION}/feishu_${FEISHU_VERSION}_linux_amd64.tar.gz"
    - tar -xzf /tmp/feishu.tar.gz -C /tmp
    - install -m 0755 /tmp/feishu /usr/local/bin/feishu

notify:text:
  stage: notify
  script:
    - feishu send text --to-type open_id --to "$FEISHU_OPEN_ID" --text "pipeline ${CI_PIPELINE_ID} finished"

notify:file:
  stage: notify
  script:
    - printf 'artifact ok\n' > report.txt
    - feishu send file --to-type chat_id --to "$FEISHU_CHAT_ID" --path ./report.txt

notify:post:
  stage: notify
  script:
    - feishu send post --to-type chat_id --to "$FEISHU_CHAT_ID" --file ./examples/post-basic.json

notify:md:
  stage: notify
  script:
    - feishu send md --to-type chat_id --to "$FEISHU_CHAT_ID" --file ./examples/post-from-markdown.md

notify:list-chats:
  stage: notify
  script:
    - feishu list chats --format json
```

For self-hosted ARM64 runners, switch the asset name from `linux_amd64` to `linux_arm64`.

</details>

<details>
<summary>Rich text commands</summary>

`send text` stays plain text. It does not parse Markdown and does not emit Feishu `post`.

Use `send post` when you already have Feishu `post` JSON:

```bash
./feishu send post \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-basic.json
```

The JSON file must be a top-level object. The CLI sends it as `msg_type=post`. Start with `examples/post-basic.json`, then try `examples/post-rich.json` for a more expressive native `post` sample.

Use `send md` when you want the CLI to convert Markdown into Feishu `post`:

```bash
./feishu send md \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-from-markdown.md
```

Supported Markdown subset today:

- the first `# H1` becomes `zh_cn.title`
- paragraphs
- `**bold**`, `*italic*`, `~~strike~~`
- links with plain-text labels and autolinks
- inline code
- fenced and indented code blocks
- blockquotes
- ordered and unordered lists

Unsupported Markdown today:

- images
- tables
- raw HTML
- task lists
- nested block structures such as quote-in-list or list-in-quote
- nested inline styles such as `***bold italic***`
- styled link labels such as `[**bold**](https://example.com)`
- headings other than the first level-1 heading

Unsupported Markdown fails fast with exit code `2`. Local file read failures still use exit code `4`.

Reference examples in this repo:

- `examples/post-basic.json`: minimal `send post` smoke sample
- `examples/post-rich.json`: richer native Feishu `post` node combinations
- `examples/post-from-markdown.md`: `send md` sample input

</details>

<details>
<summary>Output contract, exit codes, and troubleshooting</summary>

Failure output is always:

```text
error: <message>
```

Exit codes:

- `0`: success
- `2`: argument or input validation error
- `3`: config, credential, or local client error
- `4`: local file error
- `10`: Feishu API error

Common failures:

### `error: missing required credentials: app_id, app_secret`

Check these sources in order:

- `--app-id` / `--app-secret`
- `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
- `~/.config/feishu/config.yaml`

### `error: config file ".../config.yaml" has insecure permissions ...; use 0600`

Fix the file mode on Unix-like systems:

```bash
chmod 600 ~/.config/feishu/config.yaml
```

### `error: invalid --to-type "..."`

Use one of these values:

- `open_id`
- `user_id`
- `union_id`
- `chat_id`

### `error: stat_file "...": no such file or directory`

The local file given to `send file` does not exist or is not readable.

### `error: send_text: code=99991663 msg=insufficient permission`

Check:

- bot capability is enabled
- the app has message send or file upload permission
- the target user is inside the app scope
- the bot is inside the target chat and allowed to speak

</details>

<details>
<summary>Development and automation</summary>

Local baseline checks:

```bash
make
```

Equivalent commands:

```bash
go test ./...
go vet ./...
```

GitHub automation already in this repo:

- `.github/workflows/ci.yaml` runs `make check` on push and pull request
- `.github/workflows/release.yaml` runs GoReleaser on `v*` tags
- `.goreleaser.yml` builds release archives for Linux, macOS, and Windows on `amd64` and `arm64`

</details>
