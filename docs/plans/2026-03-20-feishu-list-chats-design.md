# Feishu List Chats Design

日期：2026-03-20

## 1. 目标

为现有 `feishu` CLI 增加 `list chats` 能力，用于列出当前飞书自建应用机器人所在的群聊，并输出每个群的 `chat_id`、群名，以及群主的 `open_id` 和 `union_id`。

该功能的主要用途：

- 让用户快速找到当前机器人已经加入的可发送群
- 为后续 `send text` / `send file` 提供可直接使用的 `chat_id`
- 为排查用户身份与应用上下文问题提供群主的多种用户 ID
- 支持脚本提取，便于结合 `jq` 等工具做自动化处理

## 2. 非目标

本次功能不包含：

- 不返回群的 `user_id`
- 不支持按群名搜索、过滤、排序自定义
- 不支持只查单个群的详情命令
- 不支持输出 CSV、YAML 或模板化输出
- 不支持额外查询群成员列表
- 不支持外部缓存结果

## 3. CLI 设计

### 3.1 新增命令

```bash
feishu list chats
feishu list chats --format json
```

### 3.2 输出格式

默认输出为人类友好的表格，至少包含：

- `chat_id`
- `name`
- `owner_open_id`
- `owner_union_id`

示例：

```text
CHAT_ID                          NAME        OWNER_OPEN_ID    OWNER_UNION_ID
oc_xxx                           报警群      ou_xxx           on_xxx
oc_yyy                           值班群      ou_yyy           on_yyy
```

传入 `--format json` 时，输出稳定 JSON：

```json
{
  "items": [
    {
      "chat_id": "oc_xxx",
      "name": "报警群",
      "owner": {
        "open_id": "ou_xxx",
        "union_id": "on_xxx"
      }
    }
  ]
}
```

### 3.3 参数约束

`list chats` 支持：

- `--format`

允许值：

- `table`（默认）
- `json`

非法值直接返回参数错误，不发起 API 请求。

## 4. 飞书接口设计

### 4.1 群列表

先调用：

- `GET /open-apis/im/v1/chats`

用途：

- 获取当前机器人所在群列表
- 提取 `chat_id` 和群名 `name`
- 自动处理分页，拿全量结果

### 4.2 群详情聚合

对每个 `chat_id` 再调用两次群详情接口：

- `GET /open-apis/im/v1/chats/:chat_id?user_id_type=open_id`
- `GET /open-apis/im/v1/chats/:chat_id?user_id_type=union_id`

用途：

- 读取同一群主在当前应用上下文下的 `open_id`
- 读取同一群主在当前开发者主体下的 `union_id`
- 合并成最终输出结构

### 4.3 为什么不直接用列表接口返回值

`GET /im/v1/chats` 的响应里虽然包含 `owner_id`，但一次只能对应一种 `user_id_type`。

由于本功能需要同一条输出里同时包含：

- `owner_open_id`
- `owner_union_id`

因此必须用列表接口拿群集合，再用详情接口做聚合。

## 5. 数据结构设计

建议在 `internal/feishu/` 暴露新的稳定返回结构：

```go
type ChatOwner struct {
    OpenID  string
    UnionID string
}

type ChatSummary struct {
    ChatID string
    Name   string
    Owner  ChatOwner
}
```

为了避免扩大现有 `Messenger` 接口并打断 `send text` / `send file` 的测试替身，本次新增单独接口：

```go
type ChatLister interface {
    ListChats(ctx context.Context) ([]ChatSummary, error)
}
```

`*feishu.Client` 同时实现：

- `Messenger`
- `ChatLister`

CLI 侧新增独立依赖构造，不复用 `NewMessenger`：

- `NewChatLister func(config.Config) (feishu.ChatLister, error)`

## 6. 权限要求

本功能新增依赖以下权限之一：

- `im:chat`
- `im:chat:read`
- `im:chat:readonly`

本次不依赖：

- `contact:user.employee_id:readonly`

因为第一版不返回 `user_id`。

## 7. 失败策略

### 7.1 参数错误

例如：

- `--format` 非法

处理方式：

- 返回退出码 `2`
- 不发起 API 请求

### 7.2 配置错误

例如：

- 缺少 `app_id`
- 缺少 `app_secret`
- 配置文件权限不安全

处理方式：

- 复用现有配置加载逻辑
- 返回退出码 `3`

### 7.3 飞书 API 错误

例如：

- 机器人未启用
- 应用未安装
- 机器人不在群内
- 权限不足

处理方式：

- 任一飞书请求失败，整条命令失败
- 返回退出码 `10`
- 透传清晰的飞书错误码和错误消息

### 7.4 群主为空

如果飞书接口不返回 `owner_id`，例如群主为机器人，则：

- 保留该群条目
- `owner.open_id` 和 `owner.union_id` 输出为空字符串

## 8. 实现边界

- CLI 内部自动翻页拿全量群列表
- 第一版不暴露分页参数
- 第一版不暴露并发参数
- 第一版可以先顺序聚合群详情，保持实现简单可靠
- 默认表格输出不能使用 `fmt` 固定宽度硬编码；应使用 `text/tabwriter` 或等效的列对齐方案，并用中文群名做验证

## 9. 测试要求

至少覆盖：

- `list chats` 默认表格输出
- `list chats --format json` 的稳定 JSON 输出
- 多页群列表自动聚合
- 单个群成功合并 `open_id` 与 `union_id`
- 群主缺失时输出空字段
- 任一详情请求失败时整条命令失败
- `--format` 非法时返回退出码 `2`

## 10. 文档要求

README 与 `README_ZH.md` 需要补充：

- 新命令 `feishu list chats`
- JSON 用法示例
- 该功能依赖的群信息权限
- 说明群没有 `open_id`，群的稳定标识是 `chat_id`
