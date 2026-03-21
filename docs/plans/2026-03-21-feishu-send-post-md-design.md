# Feishu Send Post and Markdown Design

日期：2026-03-21

## 1. 目标

为现有 `feishu` CLI 增加两类发送能力：

- `feishu send post`：直接发送飞书原生 `post` 富文本消息
- `feishu send md`：读取本地 Markdown 文件，转换为飞书 `post` 富文本后发送

新增能力的主要目的：

- 在不改变现有 `send text` 语义的前提下，支持真正的富文本消息发送
- 提供一个底层原语 `send post`，用于直接发送飞书原生结构
- 提供一个更高层的便利入口 `send md`，用于从常用 Markdown 子集生成飞书富文本

## 2. 非目标

本次功能不包含：

- 不修改 `send text` 为自动支持富文本
- 不支持图片、表格、HTML 原始块、任务列表 checkbox
- 不支持多语言 `zh_cn` / `en_us` 同时生成
- 不支持从 Markdown 自动上传图片、文件、视频等资源
- 不支持读取远程 URL 内容
- 不支持富文本卡片 `interactive`

## 3. CLI 设计

### 3.1 新增命令

```bash
feishu send post --to-type chat_id --to oc_xxx --file ./post.json
feishu send md --to-type chat_id --to oc_xxx --file ./notice.md
```

### 3.2 参数

`send post`：

- `--to-type`
- `--to`
- `--file`

`send md`：

- `--to-type`
- `--to`
- `--file`

其中：

- `--file` 对 `send post` 表示本地 JSON 文件
- `--file` 对 `send md` 表示本地 Markdown 文件

### 3.3 输出约定

成功时沿用现有发送命令的稳定输出：

```text
message_id=om_xxx
msg_type=post
receive_id=oc_xxx
receive_id_type=chat_id
```

失败时遵循现有退出码约定：

- `2`：参数错误、JSON 结构错误、Markdown 含不支持语法
- `3`：配置或凭证错误
- `4`：本地文件不可读
- `10`：飞书 API 错误

## 4. 飞书接口设计

新增能力仍然复用已有发送消息接口：

- `POST /open-apis/im/v1/messages`

区别只在：

- `msg_type=post`
- `content` 为飞书富文本 JSON 结构序列化后的字符串

因此在 `internal/feishu/` 新增独立能力接口，而不是直接扩展现有 `Messenger`：

```go
type PostSender interface {
    SendPost(ctx context.Context, input PostMessageInput) (MessageResult, error)
}
```

其中：

```go
type PostMessageInput struct {
    ReceiveIDType string
    ReceiveID     string
    Content       string
}
```

`*feishu.Client` 同时实现：

- `Messenger`
- `PostSender`

CLI 侧新增独立依赖构造：

- `NewPostSender func(config.Config) (feishu.PostSender, error)`

## 5. `send post` 设计

### 5.1 输入

本地 JSON 文件内容直接表示飞书 `post` 的内容对象，例如：

```json
{
  "zh_cn": {
    "title": "通知",
    "content": [
      [
        {"tag": "text", "text": "hello"}
      ]
    ]
  }
}
```

### 5.2 行为

- 读取文件
- 校验 JSON 可解析
- 校验顶层是对象
- 原样序列化为单行 JSON 字符串后发送

`send post` 不负责语义级深校验；语法合法但不符合飞书 `post` 结构的情况，由飞书 API 返回错误。

## 6. `send md` 设计

### 6.1 输入

读取本地 Markdown 文件，并转换为飞书 `post`。

### 6.2 输出结构

第一版转换器必须输出飞书 `post` 内容对象，精确包裹为：

```json
{
  "zh_cn": {
    "title": "标题",
    "content": [
      [
        {"tag": "text", "text": "正文"}
      ]
    ]
  }
}
```

第一版只生成：

- `zh_cn`

如果 Markdown 中存在一级标题 `# 标题`，则：

- 用第一个一级标题作为 `zh_cn.title`

如果不存在一级标题，则：

- 标题为空字符串

### 6.3 支持的 Markdown 子集

支持：

- 一级标题 `# heading`
- 普通段落
- `**bold**`
- `*italic*`
- `~~strike~~`
- `[text](url)`
- fenced code block
- 引用 `>`
- 有序 / 无序列表

其中转换策略为：

- 普通文本、粗体、斜体、删除线：转成飞书 `post` 的 `text` 节点，并使用官方支持的 `style` 值：`bold`、`italic`、`lineThrough`
- 链接：转成飞书 `post` 的 `a` 节点，字段为 `text` + `href`
- fenced code block：转成飞书 `post` 的 `code_block` 节点，字段为 `language` + `text`
- 引用、列表：转成飞书 `post` 的 `md` 节点，字段为 `text`，保留块级 Markdown 文本
- 行内代码：第一版退化为普通文本包反引号，不单独映射节点

这里的 `text`、`a`、`md`、`code_block` 都指飞书 `post` 官方支持的节点标签，而不是自定义结构。

### 6.4 不支持的语法

遇到以下语法时直接报错：

- 图片
- 表格
- HTML 原始块
- 任务列表
- 过于复杂的嵌套块结构

## 7. 库选型

Markdown 解析采用：

- `github.com/yuin/goldmark`

选择理由：

- Go 生态成熟稳定
- 提供 AST，便于做受控子集转换
- 比自行解析 Markdown 更可靠

## 8. 模块设计

建议新增：

```text
internal/markdown/
```

职责：

- 解析 Markdown AST
- 校验支持范围
- 转换为飞书 `post` 内容结构

建议暴露方法：

- `ConvertToFeishuPost(markdown []byte) ([]byte, error)`

其中返回值为飞书 `post` 内容对象的 JSON 字节，不直接负责发送。

## 9. 错误处理

### 9.1 参数错误

例如：

- 缺少 `--file`
- `--to-type` 非法
- Markdown 含不支持语法
- JSON 不是合法对象

处理方式：

- 本地直接报错
- 返回退出码 `2`

### 9.2 本地文件错误

例如：

- 文件不存在
- 文件不可读

处理方式：

- 返回退出码 `4`

### 9.3 飞书 API 错误

例如：

- 富文本结构不合法
- 权限不足
- 目标无效

处理方式：

- 返回退出码 `10`
- 原样透传飞书错误信息

## 10. 测试要求

至少覆盖：

- `send post` 成功发送并输出稳定字段
- `send post` 对非法 JSON 报参数错误
- `send md` 能把受支持 Markdown 转成 `post`
- `send md` 遇到不支持节点时报参数错误
- 一级标题能正确提取为 `zh_cn.title`
- 列表 / 引用能按预期转成 `md` 节点
- 代码块能转成 `code_block` 节点

## 11. 文档要求

README 与 `README_ZH.md` 需要补充：

- `send post` 示例
- `send md` 示例
- 支持的 Markdown 子集
- 明确说明 `send text` 仍然是文本消息，不等同于 `post`
