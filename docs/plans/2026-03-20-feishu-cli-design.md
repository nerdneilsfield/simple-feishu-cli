# Feishu CLI Design

日期：2026-03-20

## 1. 目标

构建一个用 Go 开发的飞书 CLI，用于在企业自建应用场景下，通过 `app_id` 和 `app_secret` 给飞书用户或群组发送文本消息与文件消息。

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

成功时输出稳定的键值信息，例如：

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

## 5. 配置设计

### 5.1 配置来源

凭证来源优先级：

1. 命令行参数 `--app-id` / `--app-secret`
2. 环境变量 `FEISHU_APP_ID` / `FEISHU_APP_SECRET`
3. 配置文件 `~/.config/feishu-cli/config.yaml`

### 5.2 配置文件格式

```yaml
app_id: cli_xxx
app_secret: xxxxx
```

### 5.3 安全取舍

- 本地默认推荐环境变量或配置文件
- CI/CD 可通过命令行显式传参
- 命令行参数中的 secret 可能暴露在进程参数中，因此仅作为支持方式，不作为默认推荐方式

## 6. 模块设计

建议目录结构：

```text
cmd/feishu-cli/
internal/cli/
internal/config/
internal/feishu/
```

### 6.1 `cmd/feishu-cli/`

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

## 10. 发布与自动化

### 10.1 `goreleaser`

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

### 10.2 GitHub Actions

建议两个 workflow：

`ci.yaml`

- 在 `push` 和 `pull_request` 上运行
- 至少执行 `go test ./...`
- 可附加 `go vet ./...`

`release.yaml`

- 在推送 `v*` tag 时触发
- 调用 `goreleaser`
- 使用 `GITHUB_TOKEN` 发布 Release

## 11. 风险

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
