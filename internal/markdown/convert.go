package markdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

var markdownParser = goldmark.New(goldmark.WithExtensions(extension.GFM))

type postEnvelope struct {
	ZhCN postLocale `json:"zh_cn"`
}

type postLocale struct {
	Title   string       `json:"title"`
	Content [][]postNode `json:"content"`
}

type postNode struct {
	Tag      string `json:"tag"`
	Language string `json:"language,omitempty"`
	Text     string `json:"text,omitempty"`
	Style    string `json:"style,omitempty"`
	Href     string `json:"href,omitempty"`
}

// ConvertToFeishuPost converts Markdown into a Feishu post JSON envelope.
func ConvertToFeishuPost(markdown []byte) ([]byte, error) {
	doc := markdownParser.Parser().Parse(text.NewReader(markdown))

	conv := converter{
		source:  markdown,
		content: make([][]postNode, 0),
	}

	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if heading, ok := node.(*gast.Heading); ok && heading.Level == 1 {
			if conv.titleSeen {
				return nil, unsupportedNodeError(node)
			}

			title, err := conv.renderPlainText(heading.FirstChild())
			if err != nil {
				return nil, err
			}

			conv.title = title
			conv.titleSeen = true
			continue
		}

		rows, err := conv.convertBlock(node)
		if err != nil {
			return nil, err
		}
		conv.content = append(conv.content, rows...)
	}

	if conv.content == nil {
		conv.content = make([][]postNode, 0)
	}

	return marshalPostEnvelope(postEnvelope{
		ZhCN: postLocale{
			Title:   conv.title,
			Content: conv.content,
		},
	})
}

type converter struct {
	source    []byte
	titleSeen bool
	title     string
	content   [][]postNode
}

func (c *converter) convertBlock(node gast.Node) ([][]postNode, error) {
	switch n := node.(type) {
	case *gast.Paragraph:
		items, err := c.convertInlines(n.FirstChild())
		if err != nil {
			return nil, err
		}
		if len(items) == 0 {
			return nil, nil
		}
		return [][]postNode{items}, nil
	case *gast.FencedCodeBlock:
		return [][]postNode{{{
			Tag:      "code_block",
			Language: strings.TrimSpace(string(n.Language(c.source))),
			Text:     trimTrailingNewlines(string(n.Text(c.source))),
		}}}, nil
	case *gast.CodeBlock:
		return [][]postNode{{{
			Tag:  "code_block",
			Text: trimTrailingNewlines(string(n.Text(c.source))),
		}}}, nil
	case *gast.Blockquote:
		block, err := c.renderBlockMarkdown(n)
		if err != nil {
			return nil, err
		}
		if block == "" {
			return nil, nil
		}
		return [][]postNode{{{Tag: "md", Text: block}}}, nil
	case *gast.List:
		block, err := c.renderBlockMarkdown(n)
		if err != nil {
			return nil, err
		}
		if block == "" {
			return nil, nil
		}
		return [][]postNode{{{Tag: "md", Text: block}}}, nil
	case *gast.HTMLBlock, *gast.ThematicBreak, *gast.Image, *gast.RawHTML:
		return nil, unsupportedNodeError(node)
	case *extast.Table, *extast.TableRow, *extast.TableHeader, *extast.TableCell:
		return nil, unsupportedNodeError(node)
	case *extast.TaskCheckBox:
		return nil, unsupportedNodeError(node)
	case *gast.Heading:
		return nil, unsupportedNodeError(node)
	default:
		return nil, unsupportedNodeError(node)
	}
}

func (c *converter) convertInlines(node gast.Node) ([]postNode, error) {
	var out []postNode
	for cur := node; cur != nil; cur = cur.NextSibling() {
		items, err := c.convertInline(cur, "")
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			out = appendMergedPostNode(out, item)
		}
	}
	return out, nil
}

func (c *converter) convertInline(node gast.Node, style string) ([]postNode, error) {
	switch n := node.(type) {
	case *gast.Text:
		text := string(n.Value(c.source))
		if n.HardLineBreak() {
			text += "\n"
		} else if n.SoftLineBreak() {
			text += " "
		}
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []postNode{{Tag: "text", Text: text, Style: style}}, nil
	case *gast.String:
		text := string(n.Value)
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []postNode{{Tag: "text", Text: text, Style: style}}, nil
	case *gast.Emphasis:
		nextStyle := "italic"
		if n.Level >= 2 {
			nextStyle = "bold"
		}
		return c.convertInlineChildren(n.FirstChild(), nextStyle)
	case *extast.Strikethrough:
		return c.convertInlineChildren(n.FirstChild(), "lineThrough")
	case *gast.Link:
		label, err := c.renderPlainText(n.FirstChild())
		if err != nil {
			return nil, err
		}
		return []postNode{{Tag: "a", Text: label, Href: string(n.Destination)}}, nil
	case *gast.AutoLink:
		url := string(n.URL(c.source))
		return []postNode{{Tag: "a", Text: url, Href: url}}, nil
	case *gast.CodeSpan:
		label, err := c.renderPlainText(n.FirstChild())
		if err != nil {
			return nil, err
		}
		if label == "" {
			return nil, nil
		}
		return []postNode{{Tag: "text", Text: "`" + label + "`", Style: style}}, nil
	case *gast.RawHTML, *gast.Image:
		return nil, unsupportedNodeError(node)
	case *extast.TaskCheckBox:
		return nil, unsupportedNodeError(node)
	default:
		if node.HasChildren() {
			return c.convertInlineChildren(node.FirstChild(), style)
		}
		return nil, unsupportedNodeError(node)
	}
}

func (c *converter) convertInlineChildren(node gast.Node, style string) ([]postNode, error) {
	var out []postNode
	for cur := node; cur != nil; cur = cur.NextSibling() {
		items, err := c.convertInline(cur, style)
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

func (c *converter) renderPlainText(node gast.Node) (string, error) {
	var b strings.Builder
	for cur := node; cur != nil; cur = cur.NextSibling() {
		part, err := c.renderPlainTextNode(cur)
		if err != nil {
			return "", err
		}
		b.WriteString(part)
	}
	return b.String(), nil
}

func (c *converter) renderPlainTextNode(node gast.Node) (string, error) {
	switch n := node.(type) {
	case *gast.Text:
		text := string(n.Value(c.source))
		if n.HardLineBreak() || n.SoftLineBreak() {
			text += " "
		}
		return text, nil
	case *gast.String:
		return string(n.Value), nil
	case *gast.Emphasis, *extast.Strikethrough:
		return c.renderPlainText(node.FirstChild())
	case *gast.Link:
		return c.renderPlainText(node.FirstChild())
	case *gast.AutoLink:
		return string(n.URL(c.source)), nil
	case *gast.CodeSpan:
		return c.renderPlainText(node.FirstChild())
	case *gast.RawHTML, *gast.Image, *gast.HTMLBlock, *gast.ThematicBreak:
		return "", unsupportedNodeError(node)
	case *extast.Table, *extast.TableRow, *extast.TableHeader, *extast.TableCell:
		return "", unsupportedNodeError(node)
	case *extast.TaskCheckBox:
		return "", unsupportedNodeError(node)
	default:
		if node.HasChildren() {
			return c.renderPlainText(node.FirstChild())
		}
		return "", unsupportedNodeError(node)
	}
}

func (c *converter) renderBlockMarkdown(node gast.Node) (string, error) {
	switch n := node.(type) {
	case *gast.Paragraph:
		return c.renderInlineMarkdown(n.FirstChild())
	case *gast.Text:
		return c.renderInlineMarkdownNode(n)
	case *gast.String:
		return c.renderInlineMarkdownNode(n)
	case *gast.Blockquote:
		var parts []string
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			part, err := c.renderBlockMarkdown(child)
			if err != nil {
				return "", err
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) == 0 {
			return "", nil
		}
		return prefixEachLine(strings.Join(parts, "\n\n"), "> "), nil
	case *gast.List:
		return c.renderListMarkdown(n)
	case *gast.ListItem:
		var parts []string
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			part, err := c.renderBlockMarkdown(child)
			if err != nil {
				return "", err
			}
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "\n\n"), nil
	case *gast.TextBlock:
		return c.renderInlineMarkdown(n.FirstChild())
	case *gast.FencedCodeBlock:
		lang := strings.TrimSpace(string(n.Language(c.source)))
		code := trimTrailingNewlines(string(n.Text(c.source)))
		if lang != "" {
			return "```" + lang + "\n" + code + "\n```", nil
		}
		return "```\n" + code + "\n```", nil
	case *gast.CodeBlock:
		code := trimTrailingNewlines(string(n.Text(c.source)))
		return "```\n" + code + "\n```", nil
	case *gast.Image, *gast.RawHTML, *gast.HTMLBlock, *gast.ThematicBreak, *gast.Heading:
		return "", unsupportedNodeError(node)
	case *extast.Table, *extast.TableRow, *extast.TableHeader, *extast.TableCell:
		return "", unsupportedNodeError(node)
	case *extast.TaskCheckBox:
		return "", unsupportedNodeError(node)
	default:
		return "", unsupportedNodeError(node)
	}
}

func (c *converter) renderListMarkdown(list *gast.List) (string, error) {
	var out []string
	index := list.Start
	if index == 0 {
		index = 1
	}

	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		listItem, ok := item.(*gast.ListItem)
		if !ok {
			return "", fmt.Errorf("unexpected list item type %T", item)
		}

		body, err := c.renderListItemMarkdown(listItem)
		if err != nil {
			return "", err
		}

		prefix := "- "
		if list.IsOrdered() {
			prefix = fmt.Sprintf("%d. ", index)
			index++
		}

		out = append(out, indentBlock(body, prefix))
	}

	return strings.Join(out, "\n"), nil
}

func (c *converter) renderListItemMarkdown(item *gast.ListItem) (string, error) {
	var parts []string
	for child := item.FirstChild(); child != nil; child = child.NextSibling() {
		part, err := c.renderBlockMarkdown(child)
		if err != nil {
			return "", err
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func (c *converter) renderInlineMarkdown(node gast.Node) (string, error) {
	var b strings.Builder
	for cur := node; cur != nil; cur = cur.NextSibling() {
		part, err := c.renderInlineMarkdownNode(cur)
		if err != nil {
			return "", err
		}
		b.WriteString(part)
	}
	return b.String(), nil
}

func (c *converter) renderInlineMarkdownNode(node gast.Node) (string, error) {
	switch n := node.(type) {
	case *gast.Text:
		text := string(n.Value(c.source))
		if n.HardLineBreak() {
			text += "  \n"
		} else if n.SoftLineBreak() {
			text += "\n"
		}
		return text, nil
	case *gast.String:
		return string(n.Value), nil
	case *gast.Emphasis:
		inner, err := c.renderInlineMarkdown(n.FirstChild())
		if err != nil {
			return "", err
		}
		if n.Level >= 2 {
			return "**" + inner + "**", nil
		}
		return "*" + inner + "*", nil
	case *extast.Strikethrough:
		inner, err := c.renderInlineMarkdown(n.FirstChild())
		if err != nil {
			return "", err
		}
		return "~~" + inner + "~~", nil
	case *gast.Link:
		inner, err := c.renderPlainText(n.FirstChild())
		if err != nil {
			return "", err
		}
		return "[" + inner + "](" + string(n.Destination) + ")", nil
	case *gast.AutoLink:
		return string(n.URL(c.source)), nil
	case *gast.CodeSpan:
		inner, err := c.renderPlainText(n.FirstChild())
		if err != nil {
			return "", err
		}
		return "`" + strings.ReplaceAll(inner, "`", "\\`") + "`", nil
	case *gast.RawHTML, *gast.Image:
		return "", unsupportedNodeError(node)
	case *extast.TaskCheckBox:
		return "", unsupportedNodeError(node)
	default:
		if node.HasChildren() {
			return c.renderInlineMarkdown(node.FirstChild())
		}
		return "", unsupportedNodeError(node)
	}
}

func unsupportedNodeError(node gast.Node) error {
	return fmt.Errorf("unsupported markdown node: %s", node.Kind())
}

func marshalPostEnvelope(envelope postEnvelope) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(envelope); err != nil {
		return nil, fmt.Errorf("marshal feishu post: %w", err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
func appendMergedPostNode(out []postNode, item postNode) []postNode {
	if item.Tag == "text" && strings.TrimSpace(item.Text) == "" {
		return out
	}

	if len(out) == 0 {
		return append(out, item)
	}

	last := &out[len(out)-1]
	if last.Tag == "text" && item.Tag == "text" && last.Style == item.Style && last.Href == "" && item.Href == "" && last.Language == "" && item.Language == "" {
		last.Text += item.Text
		return out
	}

	return append(out, item)
}

func trimTrailingNewlines(text string) string {
	return strings.TrimRight(text, "\r\n")
}

func prefixEachLine(text, prefix string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

func indentBlock(text, prefix string) string {
	if text == "" {
		return prefix
	}

	lines := strings.Split(text, "\n")
	indent := strings.Repeat(" ", len(prefix))
	for i, line := range lines {
		if i == 0 {
			lines[i] = prefix + line
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}
