package feishu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/config"
)

func TestListChatsExposesChatSummaryContract(t *testing.T) {
	var _ ChatLister = (*Client)(nil)

	want := []ChatSummary{
		{
			ChatID: "oc_123",
			Name:   "Engineering",
			Owner: ChatOwner{
				OpenID:  "ou_123",
				UnionID: "on_123",
			},
		},
	}
	got := []ChatSummary{
		{
			ChatID: "oc_123",
			Name:   "Engineering",
			Owner: ChatOwner{
				OpenID:  "ou_123",
				UnionID: "on_123",
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("chat summaries = %#v, want %#v", got, want)
	}
}

func TestListChatsReturnsClientErrorWhenChatListingIsUnconfigured(t *testing.T) {
	_, err := (&Client{}).ListChats(context.Background())
	if err == nil {
		t.Fatal("ListChats() error = nil, want client error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("ListChats() error = %T, want *ClientError", err)
	}
}

func TestListChatsFailsClosedWhenHasMoreWithoutPageToken(t *testing.T) {
	client := &Client{
		chatListAPI: &fakeChatListService{
			resps: []*larkim.ListChatResp{
				{
					CodeError: larkcore.CodeError{Code: 0},
					Data: &larkim.ListChatRespData{
						HasMore: larkcore.BoolPtr(true),
					},
				},
			},
		},
		chatGetAPI: &fakeChatGetService{resps: map[string]*larkim.GetChatResp{}},
	}

	_, err := client.ListChats(context.Background())
	if err == nil {
		t.Fatal("ListChats() error = nil, want api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("ListChats() error = %T, want *APIError", err)
	}
	if apiErr.Op != "list_chats" {
		t.Fatalf("ListChats() op = %q, want %q", apiErr.Op, "list_chats")
	}
	if !strings.Contains(apiErr.Message, "page_token") {
		t.Fatalf("ListChats() message = %q, want page_token guidance", apiErr.Message)
	}
}

func TestGetChatOwnerIDReturnsAPIErrorWhenResponseDataMissing(t *testing.T) {
	client := &Client{
		chatGetAPI: &fakeChatGetService{
			resps: map[string]*larkim.GetChatResp{
				"oc_first|open_id": {
					CodeError: larkcore.CodeError{Code: 0},
				},
			},
		},
	}

	_, err := client.getChatOwnerID(context.Background(), "oc_first", larkim.UserIdTypeGetChatOpenId)
	if err == nil {
		t.Fatal("getChatOwnerID() error = nil, want api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("getChatOwnerID() error = %T, want *APIError", err)
	}
	if apiErr.Op != "get_chat" {
		t.Fatalf("getChatOwnerID() op = %q, want %q", apiErr.Op, "get_chat")
	}
	if apiErr.Message != "missing response data" {
		t.Fatalf("getChatOwnerID() message = %q, want %q", apiErr.Message, "missing response data")
	}
}

func TestListChatsAggregatesOwnerIDsAcrossPages(t *testing.T) {
	listAPI := &fakeChatListService{
		resps: []*larkim.ListChatResp{
			{
				CodeError: larkcore.CodeError{Code: 0},
				Data: &larkim.ListChatRespData{
					Items: []*larkim.ListChat{
						larkim.NewListChatBuilder().
							ChatId("oc_first").
							Name("Engineering").
							Build(),
					},
					HasMore:   larkcore.BoolPtr(true),
					PageToken: larkcore.StringPtr("page-2"),
				},
			},
			{
				CodeError: larkcore.CodeError{Code: 0},
				Data: &larkim.ListChatRespData{
					Items: []*larkim.ListChat{
						larkim.NewListChatBuilder().
							ChatId("oc_second").
							Name("Operations").
							Build(),
					},
					HasMore: larkcore.BoolPtr(false),
				},
			},
		},
	}
	getAPI := &fakeChatGetService{
		resps: map[string]*larkim.GetChatResp{
			"oc_first|open_id": {
				CodeError: larkcore.CodeError{Code: 0},
				Data:      &larkim.GetChatRespData{OwnerId: larkcore.StringPtr("ou_first")},
			},
			"oc_first|union_id": {
				CodeError: larkcore.CodeError{Code: 0},
				Data:      &larkim.GetChatRespData{OwnerId: larkcore.StringPtr("on_first")},
			},
			"oc_second|open_id": {
				CodeError: larkcore.CodeError{Code: 0},
				Data:      &larkim.GetChatRespData{OwnerId: larkcore.StringPtr("ou_second")},
			},
			"oc_second|union_id": {
				CodeError: larkcore.CodeError{Code: 0},
				Data:      &larkim.GetChatRespData{OwnerId: larkcore.StringPtr("on_second")},
			},
		},
	}
	client := &Client{chatListAPI: listAPI, chatGetAPI: getAPI}

	got, err := client.ListChats(context.Background())
	if err != nil {
		t.Fatalf("ListChats() error = %v", err)
	}

	want := []ChatSummary{
		{
			ChatID: "oc_first",
			Name:   "Engineering",
			Owner:  ChatOwner{OpenID: "ou_first", UnionID: "on_first"},
		},
		{
			ChatID: "oc_second",
			Name:   "Operations",
			Owner:  ChatOwner{OpenID: "ou_second", UnionID: "on_second"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListChats() = %#v, want %#v", got, want)
	}

	if len(listAPI.calls) != 2 {
		t.Fatalf("list request count = %d, want %d", len(listAPI.calls), 2)
	}
	if got := listAPI.calls[0].UserIDType; got != larkim.UserIdTypeListChatOpenId {
		t.Fatalf("first list user_id_type = %q, want %q", got, larkim.UserIdTypeListChatOpenId)
	}
	if got := listAPI.calls[0].PageToken; got != "" {
		t.Fatalf("first list page_token = %q, want empty", got)
	}
	if got := listAPI.calls[1].PageToken; got != "page-2" {
		t.Fatalf("second list page_token = %q, want %q", got, "page-2")
	}

	wantGetCalls := []string{"oc_first|open_id", "oc_first|union_id", "oc_second|open_id", "oc_second|union_id"}
	if !reflect.DeepEqual(getAPI.calls, wantGetCalls) {
		t.Fatalf("get chat calls = %#v, want %#v", getAPI.calls, wantGetCalls)
	}
}

func TestNormalizeAppCredentialsTrimsWhitespace(t *testing.T) {
	appID, appSecret := normalizeAppCredentials(config.Config{
		AppID:     "  cli_xxx  ",
		AppSecret: "  secret  ",
	})

	if appID != "cli_xxx" {
		t.Fatalf("normalized app id = %q, want %q", appID, "cli_xxx")
	}
	if appSecret != "secret" {
		t.Fatalf("normalized app secret = %q, want %q", appSecret, "secret")
	}
}

func TestNewClientWiresSendDependencies(t *testing.T) {
	var _ Messenger = (*Client)(nil)
	var _ ChatLister = (*Client)(nil)

	client, err := NewClient(config.Config{
		AppID:     "  cli_xxx  ",
		AppSecret: "  secret  ",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if client.messageAPI == nil {
		t.Fatal("NewClient() returned nil message api")
	}
	if client.fileAPI == nil {
		t.Fatal("NewClient() returned nil file api")
	}
	if client.chatListAPI == nil {
		t.Fatal("NewClient() returned nil chat list api")
	}
	if client.chatGetAPI == nil {
		t.Fatal("NewClient() returned nil chat get api")
	}
}

func TestNewClientRejectsMissingCredentials(t *testing.T) {
	_, err := NewClient(config.Config{})
	if err == nil {
		t.Fatal("NewClient() error = nil, want missing credential error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("NewClient() error = %T, want *ClientError", err)
	}
}

func TestWrapErrorHandlesCodeError(t *testing.T) {
	err := wrapError("send_text", larkcore.CodeError{
		Code: 99991663,
		Msg:  "insufficient permission",
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("wrapError() error = %T, want *APIError", err)
	}

	if apiErr.Code != 99991663 || apiErr.Message != "insufficient permission" {
		t.Fatalf("wrapError() = %#v, want code/message preserved", apiErr)
	}
}

func TestSendTextBuildsRequestAndReturnsMessageResult(t *testing.T) {
	fake := &fakeMessageService{
		resp: &larkim.CreateMessageResp{
			CodeError: larkcore.CodeError{Code: 0},
			Data: &larkim.CreateMessageRespData{
				MessageId: larkcore.StringPtr("om_xxx"),
				MsgType:   larkcore.StringPtr(larkim.MsgTypeText),
			},
		},
	}
	client := &Client{messageAPI: fake}

	result, err := client.SendText(context.Background(), TextMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		Text:          "hello",
	})
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	if result.MessageID != "om_xxx" || result.MsgType != larkim.MsgTypeText {
		t.Fatalf("SendText() result = %#v", result)
	}

	if got := fake.input.ReceiveIDType; got != larkim.ReceiveIdTypeOpenId {
		t.Fatalf("receive_id_type = %q, want %q", got, larkim.ReceiveIdTypeOpenId)
	}
	if got := fake.input.ReceiveID; got != "ou_xxx" {
		t.Fatalf("request receive_id = %q, want %q", got, "ou_xxx")
	}
	if got := fake.input.MsgType; got != larkim.MsgTypeText {
		t.Fatalf("request msg_type = %q, want %q", got, larkim.MsgTypeText)
	}
	if got := fake.input.Content; !strings.Contains(got, `"text":"hello"`) {
		t.Fatalf("request content = %q, want text payload", got)
	}
}

func TestSendTextWrapsSDKErrors(t *testing.T) {
	client := &Client{
		messageAPI: &fakeMessageService{
			err: larkcore.CodeError{Code: 99991663, Msg: "insufficient permission"},
		},
	}

	_, err := client.SendText(context.Background(), TextMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		Text:          "hello",
	})
	if err == nil {
		t.Fatal("SendText() error = nil, want wrapped api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SendText() error = %T, want *APIError", err)
	}
	if apiErr.Code != 99991663 {
		t.Fatalf("SendText() error code = %d, want %d", apiErr.Code, 99991663)
	}
}

func TestSendTextWrapsUnsuccessfulResponses(t *testing.T) {
	client := &Client{
		messageAPI: &fakeMessageService{
			resp: &larkim.CreateMessageResp{
				CodeError: larkcore.CodeError{Code: 99991663, Msg: "insufficient permission"},
			},
		},
	}

	_, err := client.SendText(context.Background(), TextMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		Text:          "hello",
	})
	if err == nil {
		t.Fatal("SendText() error = nil, want wrapped api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SendText() error = %T, want *APIError", err)
	}
	if apiErr.Code != 99991663 {
		t.Fatalf("SendText() error code = %d, want %d", apiErr.Code, 99991663)
	}
}

func TestSendTextReturnsStructuredClientErrorWhenMessageAPIUnavailable(t *testing.T) {
	_, err := (&Client{}).SendText(context.Background(), TextMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		Text:          "hello",
	})
	if err == nil {
		t.Fatal("SendText() error = nil, want client error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("SendText() error = %T, want *ClientError", err)
	}
}

func TestSendFileUploadsFileAndSendsMessage(t *testing.T) {
	path := writeTempFile(t, "report.pdf", "hello")
	fileAPI := &fakeFileService{
		resp: &larkim.CreateFileResp{
			CodeError: larkcore.CodeError{Code: 0},
			Data: &larkim.CreateFileRespData{
				FileKey: larkcore.StringPtr("file_xxx"),
			},
		},
	}
	messageAPI := &fakeMessageService{
		resp: &larkim.CreateMessageResp{
			CodeError: larkcore.CodeError{Code: 0},
			Data: &larkim.CreateMessageRespData{
				MessageId: larkcore.StringPtr("om_file"),
				MsgType:   larkcore.StringPtr("file"),
			},
		},
	}
	client := &Client{
		fileAPI:    fileAPI,
		messageAPI: messageAPI,
	}

	result, err := client.SendFile(context.Background(), FileMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		FilePath:      path,
	})
	if err != nil {
		t.Fatalf("SendFile() error = %v", err)
	}

	if result.MessageID != "om_file" || result.MsgType != "file" {
		t.Fatalf("SendFile() result = %#v", result)
	}

	if got := fileAPI.input.FileName; got != "report.pdf" {
		t.Fatalf("upload file_name = %q, want %q", got, "report.pdf")
	}
	if got := fileAPI.input.FileType; got != "pdf" {
		t.Fatalf("upload file_type = %q, want %q", got, "pdf")
	}
	if got := fileAPI.input.FilePath; got != path {
		t.Fatalf("upload file_path = %q, want %q", got, path)
	}

	if got := messageAPI.input.ReceiveIDType; got != larkim.ReceiveIdTypeOpenId {
		t.Fatalf("message receive_id_type = %q, want %q", got, larkim.ReceiveIdTypeOpenId)
	}
	if got := messageAPI.input.MsgType; got != "file" {
		t.Fatalf("message msg_type = %q, want %q", got, "file")
	}
	if got := messageAPI.input.Content; !strings.Contains(got, `"file_key":"file_xxx"`) {
		t.Fatalf("message content = %q, want file_key payload", got)
	}
}

func TestUploadFileTypeFallsBackToStreamForUnsupportedExtensions(t *testing.T) {
	testCases := map[string]string{
		"notes.txt":   larkim.FileTypeStream,
		"report.docx": larkim.FileTypeStream,
		"archive":     larkim.FileTypeStream,
		"slides.PPT":  larkim.FileTypePpt,
		"report.PDF":  larkim.FileTypePdf,
	}

	for fileName, want := range testCases {
		if got := uploadFileType(fileName); got != want {
			t.Fatalf("uploadFileType(%q) = %q, want %q", fileName, got, want)
		}
	}
}

func TestSendFileReturnsLocalFileErrorForNonexistentPath(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.pdf")
	fileAPI := &fakeFileService{}
	messageAPI := &fakeMessageService{}
	client := &Client{
		fileAPI:    fileAPI,
		messageAPI: messageAPI,
	}

	_, err := client.SendFile(context.Background(), FileMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		FilePath:      missingPath,
	})
	if err == nil {
		t.Fatal("SendFile() error = nil, want local file error")
	}

	var fileErr *LocalFileError
	if !errors.As(err, &fileErr) {
		t.Fatalf("SendFile() error = %T, want *LocalFileError", err)
	}
	if fileErr.Op != "stat_file" {
		t.Fatalf("SendFile() op = %q, want %q", fileErr.Op, "stat_file")
	}
	if fileErr.Path != missingPath {
		t.Fatalf("SendFile() path = %q, want %q", fileErr.Path, missingPath)
	}
	if !os.IsNotExist(fileErr.Err) {
		t.Fatalf("SendFile() wrapped err = %v, want not-exist error", fileErr.Err)
	}
	if fileAPI.input != (createFileInput{}) {
		t.Fatal("SendFile() attempted upload for nonexistent path")
	}
	if messageAPI.input != (createMessageInput{}) {
		t.Fatal("SendFile() attempted message send for nonexistent path")
	}
}

func TestSendFileReturnsAPIErrorWhenPostUploadSendFails(t *testing.T) {
	path := writeTempFile(t, "report.pdf", "hello")
	client := &Client{
		fileAPI: &fakeFileService{
			resp: &larkim.CreateFileResp{
				CodeError: larkcore.CodeError{Code: 0},
				Data: &larkim.CreateFileRespData{
					FileKey: larkcore.StringPtr("file_xxx"),
				},
			},
		},
		messageAPI: &fakeMessageService{
			resp: &larkim.CreateMessageResp{
				CodeError: larkcore.CodeError{Code: 99991663, Msg: "insufficient permission"},
			},
		},
	}

	_, err := client.SendFile(context.Background(), FileMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		FilePath:      path,
	})
	if err == nil {
		t.Fatal("SendFile() error = nil, want api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SendFile() error = %T, want *APIError", err)
	}
	if apiErr.Op != "send_file" {
		t.Fatalf("SendFile() op = %q, want %q", apiErr.Op, "send_file")
	}
	if apiErr.Code != 99991663 {
		t.Fatalf("SendFile() code = %d, want %d", apiErr.Code, 99991663)
	}
}

func TestSendFileFailsBeforeUploadWhenMessageAPIUnavailable(t *testing.T) {
	path := writeTempFile(t, "report.pdf", "hello")
	fileAPI := &fakeFileService{}
	client := &Client{fileAPI: fileAPI}

	_, err := client.SendFile(context.Background(), FileMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		FilePath:      path,
	})
	if err == nil {
		t.Fatal("SendFile() error = nil, want client error")
	}

	var clientErr *ClientError
	if !errors.As(err, &clientErr) {
		t.Fatalf("SendFile() error = %T, want *ClientError", err)
	}
	if fileAPI.input != (createFileInput{}) {
		t.Fatal("SendFile() uploaded file before validating client usability")
	}
}

func TestSendFileWrapsUploadErrors(t *testing.T) {
	path := writeTempFile(t, "report.pdf", "hello")
	client := &Client{
		fileAPI: &fakeFileService{
			err: larkcore.CodeError{Code: 99991663, Msg: "insufficient permission"},
		},
		messageAPI: &fakeMessageService{},
	}

	_, err := client.SendFile(context.Background(), FileMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		FilePath:      path,
	})
	if err == nil {
		t.Fatal("SendFile() error = nil, want wrapped api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SendFile() error = %T, want *APIError", err)
	}
	if apiErr.Code != 99991663 {
		t.Fatalf("SendFile() error code = %d, want %d", apiErr.Code, 99991663)
	}
}

func TestSendFileWrapsUnsuccessfulUploadResponses(t *testing.T) {
	path := writeTempFile(t, "report.pdf", "hello")
	client := &Client{
		fileAPI: &fakeFileService{
			resp: &larkim.CreateFileResp{
				CodeError: larkcore.CodeError{Code: 99991663, Msg: "insufficient permission"},
			},
		},
		messageAPI: &fakeMessageService{},
	}

	_, err := client.SendFile(context.Background(), FileMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		FilePath:      path,
	})
	if err == nil {
		t.Fatal("SendFile() error = nil, want wrapped api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SendFile() error = %T, want *APIError", err)
	}
	if apiErr.Code != 99991663 {
		t.Fatalf("SendFile() error code = %d, want %d", apiErr.Code, 99991663)
	}
}

type fakeMessageService struct {
	input createMessageInput
	resp  *larkim.CreateMessageResp
	err   error
}

type fakeChatListCall struct {
	UserIDType string
	PageToken  string
}

type fakeChatListService struct {
	calls []fakeChatListCall
	resps []*larkim.ListChatResp
	err   error
}

func (f *fakeChatListService) List(_ context.Context, input listChatsPageInput) (*larkim.ListChatResp, error) {
	f.calls = append(f.calls, fakeChatListCall{
		UserIDType: input.UserIDType,
		PageToken:  input.PageToken,
	})
	if f.err != nil {
		return nil, f.err
	}
	if len(f.resps) == 0 {
		return nil, errors.New("unexpected chat list call")
	}
	resp := f.resps[0]
	f.resps = f.resps[1:]
	return resp, nil
}

type fakeChatGetService struct {
	resps map[string]*larkim.GetChatResp
	err   error
	calls []string
}

func (f *fakeChatGetService) Get(_ context.Context, input getChatInput) (*larkim.GetChatResp, error) {
	key := input.ChatID + "|" + input.UserIDType
	f.calls = append(f.calls, key)
	if f.err != nil {
		return nil, f.err
	}
	resp, ok := f.resps[key]
	if !ok {
		return nil, errors.New("unexpected get chat call: " + key)
	}
	return resp, nil
}

func (f *fakeMessageService) Create(_ context.Context, input createMessageInput) (*larkim.CreateMessageResp, error) {
	f.input = input
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

type fakeFileService struct {
	input createFileInput
	resp  *larkim.CreateFileResp
	err   error
}

func (f *fakeFileService) Create(_ context.Context, input createFileInput) (*larkim.CreateFileResp, error) {
	f.input = input
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
