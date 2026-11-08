package eui

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed themes/palettes/*.json
var embeddedThemes embed.FS

// Theme bundles all style information for windows and widgets.
type Theme struct {
	Syntax   SyntaxColors
	Window   windowData
	Button   itemData
	Text     itemData
	Checkbox itemData
	Radio    itemData
	Input    itemData
	Slider   itemData
	Dropdown itemData
	Progress itemData
	Tab      itemData
}

type themeFile struct {
	Comment          string            `json:"Comment"`
	Colors           map[string]string `json:"Colors"`
	RecommendedStyle string            `json:"RecommendedStyle"`
}

// themeAccentRefs records which theme fields referred to the named "accent"
// color in the source JSON so they can be updated dynamically when the accent
// color changes.

// resolveColor recursively resolves string references to colors after the
// theme JSON has been parsed. Color strings may reference other named colors
// from the same file.
func (ctx *Context) resolveColor(s string, colors map[string]string, seen map[string]bool) (Color, error) {
	s = strings.TrimSpace(s)
	key := strings.ToLower(s)
	if c, ok := ctx.namedColors[key]; ok {
		return c, nil
	}
	if val, ok := colors[key]; ok {
		if seen[key] {
			return Color{}, fmt.Errorf("color reference cycle for %s", key)
		}
		seen[key] = true
		c, err := ctx.resolveColor(val, colors, seen)
		if err != nil {
			return Color{}, err
		}
		ctx.namedColors[key] = c
		return c, nil
	}
	var c Color
	if err := c.UnmarshalJSON([]byte(strconv.Quote(s))); err != nil {
		return Color{}, err
	}
	return c, nil
}

// LoadTheme reads a theme JSON file from the themes directory and
// sets it as the current theme without modifying existing windows.
func (ctx *Context) LoadTheme(name string) error {
	ctx.mustAlive()

	// Try the user data directory first.
	file := filepath.Join(ctx.themeDirectory, "palettes", name+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		// Fallback to embedded palettes; embed paths must use forward slashes
		data, err = embeddedThemes.ReadFile(path.Join("themes", "palettes", name+".json"))
		if err != nil {
			return err
		}
	}

	data = themeSourceText(data)
	th, tf, colors, err := ctx.decodeThemeSource(data)
	if err != nil {
		return err
	}
	oldTheme := ctx.currentTheme
	ctx.namedColors = colors

	// Capture which fields referenced the named "accent" color so we can
	// update them when the user tweaks the accent via the color wheel.
	// We only track a small set of fields that are designed to be accent-driven.
	var refs struct {
		Window struct {
			ActiveColor string `json:"ActiveColor"`
		} `json:"Window"`
		Button struct {
			ClickColor string `json:"ClickColor"`
		} `json:"Button"`
		Text struct {
			ClickColor string `json:"ClickColor"`
		} `json:"Text"`
		Checkbox struct {
			ClickColor string `json:"ClickColor"`
		} `json:"Checkbox"`
		Radio struct {
			ClickColor string `json:"ClickColor"`
		} `json:"Radio"`
		Input struct {
			ClickColor string `json:"ClickColor"`
		} `json:"Input"`
		Slider struct {
			ClickColor   string `json:"ClickColor"`
			SliderFilled string `json:"SliderFilled"`
		} `json:"Slider"`
		Dropdown struct {
			ClickColor    string `json:"ClickColor"`
			SelectedColor string `json:"SelectedColor"`
		} `json:"Dropdown"`
		Tab struct {
			ClickColor string `json:"ClickColor"`
		} `json:"Tab"`
		Progress struct {
			SelectedColor string `json:"SelectedColor"`
		} `json:"Progress"`
	}
	// Best-effort; ignore errors since not all fields are present in every palette
	_ = json.Unmarshal(data, &refs)
	// Helper to check if a string equals "accent" (case-insensitive, trimmed)
	isAccent := func(s string) bool { return strings.ToLower(strings.TrimSpace(s)) == "accent" }
	ctx.themeAccentRefs.WindowActive = isAccent(refs.Window.ActiveColor)
	ctx.themeAccentRefs.ButtonClick = isAccent(refs.Button.ClickColor)
	ctx.themeAccentRefs.TextClick = isAccent(refs.Text.ClickColor)
	ctx.themeAccentRefs.CheckboxClick = isAccent(refs.Checkbox.ClickColor)
	ctx.themeAccentRefs.RadioClick = isAccent(refs.Radio.ClickColor)
	ctx.themeAccentRefs.InputClick = isAccent(refs.Input.ClickColor)
	ctx.themeAccentRefs.SliderClick = isAccent(refs.Slider.ClickColor)
	ctx.themeAccentRefs.SliderFilled = isAccent(refs.Slider.SliderFilled)
	ctx.themeAccentRefs.DropdownClick = isAccent(refs.Dropdown.ClickColor)
	ctx.themeAccentRefs.DropdownSelect = isAccent(refs.Dropdown.SelectedColor)
	ctx.themeAccentRefs.TabClick = isAccent(refs.Tab.ClickColor)
	ctx.themeAccentRefs.ProgressSelect = isAccent(refs.Progress.SelectedColor)
	ctx.currentTheme = &th
	ctx.bindTheme()

	ctx.SetCurrentThemeName(name)
	ctx.applyStyleToTheme(ctx.currentTheme)
	ctx.updateThemeReferences(oldTheme, ctx.currentTheme)
	ctx.applyStyleToItems(ctx.currentTheme)
	ctx.markAllDirty()
	if ac, ok := ctx.namedColors["accent"]; ok {
		ctx.accentHue, ctx.accentSaturation, ctx.accentValue, ctx.accentAlpha = rgbaToHSVA(color.RGBA(ac))
	}
	if tf.RecommendedStyle != "" {
		_ = ctx.LoadStyle(tf.RecommendedStyle)
	}
	ctx.refreshThemeMod()
	return nil
}

// updateThemeReferences replaces references to old theme with the new theme across
// all active windows and their item trees.
func (ctx *Context) updateThemeReferences(old, new *Theme) {
	for _, win := range ctx.windows {
		if win.Theme == old {
			if win.ShadowSize == old.Window.ShadowSize {
				win.ShadowSize = new.Window.ShadowSize
			}
			if win.ShadowColor == old.Window.ShadowColor {
				win.ShadowColor = new.Window.ShadowColor
			}
			if win.ShadowFalloff == old.Window.ShadowFalloff {
				win.ShadowFalloff = new.Window.ShadowFalloff
			}
			win.Theme = new
		}
		updateItemThemeTree(win.Contents, old, new)
	}
}

// updateItemThemeTree walks an item tree and updates theme pointers.
func updateItemThemeTree(items []*itemData, old, new *Theme) {
	for _, it := range items {
		if it.Theme == old {
			oldStyle := it.themeStyle()
			if oldStyle != nil {
				// A custom button reuses its resolved style on the next lookup.
				previous := *oldStyle
				oldStyle = &previous
			}
			it.Theme = new
			// Constructors copy fills and outlines. Carry inherited colors across
			// palette changes while preserving explicit per-control overrides.
			if oldStyle != nil {
				if newStyle := it.themeStyle(); newStyle != nil {
					if it.Color == oldStyle.Color {
						it.Color = newStyle.Color
					}
					if it.OutlineColor == oldStyle.OutlineColor {
						it.OutlineColor = newStyle.OutlineColor
					}
					if it.ShadowSize == oldStyle.ShadowSize {
						it.ShadowSize = newStyle.ShadowSize
					}
					if it.ShadowColor == oldStyle.ShadowColor {
						it.ShadowColor = newStyle.ShadowColor
					}
					if it.ShadowFalloff == oldStyle.ShadowFalloff {
						it.ShadowFalloff = newStyle.ShadowFalloff
					}
				}
			}
		}
		if len(it.Contents) > 0 {
			updateItemThemeTree(it.Contents, old, new)
		}
		if len(it.Tabs) > 0 {
			updateItemThemeTree(it.Tabs, old, new)
		}
	}
}

// listThemes returns the available theme names from the themes directory
func (ctx *Context) listThemes() ([]string, error) {
	embeddedEntries, err := fs.ReadDir(embeddedThemes, "themes/palettes")
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(embeddedEntries))
	addEntries := func(entries []fs.DirEntry) {
		for _, e := range entries {
			if e.IsDir() || e.Name() == "Example.json" {
				continue
			}
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			names[name] = struct{}{}
		}
	}
	addEntries(embeddedEntries)
	localEntries, err := os.ReadDir(filepath.Join(ctx.themeDirectory, "palettes"))
	if err == nil {
		addEntries(localEntries)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

// SaveTheme writes the current theme to a JSON file with the given name.
func (ctx *Context) SaveTheme(name string) error {
	ctx.mustAlive()

	if name == "" {
		return fmt.Errorf("theme name required")
	}
	data, err := json.MarshalIndent(ctx.currentTheme, "", "  ")
	if err != nil {
		return err
	}
	paletteDirectory := filepath.Join(ctx.themeDirectory, "palettes")
	if err := os.MkdirAll(paletteDirectory, 0755); err != nil {
		return err
	}
	file := filepath.Join(paletteDirectory, name+".json")
	if err := os.WriteFile(file, data, 0644); err != nil {
		return err
	}
	return nil
}

// SetAccentColor updates the accent color in the current theme and applies it
// to all windows and widgets.
func (ctx *Context) SetAccentColor(c Color) {
	ctx.mustAlive()

	ctx.accentHue, _, ctx.accentValue, ctx.accentAlpha = rgbaToHSVA(color.RGBA(c))
	if ctx.namedColors != nil {
		ctx.namedColors["accent"] = ctx.AccentColor()
	}
	// If the active theme used the named accent for certain fields, update
	// those concrete colors so widgets reflect the new accent immediately.
	if ctx.currentTheme != nil {
		ac := ctx.AccentColor()
		if ctx.themeAccentRefs.WindowActive {
			ctx.currentTheme.Window.ActiveColor = ac
		}
		if ctx.themeAccentRefs.ButtonClick {
			ctx.currentTheme.Button.ClickColor = ac
		}
		if ctx.themeAccentRefs.TextClick {
			ctx.currentTheme.Text.ClickColor = ac
		}
		if ctx.themeAccentRefs.CheckboxClick {
			ctx.currentTheme.Checkbox.ClickColor = ac
		}
		if ctx.themeAccentRefs.RadioClick {
			ctx.currentTheme.Radio.ClickColor = ac
		}
		if ctx.themeAccentRefs.InputClick {
			ctx.currentTheme.Input.ClickColor = ac
		}
		if ctx.themeAccentRefs.SliderClick {
			ctx.currentTheme.Slider.ClickColor = ac
		}
		if ctx.themeAccentRefs.SliderFilled {
			ctx.currentTheme.Slider.SelectedColor = ac
		}
		if ctx.themeAccentRefs.DropdownClick {
			ctx.currentTheme.Dropdown.ClickColor = ac
		}
		if ctx.themeAccentRefs.DropdownSelect {
			ctx.currentTheme.Dropdown.SelectedColor = ac
		}
		if ctx.themeAccentRefs.TabClick {
			ctx.currentTheme.Tab.ClickColor = ac
		}
		if ctx.themeAccentRefs.ProgressSelect {
			ctx.currentTheme.Progress.SelectedColor = ac
		}
	}
	ctx.markAllDirty()
}

// SetAccentSaturation updates the saturation component of the accent color and
// reapplies it to the current theme.
func (ctx *Context) SetAccentSaturation(s float64) {
	ctx.mustAlive()

	ctx.accentSaturation = clamp(s, 0, 1)
	if ctx.namedColors != nil {
		ctx.namedColors["accent"] = ctx.AccentColor()
	}
	// Re-apply to theme fields which referenced accent
	if ctx.currentTheme != nil {
		ac := ctx.AccentColor()
		if ctx.themeAccentRefs.WindowActive {
			ctx.currentTheme.Window.ActiveColor = ac
		}
		if ctx.themeAccentRefs.ButtonClick {
			ctx.currentTheme.Button.ClickColor = ac
		}
		if ctx.themeAccentRefs.TextClick {
			ctx.currentTheme.Text.ClickColor = ac
		}
		if ctx.themeAccentRefs.CheckboxClick {
			ctx.currentTheme.Checkbox.ClickColor = ac
		}
		if ctx.themeAccentRefs.RadioClick {
			ctx.currentTheme.Radio.ClickColor = ac
		}
		if ctx.themeAccentRefs.InputClick {
			ctx.currentTheme.Input.ClickColor = ac
		}
		if ctx.themeAccentRefs.SliderClick {
			ctx.currentTheme.Slider.ClickColor = ac
		}
		if ctx.themeAccentRefs.SliderFilled {
			ctx.currentTheme.Slider.SelectedColor = ac
		}
		if ctx.themeAccentRefs.DropdownClick {
			ctx.currentTheme.Dropdown.ClickColor = ac
		}
		if ctx.themeAccentRefs.DropdownSelect {
			ctx.currentTheme.Dropdown.SelectedColor = ac
		}
		if ctx.themeAccentRefs.TabClick {
			ctx.currentTheme.Tab.ClickColor = ac
		}
		if ctx.themeAccentRefs.ProgressSelect {
			ctx.currentTheme.Progress.SelectedColor = ac
		}
	}
	ctx.markAllDirty()
}

func (ctx *Context) decodeThemeSource(data []byte) (Theme, themeFile, map[string]Color, error) {
	data = themeSourceText(data)
	if err := validateThemeObject(data); err != nil {
		return Theme{}, themeFile{}, nil, err
	}
	previous := ctx.namedColors
	defer func() { ctx.namedColors = previous }()
	// Reset named colors
	ctx.namedColors = map[string]Color{}

	var tf themeFile
	if err := json.Unmarshal(data, &tf); err != nil {
		return Theme{}, themeFile{}, nil, err
	}
	for n, v := range tf.Colors {
		c, err := ctx.resolveColor(v, tf.Colors, map[string]bool{strings.ToLower(n): true})
		if err != nil {
			return Theme{}, themeFile{}, nil, fmt.Errorf("%s: %w", n, err)
		}
		ctx.namedColors[strings.ToLower(n)] = c
	}

	// Expand named colors using this decode's own table. Color.UnmarshalJSON
	// remains independent of any live UI instance.
	expanded, err := expandThemeColorJSON(data, ctx.namedColors)
	if err != nil {
		return Theme{}, themeFile{}, nil, err
	}
	data = expanded
	// Start with the compiled in defaults
	th := *baseTheme
	if err := json.Unmarshal(data, &th); err != nil {
		return Theme{}, themeFile{}, nil, err
	}
	// Resolve omitted syntax colors against the loaded input background, and
	// extract the additional slider fill field.
	var extra struct {
		Syntax SyntaxColors
		Slider struct {
			SliderFilled string `json:"SliderFilled"`
		} `json:"Slider"`
	}
	extra.Syntax = DefaultSyntaxColors(th.Input.Color)
	if err := json.Unmarshal(data, &extra); err != nil {
		return Theme{}, themeFile{}, nil, err
	}
	th.Syntax = extra.Syntax

	if extra.Slider.SliderFilled != "" {
		col, err := ctx.resolveColor(extra.Slider.SliderFilled, tf.Colors, map[string]bool{"sliderfilled": true})
		if err != nil {
			return Theme{}, themeFile{}, nil, fmt.Errorf("SliderFilled: %w", err)
		}
		ctx.namedColors["sliderfilled"] = col
		th.Slider.SelectedColor = col
	}
	return th, tf, ctx.namedColors, nil
}
