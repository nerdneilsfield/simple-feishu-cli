# simple-feishu-cli

`feishu` 是一个最小可用的飞书 CLI，用来在企业自建应用场景下给飞书用户或群组发送文本消息，或者上传本地文件后发送文件消息。

当前已实现：

- `feishu send text`
- `feishu send file`
- 凭证来源优先级：命令行参数 > 环境变量 > 配置文件
- 稳定输出字段：`message_id`、`msg_type`、`receive_id`、`receive_id_type`
- 固定退出码：`0`、`2`、`3`、`4`、`10`

第一版刻意不做：

- 查人/查群
- 富文本、卡片、图片等更多消息类型
- 多 profile / 多租户配置
- token 本地缓存

## 安装

从源码构建：

```bash
go build -o feishu ./cmd/feishu
```

或者安装到你的 `GOBIN` / `GOPATH/bin`：

```bash
go install ./cmd/feishu
```

查看帮助：

```bash
feishu --help
```

## 前提条件

在飞书开放平台侧，至少需要满足这些条件：

- 使用企业自建应用
- 应用已开启机器人能力
- 发送给用户时，该用户在机器人的可用范围内
- 发送给群时，机器人已在群中且有发言权限

官方文档：

- 发送消息：<https://open.feishu.cn/document/server-docs/im-v1/message/create.md>
- 上传文件：<https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create.md>
- 自建应用获取 `tenant_access_token`：<https://open.feishu.cn/document/server-docs/authentication-management/access-token/tenant_access_token_internal.md>

## 权限要求

`tenant_access_token` 接口本身不需要额外权限。

发送消息接口满足以下任一权限即可：

- `im:message`
- `im:message:send_as_bot`
- `im:message:send`

上传文件接口满足以下任一权限即可：

- `im:resource`
- `im:resource:upload`

补充说明：

- 发送消息和上传文件都要求应用开启机器人能力
- 上传文件官方限制为：文件不能是空文件，且大小不能超过 30 MB

## 配置凭证

当前凭证优先级：

1. `--app-id` / `--app-secret`
2. `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
3. `~/.config/feishu/config.yaml`

### 方式 1：命令行参数

适合 CI/CD 或一次性调用：

```bash
feishu \
  --app-id "$FEISHU_APP_ID" \
  --app-secret "$FEISHU_APP_SECRET" \
  send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello"
```

### 方式 2：环境变量

```bash
export FEISHU_APP_ID='cli_xxx'
export FEISHU_APP_SECRET='secret_xxx'
```

然后直接调用：

```bash
feishu send text --to-type open_id --to ou_xxx --text "hello"
```

### 方式 3：配置文件

默认路径：

```text
~/.config/feishu/config.yaml
```

示例内容：

```yaml
app_id: cli_xxx
app_secret: secret_xxx
```

Unix-like 系统下，配置文件权限必须是 owner-only，例如：

```bash
chmod 600 ~/.config/feishu/config.yaml
```

如果权限过宽，CLI 会拒绝加载该文件。

安全建议：

- 本地优先使用环境变量或受限权限的配置文件
- CI/CD 中如必须使用命令行参数，注意平台是否会屏蔽敏感参数
- 不要把 `app_secret` 提交到仓库

## 使用方式

当前 `--to-type` 只支持：

- `open_id`
- `user_id`
- `union_id`
- `chat_id`

### 发送文本消息

```bash
feishu send text \
  --to-type open_id \
  --to ou_xxx \
  --text "hello from cli"
```

### 上传并发送文件

```bash
feishu send file \
  --to-type chat_id \
  --to oc_xxx \
  --path ./report.pdf
```

### CI/CD 示例

使用命令行参数：

```bash
./feishu \
  --app-id "${FEISHU_APP_ID}" \
  --app-secret "${FEISHU_APP_SECRET}" \
  send file \
  --to-type chat_id \
  --to "${FEISHU_CHAT_ID}" \
  --path ./artifacts/report.pdf
```

使用环境变量：

```bash
export FEISHU_APP_ID="${FEISHU_APP_ID}"
export FEISHU_APP_SECRET="${FEISHU_APP_SECRET}"

./feishu send text \
  --to-type open_id \
  --to "${FEISHU_OPEN_ID}" \
  --text "build succeeded"
```

## 输出格式

成功时固定输出：

```text
message_id=om_xxx
msg_type=text
receive_id=ou_xxx
receive_id_type=open_id
```

`send file` 也使用同样的输出字段，不会打印 `file_key`。

失败时输出格式：

```text
error: <message>
```

## 退出码

- `0`：成功
- `2`：参数或输入校验错误
- `3`：配置、凭证或本地客户端错误
- `4`：本地文件错误
- `10`：飞书 API 错误

## 常见错误

### `error: missing required credentials: app_id, app_secret`

原因：

- 没有通过参数、环境变量或配置文件提供凭证

处理：

- 检查 `--app-id` / `--app-secret`
- 检查 `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
- 检查 `~/.config/feishu/config.yaml`

### `error: config file ".../config.yaml" has insecure permissions ...; use 0600`

原因：

- Unix-like 系统下配置文件权限过宽

处理：

```bash
chmod 600 ~/.config/feishu/config.yaml
```

### `error: invalid --to-type "..."`

原因：

- `--to-type` 不在 CLI 支持范围内

处理：

- 改用 `open_id`、`user_id`、`union_id` 或 `chat_id`

### `error: stat_file "...": no such file or directory`

原因：

- `send file` 指定的本地文件不存在

处理：

- 检查 `--path`
- 确认文件存在且可读

### `error: send_text: code=99991663 msg=insufficient permission`

原因：

- 飞书权限不足，或者机器人能力/可用范围/群权限不满足

处理：

- 检查应用是否开启机器人能力
- 检查是否授予了消息发送或文件上传权限
- 检查用户是否在可用范围内
- 检查机器人是否在目标群中且有发言权限

## 开发

运行测试：

```bash
go test ./...
```
