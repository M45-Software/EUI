package eui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSetUserDataRootMovesEditableThemes(t *testing.T) {
	ctx := newTestContext(t)

	originalDirectory := ctx.themeDirectory
	t.Cleanup(func() {
		ctx.themeDirectory = originalDirectory
		ctx.updateThemePath()
		ctx.updateStylePath()
		ctx.refreshThemeMod()
		ctx.refreshStyleMod()
	})

	root := t.TempDir()
	ctx.SetUserDataRoot(root)
	wantDirectory := filepath.Join(root, "themes")
	if ctx.themeDirectory != wantDirectory {
		t.Fatalf("themeDirectory = %q, want %q", ctx.themeDirectory, wantDirectory)
	}
	for _, path := range []string{
		filepath.Join(wantDirectory, "README.md"),
		filepath.Join(wantDirectory, "palettes", "Example.json"),
		filepath.Join(wantDirectory, "styles", "Example.json"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("generated theme file %q: %v", path, err)
		}
	}
	if want := filepath.Join(wantDirectory, "palettes", ctx.currentThemeName+".json"); ctx.themePath != want {
		t.Fatalf("themePath = %q, want %q", ctx.themePath, want)
	}
	if want := filepath.Join(wantDirectory, "styles", ctx.currentStyleName+".json"); ctx.stylePath != want {
		t.Fatalf("stylePath = %q, want %q", ctx.stylePath, want)
	}

	customPalette := filepath.Join(wantDirectory, "palettes", "MyPalette.json")
	if err := os.WriteFile(customPalette, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write custom palette: %v", err)
	}
	customStyle := filepath.Join(wantDirectory, "styles", "MyStyle.json")
	if err := os.WriteFile(customStyle, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write custom style: %v", err)
	}
	assertContainsThemeName(t, ctx.ListThemes, "MyPalette")
	assertContainsThemeName(t, ctx.ListThemes, "AccentDark")
	assertContainsThemeName(t, ctx.ListStyles, "MyStyle")
	assertContainsThemeName(t, ctx.ListStyles, "Default")
}

func assertContainsThemeName(t *testing.T, list func() ([]string, error), want string) {
	t.Helper()
	names, err := list()
	if err != nil {
		t.Fatalf("list themes: %v", err)
	}
	for _, name := range names {
		if name == want {
			return
		}
	}
	t.Fatalf("theme names %v do not contain %q", names, want)
}
