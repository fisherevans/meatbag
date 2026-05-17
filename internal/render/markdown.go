// Package render produces sanitized HTML from markdown for the web UI to embed.
// Tables, fenced code blocks, autolinks, and strikethrough are enabled; raw
// HTML in markdown is stripped during sanitization.
package render

import (
	"bytes"
	"sync"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

var (
	once sync.Once
	md   goldmark.Markdown
	pol  *bluemonday.Policy
)

func initOnce() {
	once.Do(func() {
		md = goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(
				html.WithHardWraps(),
				html.WithUnsafe(), // keep before sanitize; sanitizer scrubs
			),
		)
		pol = bluemonday.UGCPolicy()
		pol.AllowAttrs("class").OnElements("code", "pre", "span")
		// Inline SVG / PNG data URIs for screenshots and diagrams the agent
		// generates without needing to upload to a host.
		pol.AllowDataURIImages()
		// GFM task list markers render as <input type=checkbox disabled>.
		pol.AllowAttrs("type", "checked", "disabled").OnElements("input")
	})
}

// Markdown returns sanitized HTML for the given markdown source.
func Markdown(src string) string {
	initOnce()
	if src == "" {
		return ""
	}
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		return ""
	}
	return pol.Sanitize(buf.String())
}
