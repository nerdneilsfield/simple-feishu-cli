package feishu

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unsafe"

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

	if fake.req == nil {
		t.Fatal("SendText() did not call Create")
	}
	if got := fake.receiveIDType(); got != larkim.ReceiveIdTypeOpenId {
		t.Fatalf("receive_id_type = %q, want %q", got, larkim.ReceiveIdTypeOpenId)
	}

	body := fake.req.Body
	if body == nil {
		t.Fatal("SendText() request body = nil")
	}

	if got := larkcore.StringValue(body.ReceiveId); got != "ou_xxx" {
		t.Fatalf("request receive_id = %q, want %q", got, "ou_xxx")
	}
	if got := larkcore.StringValue(body.MsgType); got != larkim.MsgTypeText {
		t.Fatalf("request msg_type = %q, want %q", got, larkim.MsgTypeText)
	}
	if got := larkcore.StringValue(body.Content); !strings.Contains(got, `"text":"hello"`) {
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

	if fileAPI.req == nil || fileAPI.req.Body == nil {
		t.Fatal("SendFile() did not build file upload request")
	}
	if got := larkcore.StringValue(fileAPI.req.Body.FileName); got != "report.pdf" {
		t.Fatalf("upload file_name = %q, want %q", got, "report.pdf")
	}
	if got := larkcore.StringValue(fileAPI.req.Body.FileType); got != "pdf" {
		t.Fatalf("upload file_type = %q, want %q", got, "pdf")
	}
	if fileAPI.req.Body.File == nil {
		t.Fatal("upload file content = nil")
	}

	if messageAPI.req == nil || messageAPI.req.Body == nil {
		t.Fatal("SendFile() did not build file message request")
	}
	if got := messageAPI.receiveIDType(); got != larkim.ReceiveIdTypeOpenId {
		t.Fatalf("message receive_id_type = %q, want %q", got, larkim.ReceiveIdTypeOpenId)
	}
	if got := larkcore.StringValue(messageAPI.req.Body.MsgType); got != "file" {
		t.Fatalf("message msg_type = %q, want %q", got, "file")
	}
	if got := larkcore.StringValue(messageAPI.req.Body.Content); !strings.Contains(got, `"file_key":"file_xxx"`) {
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
	if fileAPI.req != nil {
		t.Fatal("SendFile() attempted upload for nonexistent path")
	}
	if messageAPI.req != nil {
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
	if fileAPI.req != nil {
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
	req  *larkim.CreateMessageReq
	resp *larkim.CreateMessageResp
	err  error
}

func (f *fakeMessageService) Create(_ context.Context, req *larkim.CreateMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error) {
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}

func (f *fakeMessageService) receiveIDType() string {
	return queryParamValue(f.req, "receive_id_type")
}

type fakeFileService struct {
	req  *larkim.CreateFileReq
	resp *larkim.CreateFileResp
	err  error
}

func (f *fakeFileService) Create(_ context.Context, req *larkim.CreateFileReq, _ ...larkcore.RequestOptionFunc) (*larkim.CreateFileResp, error) {
	f.req = req
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

func queryParamValue(req interface{}, key string) string {
	if req == nil {
		return ""
	}

	v := reflect.ValueOf(req)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return ""
	}

	apiReqField := v.Elem().FieldByName("apiReq")
	if !apiReqField.IsValid() || apiReqField.IsNil() {
		return ""
	}
	apiReqValue := reflect.NewAt(apiReqField.Type(), unsafe.Pointer(apiReqField.UnsafeAddr())).Elem()
	queryParamsField := apiReqValue.Elem().FieldByName("QueryParams")
	queryParamsValue := reflect.NewAt(queryParamsField.Type(), unsafe.Pointer(queryParamsField.UnsafeAddr())).Elem().Interface().(larkcore.QueryParams)
	values := queryParamsValue[key]
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
