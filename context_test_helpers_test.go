package eui

import "testing"

func newTestContext(t testing.TB) *Context {
	t.Helper()
	ctx, err := NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(ctx.Dispose)
	return ctx
}
