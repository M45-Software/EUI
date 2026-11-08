package eui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntaxColorsInBundledPalettes(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	entries, err := embeddedThemes.ReadDir("themes/palettes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := embeddedThemes.ReadFile("themes/palettes/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			var raw struct{ Syntax map[string]string }
			if err := json.Unmarshal(data, &raw); err != nil || len(raw.Syntax) != 6 {
				t.Fatalf("palette must define six syntax colors: %v", err)
			}
			if err := ctx.LoadTheme(strings.TrimSuffix(entry.Name(), ".json")); err != nil {
				t.Fatal(err)
			}
			colors := ctx.CurrentSyntaxColors()
			for _, col := range []Color{colors.Comments, colors.Strings, colors.Keywords, colors.Numbers, colors.Variables, colors.Bindings} {
				if ratio := textContrast(col, ctx.currentTheme.Input.Color); col.A != 255 || ratio < 4.5 {
					t.Errorf("syntax color %v on %v has contrast %.2f", col, ctx.currentTheme.Input.Color, ratio)
				}
			}
		})
	}
}

func TestSyntaxColorsFallbackReferencesAndSave(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	dir := filepath.Join(ctx.themeDirectory, "palettes")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, light := range []bool{false, true} {
		background := "#141618ff"
		if light {
			background = "#f5f5f5ff"
		}
		for _, partial := range []bool{false, true} {
			data := `{"Input":{"Color":"` + background + `"}}`
			if partial {
				data = `{"Colors":{"syntaxcomment":"#12345678"},"Input":{"Color":"` + background + `"},"Syntax":{"Comments":"syntaxcomment"}}`
			}
			if err := os.WriteFile(filepath.Join(dir, "SyntaxTest.json"), []byte(data), 0644); err != nil {
				t.Fatal(err)
			}
			if err := ctx.LoadTheme("SyntaxTest"); err != nil {
				t.Fatal(err)
			}
			want := DefaultSyntaxColors(ctx.currentTheme.Input.Color)
			if partial {
				want.Comments = NewColor(0x12, 0x34, 0x56, 0x78)
			}
			if ctx.CurrentSyntaxColors() != want {
				t.Fatalf("missing syntax fields did not use background defaults: %+v", ctx.CurrentSyntaxColors())
			}
			saved, err := json.Marshal(struct{ Syntax SyntaxColors }{want})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "SyntaxSaved.json"), saved, 0644); err != nil {
				t.Fatal(err)
			}
			if err := ctx.LoadTheme("SyntaxSaved"); err != nil || ctx.CurrentSyntaxColors() != want {
				t.Fatalf("saved syntax palette did not round trip: %v", err)
			}
		}
	}
}
