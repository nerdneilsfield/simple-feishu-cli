# Feishu Send Post and Markdown Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `feishu send post` for raw Feishu rich-text payloads and `feishu send md` for Markdown-to-post conversion and sending.

**Architecture:** Keep `send text` unchanged, add a separate `PostSender` capability in the Feishu client, then layer `send post` and `send md` on top without mutating the existing `Messenger` surface. Implement Markdown parsing in a dedicated internal package using `goldmark`, with a deliberately limited supported syntax set and explicit errors for unsupported nodes.

**Tech Stack:** Go, Cobra, larksuite/oapi-sdk-go/v3, goldmark, Go testing package

---

### Task 1: Add `PostSender` contract to the Feishu client

**Files:**
- Modify: `internal/feishu/client.go`
- Test: `internal/feishu/client_test.go`

**Step 1: Write the failing tests**

Add tests covering:
- `SendPost` builds a `msg_type=post` message request
- success returns the stable `MessageResult`
- malformed client state returns `*ClientError`
- existing `Messenger`-only fakes do not need to change

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/feishu -run 'TestSendPost' -v
```

Expected: FAIL because `SendPost` does not exist yet.

**Step 3: Write the minimal implementation**

In `internal/feishu/client.go`:
- add a new exported `PostSender` interface with `SendPost(ctx context.Context, input PostMessageInput) (MessageResult, error)`
- keep `Messenger` unchanged
- add `PostMessageInput`
- implement `SendPost` by reusing the existing message API path with `msg_type=post`
- ensure `*Client` satisfies `PostSender` without forcing unrelated CLI test doubles to grow new methods

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/feishu -run 'TestSendPost' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/feishu/client.go internal/feishu/client_test.go
git commit -m "feat: add send post client support"
```

### Task 2: Add CLI skeleton for `send post`

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`
- Test: `cmd/feishu/main_test.go` if needed

**Step 1: Write the failing tests**

Add tests covering:
- `feishu send post` is discoverable in help
- missing `--file` is exit code `2`
- malformed JSON is exit code `2`
- valid non-object JSON is exit code `2`
- unreadable or missing local file is exit code `4`
- command loads config, reads JSON file, and calls `SendPost`

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/cli -run 'TestSendPostCommand' -v
```

Expected: FAIL because the command does not exist.

**Step 3: Write the minimal implementation**

In `internal/cli/root.go`:
- add `newSendPostCmd`
- bind `--file`
- read the JSON file
- validate it parses as a JSON object
- add `Deps.NewPostSender func(config.Config) (feishu.PostSender, error)`
- keep existing `Deps.NewMessenger` untouched
- call `SendPost`

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/cli -run 'TestSendPostCommand' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go cmd/feishu/main_test.go
git commit -m "feat: add send post command"
```

### Task 3: Build Markdown-to-Feishu conversion package

**Files:**
- Create: `internal/markdown/convert.go`
- Create: `internal/markdown/convert_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Step 1: Write the failing tests**

Add conversion tests for:
- title extraction from first `# heading`
- plain paragraph conversion
- bold / italic / strike conversion
- link conversion
- fenced code block conversion to `code_block`
- list conversion to `md`
- quote conversion to `md`
- unsupported image/table/html nodes returning errors

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/markdown -v
```

Expected: FAIL because the package does not exist.

**Step 3: Write the minimal implementation**

Implement a converter using `goldmark` that:
- parses Markdown into AST
- extracts the first H1 as title
- emits the exact Feishu `post` envelope `{ "zh_cn": { "title": ..., "content": ... } }`
- converts supported nodes into official Feishu `post` node tags (`text`, `a`, `md`, `code_block`)
- returns explicit errors on unsupported nodes

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/markdown -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/markdown/convert.go internal/markdown/convert_test.go go.mod go.sum
git commit -m "feat: add markdown to feishu post converter"
```

### Task 4: Add CLI command for `send md`

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`

**Step 1: Write the failing tests**

Add tests covering:
- `feishu send md` is discoverable in help
- missing `--file` is exit code `2`
- supported Markdown file is converted and sent via `SendPost`
- unsupported Markdown returns exit code `2`

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/cli -run 'TestSendMDCommand' -v
```

Expected: FAIL because the command does not exist.

**Step 3: Write the minimal implementation**

In `internal/cli/root.go`:
- add `newSendMDCmd`
- read Markdown file
- call `internal/markdown.ConvertToFeishuPost`
- call `SendPost`

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/cli -run 'TestSendMDCommand' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: add send markdown command"
```

### Task 5: Update README and README_ZH

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: Write the doc changes**

Add:
- `send post` usage example
- `send md` usage example
- supported Markdown subset
- note that `send text` remains plain text and is separate from `post`

**Step 2: Verify the docs read cleanly**

Run:
```bash
rg -n "send post|send md|Markdown|post" README.md README_ZH.md
```

Expected: both READMEs include the new command docs and consistent wording.

**Step 3: Commit**

```bash
git add README.md README_ZH.md
git commit -m "docs: add rich text send usage"
```

### Task 6: Run full verification and prepare for review

**Files:**
- Modify: none unless verification finds issues

**Step 1: Run formatting and tests**

Run:
```bash
gofmt -w internal/feishu/client.go internal/feishu/client_test.go internal/cli/root.go internal/cli/root_test.go internal/markdown/convert.go internal/markdown/convert_test.go
make check
go test ./...
```

Expected: all commands pass.

**Step 2: Perform manual smoke checks if credentials are available**

Run:
```bash
cat > /tmp/feishu-post.json <<'JSON'
{
  "zh_cn": {
    "title": "通知",
    "content": [[{"tag":"text","text":"hello"}]]
  }
}
JSON

cat > /tmp/feishu-notice.md <<'MD'
# 通知

这是 **测试** 内容。
MD

./feishu send post --to-type chat_id --to oc_xxx --file /tmp/feishu-post.json
./feishu send md --to-type chat_id --to oc_xxx --file /tmp/feishu-notice.md
```

Expected: both commands succeed and return the stable message output fields.

**Step 3: Commit any final fixes**

```bash
git add .
git commit -m "fix: finalize rich text send commands"
```
