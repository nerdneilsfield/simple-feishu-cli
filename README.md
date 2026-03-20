# simple-feishu-cli

[English](README.md) | [简体中文](README_ZH.md)

[![CI](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml)
[![Release](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml)
[![Go Version](https://img.shields.io/badge/go-1.24.0-00ADD8?logo=go)](go.mod)

`feishu` is a small CLI for self-built Feishu apps. It sends text messages to users or chats, and uploads a local file before sending it as a file message.

Current scope:

- `./feishu send text`
- `./feishu send file`
- `./feishu list chats`
- credential precedence: CLI flags > environment variables > config file
- stable success fields for message sends: `message_id`, `msg_type`, `receive_id`, `receive_id_type`
- fixed exit codes: `0`, `2`, `3`, `4`, `10`

Out of scope for now:

- user/chat lookup
- rich text, cards, images, or other message types
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

Successful output for `send text` and `send file` is:

```text
message_id=om_xxx
msg_type=text
receive_id=ou_xxx
receive_id_type=open_id
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
