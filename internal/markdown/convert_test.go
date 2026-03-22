package markdown

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertToFeishuPostExtractsTitleFromFirstH1(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("# Notice\n\nHello world.\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"Notice","content":[[{"tag":"text","text":"Hello world."}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostPreservesSoftLineBreaksInParagraphs(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("first line\nsecond line\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"text","text":"first line\nsecond line"}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostPreservesSoftLineBreaksInStyledParagraphs(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("**first line\nsecond line**\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"text","text":"first line\nsecond line","style":["bold"]}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostConvertsParagraphAndInlineStyles(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("**bold** *italic* ~~strike~~\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"text","text":"bold","style":["bold"]},{"tag":"text","text":"italic","style":["italic"]},{"tag":"text","text":"strike","style":["lineThrough"]}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostConvertsLink(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("[OpenAI](https://openai.com)\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"a","text":"OpenAI","href":"https://openai.com"}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostConvertsFencedCodeBlock(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("# Title\n\n```go\nfmt.Println(\"hi\")\n```\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"Title","content":[[{"tag":"code_block","language":"go","text":"fmt.Println(\"hi\")"}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostOmitsUnsupportedFencedCodeLanguage(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("```text\nhello\n```\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"code_block","text":"hello"}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostConvertsListAsMDNode(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("- item 1\n- item 2\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"md","text":"- item 1\n- item 2"}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostConvertsQuoteAsMDNode(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("> quote\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	want := `{"zh_cn":{"title":"","content":[[{"tag":"md","text":"> quote"}]]}}`
	if string(got) != want {
		t.Fatalf("ConvertToFeishuPost() = %s, want %s", string(got), want)
	}
}

func TestConvertToFeishuPostRejectsUnsupportedNodes(t *testing.T) {
	testCases := map[string]string{
		"image": "![alt](https://example.com/image.png)",
		"table": "| a | b |\n| --- | --- |\n| 1 | 2 |",
		"html":  "<div>hi</div>",
		"task":  "- [ ] todo",
	}

	for name, input := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ConvertToFeishuPost([]byte(input))
			if err == nil {
				t.Fatalf("ConvertToFeishuPost() = %s, want unsupported node error", string(got))
			}
			if !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
				t.Fatalf("ConvertToFeishuPost() error = %v, want unsupported node message", err)
			}
		})
	}
}

func TestConvertToFeishuPostRejectsNestedBlockStructures(t *testing.T) {
	testCases := map[string]string{
		"quote-with-list": "> - item\n",
		"list-with-quote": "- item\n  > quote\n",
	}

	for name, input := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := ConvertToFeishuPost([]byte(input))
			if err == nil {
				t.Fatalf("ConvertToFeishuPost() = %s, want unsupported nested block error", string(got))
			}
			if !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
				t.Fatalf("ConvertToFeishuPost() error = %v, want unsupported nested block message", err)
			}
		})
	}
}

func TestConvertToFeishuPostRejectsNestedInlineStyling(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("***text***\n"))
	if err == nil {
		t.Fatalf("ConvertToFeishuPost() = %s, want unsupported nested inline style error", string(got))
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
		t.Fatalf("ConvertToFeishuPost() error = %v, want unsupported nested inline style message", err)
	}
}

func TestConvertToFeishuPostRejectsStyledLinkLabel(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("[**bold**](https://example.com)\n"))
	if err == nil {
		t.Fatalf("ConvertToFeishuPost() = %s, want unsupported styled link label error", string(got))
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unsupported") {
		t.Fatalf("ConvertToFeishuPost() error = %v, want unsupported styled link label message", err)
	}
}

func TestConvertToFeishuPostProducesValidJSON(t *testing.T) {
	got, err := ConvertToFeishuPost([]byte("# Hello\n\nParagraph.\n"))
	if err != nil {
		t.Fatalf("ConvertToFeishuPost() error = %v", err)
	}

	var envelope map[string]any
	if err := json.Unmarshal(got, &envelope); err != nil {
		t.Fatalf("ConvertToFeishuPost() returned invalid JSON: %v", err)
	}
}
