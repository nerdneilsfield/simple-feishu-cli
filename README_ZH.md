# simple-feishu-cli

[English](README.md) | [简体中文](README_ZH.md)

[![CI](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/ci.yaml)
[![Release](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml/badge.svg)](https://github.com/nerdneilsfield/simple-feishu-cli/actions/workflows/release.yaml)
[![Go Version](https://img.shields.io/badge/go-1.24.0-00ADD8?logo=go)](go.mod)

`feishu` 是一个面向企业自建应用的极简飞书 CLI，用来给飞书用户或群组发送纯文本消息、Feishu post 富文本消息、Markdown 转换后的富文本消息，或者先上传本地文件再发送文件消息，并支持列出机器人已加入的群。

当前范围：

- `./dist/feishu send text`
- `./dist/feishu send file`
- `./dist/feishu send post`
- `./dist/feishu send md`
- `./dist/feishu list chats`
- 凭证优先级：命令行参数 > 环境变量 > 配置文件
- 发送消息命令的成功输出字段固定：`message_id`、`msg_type`、`receive_id`、`receive_id_type`
- 退出码固定：`0`、`2`、`3`、`4`、`10`

当前不做：

- 查人 / 查群
- 除 `text`、`file`、`post` 之外的其他消息类型，例如卡片和图片
- 多 profile / 多租户配置
- token 本地缓存

## 快速开始

要求 Go `1.24.0` 或兼容的 `1.24.x` 版本。

从源码构建：

```bash
go build -o ./dist/feishu ./cmd/feishu
./dist/feishu --help
```

先设置凭证环境变量：

```bash
export FEISHU_APP_ID='cli_xxx'
export FEISHU_APP_SECRET='secret_xxx'
```

发送文本消息：

```bash
./dist/feishu send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello from cli"
```

上传并发送文件：

```bash
./dist/feishu send file \
  --to-type chat_id \
  --to oc_xxx \
  --path ./report.pdf
```

直接发送本地 Feishu `post` JSON：

```bash
./dist/feishu send post \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-basic.json
```

把 Markdown 转成 Feishu `post` 后发送：

```bash
./dist/feishu send md \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-from-markdown.md
```

所有发送命令的成功输出字段名都固定不变，`msg_type` 会随着命令变化，例如 `text`、`file` 或 `post`：

```text
message_id=om_xxx
msg_type=post
receive_id=oc_xxx
receive_id_type=chat_id
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
./dist/feishu \
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

群列表查询满足以下任一权限即可：

- `im:chat`
- `im:chat:read`
- `im:chat:readonly`

补充：

- 发送消息和上传文件都要求应用开启机器人能力
- 上传文件不能是空文件
- 官方文档给出的上传大小限制是 30 MB
- `send post` 和 `send md` 使用的仍然是同一套发消息权限

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
- 给群发消息时用 `chat_id`；群组不是用 `open_id` 作为发送目标
- 如果别的系统已经给你 `user_id` 或 `union_id`，CLI 也可以直接使用

官方消息接口文档里也列出了这些 `receive_id_type`。

</details>

## 作为 Go 库使用

这个模块现在暴露了公开的 `config` 和 `feishu` 包，所以你可以在 Go 程序里直接复用同一套行为，而不是通过 shell 调 CLI。

最小示例：

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
<summary>其它公开库入口</summary>

下面这些能力都复用同一个 `client`。下面默认你已经有 `ctx := context.Background()`，并为 `SendPost` 导入了 `encoding/json`：

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

公开的配置加载器和 CLI 保持同样的优先级：显式 options，其次环境变量，最后配置文件。`SendMarkdown` 和 CLI 的 `send md` 使用同一套受控 Markdown 子集，遇到不支持的结构会直接返回错误。

</details>

## 更多示例

列出机器人已加入的群：

```bash
./dist/feishu list chats
```

以 JSON 输出群列表：

```bash
./dist/feishu list chats --format json
```

CI/CD 场景下用显式参数：

```bash
./dist/feishu \
  --app-id "${FEISHU_APP_ID}" \
  --app-secret "${FEISHU_APP_SECRET}" \
  send file \
  --to-type chat_id \
  --to "${FEISHU_CHAT_ID}" \
  --path ./artifacts/report.pdf
```

如果你把二进制安装进了 `PATH`，把 `./dist/feishu` 换成 `feishu` 即可。

<details>
<summary>在 CI/CD 中从 GitHub Releases 安装</summary>

下面的示例固定使用你即将发布的 `0.0.2` 版本。Linux 的 GitHub Hosted Runner 和 GitLab Linux Runner 都可以直接下载这个包：

```text
https://github.com/nerdneilsfield/simple-feishu-cli/releases/download/v0.0.2/feishu_0.0.2_linux_amd64.tar.gz
```

GitHub Actions 示例：

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

      - name: 安装 feishu CLI
        run: |
          curl -fsSL -o /tmp/feishu.tar.gz \
            "https://github.com/nerdneilsfield/simple-feishu-cli/releases/download/v${FEISHU_VERSION}/feishu_${FEISHU_VERSION}_linux_amd64.tar.gz"
          tar -xzf /tmp/feishu.tar.gz -C /tmp
          install /tmp/feishu /usr/local/bin/feishu
          feishu --version || true

      - name: 列出机器人所在群
        run: feishu list chats --format json

      - name: 发送文本消息
        run: |
          feishu send text \
            --to-type open_id \
            --to "$FEISHU_OPEN_ID" \
            --text "build ${GITHUB_SHA} finished"

      - name: 上传并发送文件
        run: |
          printf 'build ok\n' > report.txt
          feishu send file \
            --to-type chat_id \
            --to "$FEISHU_CHAT_ID" \
            --path ./report.txt

      - name: 发送原生 post JSON
        run: |
          feishu send post \
            --to-type chat_id \
            --to "$FEISHU_CHAT_ID" \
            --file ./examples/post-basic.json

      - name: 发送 Markdown 富文本
        run: |
          feishu send md \
            --to-type chat_id \
            --to "$FEISHU_CHAT_ID" \
            --file ./examples/post-from-markdown.md
```

GitLab Runner 示例：

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

如果你用的是自托管 ARM64 Runner，把资产名里的 `linux_amd64` 换成 `linux_arm64`。

</details>

<details>
<summary>富文本命令说明</summary>

`send text` 仍然只发纯文本，不会解析 Markdown，也不会生成 Feishu `post`。

如果你已经有 Feishu `post` JSON，就用 `send post`：

```bash
./dist/feishu send post \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-basic.json
```

这个 JSON 文件必须是顶层对象，CLI 会按 `msg_type=post` 直接发送。先用 `examples/post-basic.json` 验证链路，再用 `examples/post-rich.json` 看更丰富的原生 `post` 结构。

如果你想让 CLI 负责把 Markdown 转成 Feishu `post`，就用 `send md`：

```bash
./dist/feishu send md \
  --to-type chat_id \
  --to oc_xxx \
  --file ./examples/post-from-markdown.md
```

当前支持的 Markdown 子集：

- 第一个 `# H1` 会变成 `zh_cn.title`
- 普通段落
- `**粗体**`、`*斜体*`、`~~删除线~~`
- 纯文本标签链接和自动链接
- 行内代码
- 围栏代码块和缩进代码块
- 引用块
- 有序列表和无序列表

当前不支持：

- 图片
- 表格
- 原始 HTML
- task list
- 引用里套列表、列表里套引用这类嵌套块结构
- `***bold italic***` 这类嵌套行内样式
- `[**bold**](https://example.com)` 这类带样式的链接文本
- 除第一个一级标题之外的其他标题

遇到不支持的 Markdown，CLI 会直接失败并返回退出码 `2`。本地文件读取失败仍然返回退出码 `4`。

仓库内可直接参考的示例：

- `examples/post-basic.json`：最小可运行的 `send post` 示例
- `examples/post-rich.json`：更丰富的 Feishu 原生 `post` 节点组合
- `examples/post-from-markdown.md`：给 `send md` 使用的 Markdown 输入示例

</details>

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
