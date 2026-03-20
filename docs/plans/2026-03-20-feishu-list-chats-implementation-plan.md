# Feishu List Chats Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `feishu list chats` to list chats joined by the current app bot, returning each chat's `chat_id`, name, and owner `open_id` / `union_id`.

**Architecture:** Extend the Feishu client wrapper with chat listing and chat detail aggregation. Keep `Messenger` unchanged and add a separate `ChatLister` capability so existing send-command test doubles keep compiling. Add a new `list chats` CLI command that defaults to table output and optionally emits stable JSON, reusing the existing config loading and error classification patterns.

**Tech Stack:** Go, Cobra, larksuite/oapi-sdk-go/v3, Go testing package

---

### Task 1: Add chat-domain types and client interface methods

**Files:**
- Modify: `internal/feishu/client.go`
- Test: `internal/feishu/client_test.go`

**Step 1: Write the failing tests**

Add tests that describe the new Feishu client surface:
- `ListChats(ctx)` returns `[]ChatSummary`
- `ChatSummary` includes `ChatID`, `Name`, `Owner.OpenID`, `Owner.UnionID`

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/feishu -run 'TestListChats' -v
```

Expected: FAIL because the types and method do not exist yet.

**Step 3: Write the minimal implementation**

In `internal/feishu/client.go`:
- Add `ChatOwner`
- Add `ChatSummary`
- Add a new exported `ChatLister` interface with `ListChats(ctx context.Context) ([]ChatSummary, error)`
- Keep `Messenger` unchanged
- Make sure `*Client` is positioned to satisfy `ChatLister` without changing existing send-command fakes

Do not implement API calls yet beyond placeholders required for compilation.

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/feishu -run 'TestListChats' -v
```

Expected: PASS for compilation-level tests.

**Step 5: Commit**

```bash
git add internal/feishu/client.go internal/feishu/client_test.go
git commit -m "feat: add list chats client contract"
```

### Task 2: Add SDK wrappers for chat list and chat get

**Files:**
- Modify: `internal/feishu/client.go`
- Test: `internal/feishu/client_test.go`

**Step 1: Write the failing tests**

Add fake chat list and chat get services that verify:
- list endpoint pagination is followed until `has_more=false`
- detail endpoint is called once with `user_id_type=open_id` and once with `user_id_type=union_id` for each chat

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/feishu -run 'TestListChatsAggregatesOwnerIDsAcrossPages' -v
```

Expected: FAIL because chat APIs are not wired.

**Step 3: Write the minimal implementation**

In `internal/feishu/client.go`:
- Add chat list and chat get API interfaces backed by the SDK
- Add those dependencies to `Client`
- Wire them in `NewClient`
- Implement internal pagination loop for `/im/v1/chats`
- Implement per-chat detail fetch for `open_id` and `union_id`
- Merge results into `[]ChatSummary`

Keep implementation sequential in the first version.

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/feishu -run 'TestListChatsAggregatesOwnerIDsAcrossPages' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/feishu/client.go internal/feishu/client_test.go
git commit -m "feat: implement feishu chat listing"
```

### Task 3: Add error-path coverage for list chats aggregation

**Files:**
- Modify: `internal/feishu/client_test.go`

**Step 1: Write the failing tests**

Add tests for:
- list chats API error becomes `*APIError`
- chat detail open_id fetch error fails whole command path
- chat detail union_id fetch error fails whole command path
- missing owner data produces empty owner fields instead of failure

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/feishu -run 'TestListChats' -v
```

Expected: FAIL on unimplemented edge cases.

**Step 3: Write the minimal implementation**

Adjust `internal/feishu/client.go` only as needed to satisfy the tests while preserving the strict failure policy.

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/feishu -run 'TestListChats' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/feishu/client.go internal/feishu/client_test.go
git commit -m "test: cover list chats aggregation failures"
```

### Task 4: Add CLI dependency wiring and command skeleton for `list chats`

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/root_test.go`
- Test: `cmd/feishu/main_test.go`

**Step 1: Write the failing tests**

Add CLI tests covering:
- `feishu list chats` is discoverable in help
- invalid `--format` returns exit code `2`
- command loads config and calls the list method

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/cli -run 'TestListChats' -v
```

Expected: FAIL because command does not exist.

**Step 3: Write the minimal implementation**

In `internal/cli/root.go`:
- Add a `list` parent command
- Add a `chats` subcommand
- Reuse existing config loading flow
- Add `Deps.NewChatLister func(config.Config) (feishu.ChatLister, error)`
- Keep existing `Deps.NewMessenger` untouched so current send tests continue to compile
- Update any root/main tests affected by the new dependency shape

Do not finalize output formatting yet.

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/cli -run 'TestListChats' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: add list chats command"
```

### Task 5: Implement table and JSON output for `list chats`

**Files:**
- Modify: `internal/cli/root.go`
- Possibly modify: `internal/cli/output helpers` within the same file
- Test: `internal/cli/root_test.go`

**Step 1: Write the failing tests**

Add tests for:
- default output is a table with headers `CHAT_ID`, `NAME`, `OWNER_OPEN_ID`, `OWNER_UNION_ID`
- Chinese group names still align acceptably in the default table output
- `--format json` emits stable JSON with `items`
- empty list still prints a valid empty table or JSON object

**Step 2: Run the targeted test to verify it fails**

Run:
```bash
go test ./internal/cli -run 'TestListChats(Output|JSON|Table)' -v
```

Expected: FAIL because formatting is incomplete.

**Step 3: Write the minimal implementation**

Implement:
- default table formatter using `text/tabwriter` or an equivalent non-fixed-column approach
- JSON formatter using `encoding/json`
- stable field naming matching the design doc

**Step 4: Run the targeted test to verify it passes**

Run:
```bash
go test ./internal/cli -run 'TestListChats(Output|JSON|Table)' -v
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/root_test.go
git commit -m "feat: add list chats output formats"
```

### Task 6: Update README and README_ZH

**Files:**
- Modify: `README.md`
- Modify: `README_ZH.md`

**Step 1: Write the doc changes**

Add:
- `feishu list chats` examples
- `--format json` example
- required permissions for chat listing
- explicit note that groups use `chat_id`, not `open_id`

**Step 2: Verify the docs read cleanly**

Check relevant sections manually:
```bash
rg -n "list chats|chat_id|owner_open_id|owner_union_id" README.md README_ZH.md
```

Expected: both READMEs include the new command and wording is consistent.

**Step 3: Commit**

```bash
git add README.md README_ZH.md
git commit -m "docs: add list chats usage"
```

### Task 7: Run full verification and prepare for review

**Files:**
- Modify: none unless verification finds issues

**Step 1: Run formatting and tests**

Run:
```bash
gofmt -w internal/feishu/client.go internal/feishu/client_test.go internal/cli/root.go internal/cli/root_test.go
make check
go test ./...
```

Expected: all commands pass.

**Step 2: Perform manual CLI smoke check if credentials are available**

Run:
```bash
./feishu list chats
./feishu list chats --format json
```

Expected: table output and JSON output both succeed against a real tenant.

**Step 3: Commit any final fixes**

```bash
git add .
git commit -m "fix: finalize list chats command"
```
