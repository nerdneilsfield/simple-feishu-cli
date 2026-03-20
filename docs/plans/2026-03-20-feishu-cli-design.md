# Feishu CLI Design

日期：2026-03-20

## 1. 目标

构建一个用 Go 开发的飞书 CLI `feishu`，用于在企业自建应用场景下，通过 `app_id` 和 `app_secret` 给飞书用户或群组发送文本消息与文件消息。

第一版目标刻意收窄为：

- 只支持企业自建应用
- 只支持传入现成的目标 ID
- 支持发送文本消息
- 支持上传本地文件并立即发送
- 支持通过命令行参数、环境变量、配置文件提供凭证
- 支持 `goreleaser` 和 GitHub Actions 发布流程

## 2. 非目标

以下内容不在第一版范围内：

- 不支持 ISV 应用和 `tenant_key`
- 不支持通过邮箱、手机号、群名查找目标对象
- 不支持批量发送
- 不支持 markdown、post、卡片、图片等更多消息类型
- 不支持多 profile、多租户配置
- 不支持 token 落盘缓存
- 不支持 webhook、daemon 或长期运行模式
- 不支持 Homebrew、Scoop、apt、rpm、Docker 镜像分发

## 3. 技术选型

### 3.1 飞书 SDK

采用官方 Go SDK：`larksuite/oapi-sdk-go`

选择理由：

- 官方维护，接口覆盖稳定
- 更适合“封装飞书 OpenAPI”这个目标
- 能支撑后续扩展，而不是局限在 IM-only 场景

### 3.2 CLI 框架

采用 `cobra`

选择理由：

- 命令结构清晰
- 参数校验和帮助信息成熟
- 适合后续扩展更多子命令

### 3.3 配置加载

采用手写配置加载逻辑，不引入 `viper`

选择理由：

- 项目配置项极少
- 可以明确控制优先级，避免隐式行为
- 降低依赖和调试复杂度

## 4. CLI 设计

### 4.1 命令结构

```bash
feishu send text --to-type open_id --to ou_xxx --text "hello"
feishu send file --to-type chat_id --to oc_xxx --path ./report.pdf
```

### 4.2 全局凭证参数

所有命令支持：

- `--app-id`
- `--app-secret`

### 4.3 目标参数

发送命令统一要求：

- `--to-type`
- `--to`

支持的 `--to-type`：

- `open_id`
- `user_id`
- `union_id`
- `chat_id`

### 4.4 子命令参数

`send text`：

- `--text`

`send file`：

- `--path`

### 4.5 输出约定

成功时输出稳定的键值信息。

`send text` 与 `send file` 都必须至少输出：

- `message_id`
- `msg_type`
- `receive_id`
- `receive_id_type`

`send file` 不额外输出上传阶段内部细节，例如 `file_key`。

例如：

```text
message_id=om_xxx
msg_type=file
receive_id=oc_xxx
receive_id_type=chat_id
```

失败时返回非零退出码，并输出清晰错误，例如：

```text
error: feishu api error: code=99991663 msg=insufficient permission
```

退出码约定：

- `0`：成功
- `2`：命令行参数或输入校验错误
- `3`：配置或凭证错误
- `4`：本地文件读写错误
- `10`：飞书 API 错误

## 5. 配置设计

### 5.1 配置来源

凭证来源优先级：

1. 命令行参数 `--app-id` / `--app-secret`
2. 环境变量 `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
3. 配置文件 `~/.config/feishu/config.yaml`

### 5.2 配置文件格式

```yaml
app_id: cli_xxx
app_secret: xxxxx
```

### 5.3 安全取舍

- 本地默认推荐环境变量或配置文件
- CI/CD 可通过命令行显式传参
- 命令行参数中的 secret 可能暴露在进程参数中，因此仅作为支持方式，不作为默认推荐方式
- Unix-like 系统上，如果配置文件权限对 group 或 other 可读写执行，则 CLI 默认拒绝加载，并提示收紧到 owner-only，例如 `0600`
- Windows 上不做 POSIX 权限位检查，但文档中仍要求用户只将配置文件放在当前用户可访问目录

## 6. 模块设计

建议目录结构：

```text
cmd/feishu/
internal/cli/
internal/config/
internal/feishu/
```

### 6.1 `cmd/feishu/`

- 程序入口
- 初始化根命令

### 6.2 `internal/cli/`

- 子命令定义
- 参数绑定
- 参数校验
- 文本输出格式化

### 6.3 `internal/config/`

- 读取命令行参数
- 读取环境变量
- 读取配置文件
- 合并运行时配置

### 6.4 `internal/feishu/`

对官方 SDK 做薄封装，暴露稳定内部接口：

- `SendText(toType, toID, text)`
- `SendFile(toType, toID, path)`

## 7. 数据流设计

### 7.1 发送文本消息

1. 解析命令行参数
2. 读取并合并凭证配置
3. 初始化飞书客户端
4. 使用 `receive_id_type` 和 `receive_id` 调用消息发送接口
5. 输出 `message_id` 和上下文信息

### 7.2 发送文件消息

1. 解析命令行参数
2. 校验本地文件存在且可读
3. 读取并合并凭证配置
4. 初始化飞书客户端
5. 上传本地文件
6. 获取 `file_key`
7. 发送 `msg_type=file` 消息
8. 输出 `message_id` 和上下文信息

## 8. 错误处理

错误分为三类：

### 8.1 参数错误

例如：

- 缺少 `--to`
- 缺少 `--text`
- 缺少 `--path`
- `--to-type` 非法

处理方式：

- 本地直接报错
- 不发起 API 请求

### 8.2 配置错误

例如：

- 缺少 `app_id`
- 缺少 `app_secret`
- 配置文件格式错误

处理方式：

- 明确指出当前缺失项和读取来源

### 8.3 飞书 API 错误

例如：

- 权限不足
- 文件上传失败
- 目标 ID 无效

处理方式：

- 保留飞书错误码和错误消息
- 返回非零退出码

## 9. 权限与运行前提

运行前需要满足：

- 飞书后台已创建企业自建应用
- 已获得应用的 `app_id` 与 `app_secret`
- 应用已具备发送消息、上传文件等所需权限
- 应用对目标用户或群组有可用的发送范围

在开始实现前，需要补充并固定 README 中列出的最小权限清单，至少覆盖：

- 获取 tenant access token
- 发送消息
- 上传文件

如果某个 API 的准确权限名在设计阶段尚未确认，应在实现前通过官方文档核实，并将精确权限名写入 README 与验收清单。

## 10. 验证要求

在认为第一版“完成”之前，除单元测试外，必须通过一次真实飞书环境 smoke test。

最低要求：

1. 使用真实企业自建应用凭证成功执行一次 `send text`
2. 使用真实企业自建应用凭证成功执行一次 `send file`
3. 两次调用都验证退出码和稳定输出字段

该验证可以手动执行，但不能省略。

## 11. 发布与自动化

### 11.1 `goreleaser`

用于：

- 构建多平台二进制
- 生成压缩包
- 生成 checksums
- 发布 GitHub Releases

目标平台：

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`
- `windows/amd64`
- `windows/arm64`

### 11.2 GitHub Actions

建议两个 workflow：

`ci.yaml`

- 在 `push` 和 `pull_request` 上运行
- 至少执行 `go test ./...`
- 可附加 `go vet ./...`

`release.yaml`

- 在推送 `v*` tag 时触发
- 调用 `goreleaser`
- 使用 `GITHUB_TOKEN` 发布 Release
- 显式声明发布所需权限，例如 `contents: write`
- 禁止仅以本地 `--snapshot` 构建通过作为发版链路验收标准

### 11.3 发布链路验收

在首次正式 release 前，必须验证一次真实 GitHub 发布链路。

最低要求：

1. 基于临时 tag 或预发布 tag 触发 `release.yaml`
2. 确认 workflow 拥有正确权限并能创建 GitHub Release
3. 确认 release 附件与 checksum 均已上传
4. 确认产物名与二进制名 `feishu` 一致

本地 `goreleaser release --snapshot --clean` 仅用于验证打包配置，不替代真实发布验证。

## 12. 风险

### 11.1 应用权限不完整

风险：

- 能拿到 token，但发送消息或上传文件失败

缓解：

- 透传飞书错误码
- 在 README 中列出必要权限

### 11.2 不同目标 ID 类型混用

风险：

- `receive_id_type` 与实际 ID 不匹配导致发送失败

缓解：

- 强制用户显式传 `--to-type`
- 不做自动猜测

### 11.3 跨平台发布仅构建不运行

风险：

- `windows/arm64` 等目标平台可能只能验证编译通过，难以在 CI 中真实运行

缓解：

- 第一版接受“构建成功”为发布标准
- 后续如有需要再补目标平台实机验证
