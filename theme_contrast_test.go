package eui

import (
	"encoding/json"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func (ctx *Context) isolateThemeTest(t *testing.T) {
	t.Helper()
	oldTheme, oldStyle := ctx.currentTheme, ctx.currentStyle
	oldColors, oldRefs := ctx.namedColors, ctx.themeAccentRefs
	oldWindows, oldDirectory := ctx.windows, ctx.themeDirectory
	oldThemeName, oldStyleName := ctx.currentThemeName, ctx.currentStyleName
	h, s, v, a := ctx.accentHue, ctx.accentSaturation, ctx.accentValue, ctx.accentAlpha
	ctx.windows = nil
	if ctx.currentTheme != nil {
		copy := *ctx.currentTheme
		ctx.currentTheme = &copy
	}
	ctx.themeDirectory = t.TempDir()
	t.Cleanup(func() {
		ctx.currentTheme, ctx.currentStyle = oldTheme, oldStyle
		ctx.namedColors, ctx.themeAccentRefs = oldColors, oldRefs
		ctx.windows, ctx.themeDirectory = oldWindows, oldDirectory
		ctx.accentHue, ctx.accentSaturation, ctx.accentValue, ctx.accentAlpha = h, s, v, a
		ctx.SetCurrentThemeName(oldThemeName)
		ctx.SetCurrentStyleName(oldStyleName)
		ctx.refreshThemeMod()
		ctx.refreshStyleMod()
	})
}

func TestReadableTextAcrossAccentColors(t *testing.T) {
	for r := 0; r <= 255; r += 17 {
		for g := 0; g <= 255; g += 17 {
			for b := 0; b <= 255; b += 17 {
				bg := NewColor(uint8(r), uint8(g), uint8(b), 255)
				for _, preferred := range []Color{bg, ColorWhite, ColorBlack, NewColor(140, 150, 160, 255)} {
					got := readableTextColor(preferred, bg)
					if ratio := textContrast(got, bg); ratio < 4.5 {
						t.Fatalf("foreground %v on %v has contrast %.2f", got, bg, ratio)
					}
					if textContrast(preferred, bg) >= 4.5 && got != preferred {
						t.Fatalf("readable palette color %v was replaced by %v", preferred, got)
					}
				}
			}
		}
	}
}

func TestBundledPalettesHaveReadableSurfaces(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	entries, err := embeddedThemes.ReadDir("themes/palettes")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()[:len(entry.Name())-len(".json")]
		if name == "Example" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			if err := ctx.LoadTheme(name); err != nil {
				t.Fatal(err)
			}
			// Every recommendation must name an actual style, even though LoadTheme
			// tolerates an unavailable recommendation in user-authored palettes.
			data, err := embeddedThemes.ReadFile("themes/palettes/" + entry.Name())
			if err != nil {
				t.Fatal(err)
			}
			var metadata themeFile
			if err := json.Unmarshal(data, &metadata); err != nil {
				t.Fatal(err)
			}
			if _, err := embeddedStyles.ReadFile("themes/styles/" + metadata.RecommendedStyle + ".json"); err != nil {
				t.Fatal(err)
			}
			for _, kind := range []struct {
				name  string
				style *itemData
			}{
				{"Button", &ctx.currentTheme.Button}, {"Input", &ctx.currentTheme.Input}, {"Dropdown", &ctx.currentTheme.Dropdown}, {"Tab", &ctx.currentTheme.Tab},
			} {
				for _, background := range []Color{kind.style.Color, kind.style.HoverColor, ctx.currentTheme.Window.BGColor} {
					if ratio := textContrast(kind.style.TextColor, background); ratio < 4.5 {
						t.Errorf("%s normal/hover/window text contrast %.2f", kind.name, ratio)
					}
				}
				disabled := disabledStyle(kind.style)
				if ratio := textContrast(disabled.TextColor, disabled.Color); ratio < 3 {
					t.Errorf("%s disabled caption contrast %.2f", kind.name, ratio)
				}
			}
			for _, accent := range []Color{ColorBlack, ColorWhite, ColorYellow, ColorBlue} {
				_, saturation, _, _ := rgbaToHSVA(color.RGBA(accent))
				ctx.SetAccentSaturation(saturation)
				ctx.SetAccentColor(accent)
				if ratio := textContrast(readableTextColor(ctx.currentTheme.Button.TextColor, ctx.currentTheme.Button.ClickColor), ctx.currentTheme.Button.ClickColor); ratio < 4.5 {
					t.Errorf("custom accent contrast %.2f", ratio)
				}
			}
		})
	}
}

func TestPartialStyleDoesNotInheritPreviousStyle(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	dir := filepath.Join(ctx.themeDirectory, "styles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Partial.json"), []byte(`{"Fillet":{"Button":5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"HighContrast", "Rounded"} {
		if err := ctx.LoadStyle(name); err != nil {
			t.Fatal(err)
		}
		if err := ctx.LoadStyle("Partial"); err != nil {
			t.Fatal(err)
		}
		want := baseStyle
		want.Fillet.Button = 5
		if *ctx.currentStyle != want {
			t.Fatalf("partial style inherited values from %s", name)
		}
	}
	before := *ctx.currentStyle
	if err := os.WriteFile(filepath.Join(dir, "Broken.json"), []byte(`{"TextPadding":99,"Fillet":{"Button":"invalid"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ctx.LoadStyle("Broken"); err == nil {
		t.Fatal("invalid style was accepted")
	}
	if *ctx.currentStyle != before || ctx.CurrentStyleName() != "Partial" {
		t.Fatal("failed style load changed the active style")
	}
}

func TestInputFillFollowsPaletteWithoutReplacingOverrides(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadTheme("AccentDark"); err != nil {
		t.Fatal(err)
	}
	old := ctx.currentTheme
	inherited, _ := ctx.NewInput()
	custom, _ := ctx.NewInput()
	custom.Color = NewColor(70, 20, 60, 255)
	custom.OutlineColor = NewColor(90, 100, 80, 255)
	if err := ctx.LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	updateItemThemeTree([]*itemData{inherited, custom}, old, ctx.currentTheme)
	if inherited.Color != ctx.currentTheme.Input.Color {
		t.Fatal("input retained the previous palette's fill")
	}
	if inherited.OutlineColor != ctx.currentTheme.Input.OutlineColor || custom.OutlineColor != NewColor(90, 100, 80, 255) {
		t.Fatal("palette switch failed to update inherited outlines or preserve custom outlines")
	}
	if custom.Color != NewColor(70, 20, 60, 255) {
		t.Fatal("explicit input fill was replaced")
	}
}

func TestUnfilledControlUsesWindowContrast(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	item, _ := ctx.NewButton()
	item.Filled = false
	if got := item.surfaceTextColor(ctx.currentTheme.Button.TextColor, ColorBlack, false); got != ctx.currentTheme.Button.TextColor {
		t.Fatal("unfilled button used the unpainted highlight color")
	}
}

func TestCustomThemesAndTemplateFiltering(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadTheme("NeonNight"); err != nil || ctx.CurrentThemeName() != "NeonNight" {
		t.Fatalf("NeonNight lost its identity: %v", err)
	}
	for _, dir := range []string{"palettes", "styles"} {
		if err := os.MkdirAll(filepath.Join(ctx.themeDirectory, dir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ctx.themeDirectory, dir, "Example.json"), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, list := range []func() ([]string, error){ctx.ListThemes, ctx.ListStyles} {
		names, err := list()
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if name == "Example" {
				t.Errorf("template name %s appeared in selector", name)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(ctx.themeDirectory, "palettes", "CustomPalette.json"), []byte(`{"Window":{"BGColor":"#123456ff"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ctx.LoadTheme("CustomPalette"); err != nil || ctx.CurrentThemeName() != "CustomPalette" || ctx.currentTheme.Window.BGColor != NewColor(18, 52, 86, 255) {
		t.Fatalf("custom palette did not load: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ctx.themeDirectory, "styles", "CustomStyle.json"), []byte(`{"TextPadding":19}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ctx.LoadStyle("CustomStyle"); err != nil || ctx.CurrentStyleName() != "CustomStyle" || ctx.currentStyle.TextPadding != 19 {
		t.Fatalf("custom style did not load: %v", err)
	}
}

func TestUnknownStyleFallsBackToDefault(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadStyle("MissingStyle"); err != nil {
		t.Fatal(err)
	}
	if got := ctx.CurrentStyleName(); got != "Default" {
		t.Fatalf("unknown style resolved to %q, want Default", got)
	}
}

func TestNonTextWidgetsRenderWithoutCaptionStyle(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadTheme("AccentLight"); err != nil {
		t.Fatal(err)
	}
	wheel, _ := ctx.NewColorWheel()
	for _, item := range []*itemData{wheel, {ctx: ctx, ItemType: ITEM_IMAGE}} {
		if item.themeStyle() != nil {
			t.Fatal("test requires a widget without a caption style")
		}
		canvas := ebiten.NewImage(48, 48)
		item.Size = point{48, 48}
		item.drawItemInternal(point{}, point{}, point{48, 48}, rect{0, 0, 48, 48}, canvas)
		canvas.Deallocate()
	}
}

func TestExistingControlsFollowEveryPaletteAfterNeonNight(t *testing.T) {
	ctx := newTestContext(t)

	ctx.isolateThemeTest(t)
	if err := ctx.LoadTheme("NeonNight"); err != nil {
		t.Fatal(err)
	}
	win := ctx.NewWindow()
	constructors := []func() (*itemData, *EventHandler){ctx.NewButton, ctx.NewInput, ctx.NewText, ctx.NewDropdown, ctx.NewSlider, ctx.NewCheckbox, ctx.NewRadio, ctx.NewProgressBar}
	for _, makeControl := range constructors {
		control, _ := makeControl()
		win.AddItem(control)
	}
	ctx.windows = []*windowData{win}
	names, err := ctx.ListThemes()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append(names, "NeonNight", "AccentDark") {
		if err := ctx.LoadTheme(name); err != nil {
			t.Fatal(err)
		}
		for i, makeControl := range constructors {
			fresh, _ := makeControl()
			existing := win.Contents[i]
			if existing.Theme != ctx.currentTheme || existing.OutlineColor != fresh.OutlineColor || existing.Color != fresh.Color {
				t.Fatalf("%s control %v retained colors from a previous palette", name, existing.ItemType)
			}
		}
		if ctx.currentTheme.Progress.SelectedColor != ctx.namedColors["accent"] {
			t.Fatalf("%s progress retained a fixed accent", name)
		}
		ctx.SetAccentSaturation(0.3)
		ctx.SetAccentColor(NewColor(150, 100, 180, 255))
		if ctx.currentTheme.Progress.SelectedColor != ctx.AccentColor() {
			t.Fatal("progress ignored custom accent")
		}
	}
}
