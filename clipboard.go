package eui

import (
	"context"
	clipboard "golang.design/x/clipboard"
)

// Clipboard provides text transfer for copy, cut, and paste. Configure it on the
// UI goroutine; methods run synchronously there. Nil selects the system clipboard.
type Clipboard interface {
	ReadText() (string, error)
	WriteText(string) error
}

// SetClipboard installs a clipboard and returns the previous one. Passing nil
// restores the system clipboard. A memory implementation can isolate tests or
// connect an embedding host's clipboard without touching the operating system.
func (ctx *Context) SetClipboard(value Clipboard) Clipboard {
	ctx.mustAlive()

	previous := ctx.configuredClipboard
	ctx.configuredClipboard = value
	return previous
}
func (ctx *Context) readClipboard() (string, error) {
	if ctx.configuredClipboard != nil {
		return ctx.configuredClipboard.ReadText()
	}
	data, err := clipboard.Read(context.Background(), clipboard.FmtText)
	return string(data), err
}
func (ctx *Context) writeClipboard(value string) error {
	if ctx.configuredClipboard != nil {
		return ctx.configuredClipboard.WriteText(value)
	}
	_, err := clipboard.Write(context.Background(), clipboard.FmtText, []byte(value))
	return err
}
