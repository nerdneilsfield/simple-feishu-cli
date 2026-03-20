# simple-feishu-cli

[English](README.md) | [简体中文](README_ZH.md)

[![CI](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml)
[![Release](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml)
[![Go Version](https://img.shields.io/badge/go-1.24.0-00ADD8?logo=go)](go.mod)

`feishu` 是一个面向企业自建应用的极简飞书 CLI，用来给飞书用户或群组发送文本消息，或者先上传本地文件再发送文件消息。

当前范围：

- `./feishu send text`
- `./feishu send file`
- 凭证优先级：命令行参数 > 环境变量 > 配置文件
- 成功输出字段固定：`message_id`、`msg_type`、`receive_id`、`receive_id_type`
- 退出码固定：`0`、`2`、`3`、`4`、`10`

当前不做：

- 查人 / 查群
- 富文本、卡片、图片等其他消息类型
- 多 profile / 多租户配置
- token 本地缓存

## 快速开始

要求 Go `1.24.0` 或兼容的 `1.24.x` 版本。

从源码构建：

```bash
go build -o feishu ./cmd/feishu
./feishu --help
```

先设置凭证环境变量：

```bash
export FEISHU_APP_ID='cli_xxx'
export FEISHU_APP_SECRET='secret_xxx'
```

发送文本消息：

```bash
./feishu send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello from cli"
```

上传并发送文件：

```bash
./feishu send file \
  --to-type chat_id \
  --to oc_xxx \
  --path ./report.pdf
```

成功输出固定为：

```text
message_id=om_xxx
msg_type=text
receive_id=ou_xxx
receive_id_type=open_id
```

<details>
<summary>配置方式与优先级</summary>

凭证来源优先级：

1. `--app-id` / `--app-secret`
2. `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
3. `~/.config/feishu/config.yaml`

仓库里提供了示例配置：

```bash
mkdir -p ~/.config/feishu
cp config.example.yaml ~/.config/feishu/config.yaml
chmod 600 ~/.config/feishu/config.yaml
```

示例字段请直接参考 `config.example.yaml`。

如果你要指定自定义配置文件：

```bash
./feishu \
  --config ./feishu-prod.yaml \
  send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello"
```

说明：

- 在 Unix-like 系统上，配置文件权限必须是 owner-only，例如 `0600`
- 如果显式传了 `--config` 但路径不存在，CLI 会直接报错
- `config.example.yaml` 只用于示例，不是实际运行配置
- 仓库本地配置文件例如 `config.toml`、`config.yaml`，以及构建产物和 `dist/` 都会被 git 忽略
- `--app-secret` 适合 CI/CD；在本地更推荐环境变量或受保护的配置文件

</details>

<details>
<summary>飞书前置条件与权限</summary>

飞书侧至少需要满足：

- 使用企业自建应用
- 应用已开启机器人能力
- 给用户发消息时，该用户在机器人的可用范围内
- 给群发消息时，机器人已经在群里且有发言权限

官方文档：

- 发送消息：<https://open.feishu.cn/document/server-docs/im-v1/message/create.md>
- 上传文件：<https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create.md>
- 自建应用获取 `tenant_access_token`：<https://open.feishu.cn/document/server-docs/authentication-management/access-token/tenant_access_token_internal.md>

`tenant_access_token` 接口本身不需要额外权限。

发送消息满足以下任一权限即可：

- `im:message`
- `im:message:send_as_bot`
- `im:message:send`

上传文件满足以下任一权限即可：

- `im:resource`
- `im:resource:upload`

补充：

- 发送消息和上传文件都要求应用开启机器人能力
- 上传文件不能是空文件
- 官方文档给出的上传大小限制是 30 MB

</details>

<details>
<summary>支持的目标 ID</summary>

当前支持的 `--to-type`：

- `open_id`
- `user_id`
- `union_id`
- `chat_id`

获取 ID 的官方入口：

- Open ID：<https://open.feishu.cn/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-openid>
- Union ID：<https://open.feishu.cn/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-union-id>
- User ID：<https://open.feishu.cn/document/uAjLw4CM/ugTN1YjL4UTN24CO1UjN/trouble-shooting/how-to-obtain-user-id>
- Chat ID：<https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/chat-id-description>

实践上：

- 给单个用户发消息时优先用 `open_id`
- 给群发消息时用 `chat_id`
- 如果别的系统已经给你 `user_id` 或 `union_id`，CLI 也可以直接使用

官方消息接口文档里也列出了这些 `receive_id_type`。

</details>

## 更多示例

CI/CD 场景下用显式参数：

```bash
./feishu \
  --app-id "${FEISHU_APP_ID}" \
  --app-secret "${FEISHU_APP_SECRET}" \
  send file \
  --to-type chat_id \
  --to "${FEISHU_CHAT_ID}" \
  --path ./artifacts/report.pdf
```

如果你把二进制安装进了 `PATH`，把 `./feishu` 换成 `feishu` 即可。

<details>
<summary>输出契约、退出码与排障</summary>

失败输出固定为：

```text
error: <message>
```

退出码：

- `0`：成功
- `2`：参数或输入校验错误
- `3`：配置、凭证或本地客户端错误
- `4`：本地文件错误
- `10`：飞书 API 错误

常见错误：

### `error: missing required credentials: app_id, app_secret`

按顺序检查：

- `--app-id` / `--app-secret`
- `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
- `~/.config/feishu/config.yaml`

### `error: config file ".../config.yaml" has insecure permissions ...; use 0600`

在 Unix-like 系统上修正权限：

```bash
chmod 600 ~/.config/feishu/config.yaml
```

### `error: invalid --to-type "..."`

改用以下值之一：

- `open_id`
- `user_id`
- `union_id`
- `chat_id`

### `error: stat_file "...": no such file or directory`

`send file` 指向的本地文件不存在，或者不可读。

### `error: send_text: code=99991663 msg=insufficient permission`

重点检查：

- 是否开启了机器人能力
- 是否授予了消息发送或文件上传权限
- 目标用户是否在应用可用范围内
- 机器人是否已经在目标群里且允许发言

</details>

<details>
<summary>开发与自动化</summary>

本地基线检查入口：

```bash
make
```

等价命令：

```bash
go test ./...
go vet ./...
```

当前仓库已配置的自动化：

- `.github/workflows/ci.yaml` 会在 push 和 pull request 上运行 `make check`
- `.github/workflows/release.yaml` 会在 `v*` tag 上运行 GoReleaser
- `.goreleaser.yml` 会为 Linux、macOS、Windows 的 `amd64` 和 `arm64` 产出发布包

</details>
