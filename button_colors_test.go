package eui

import "testing"

func TestButtonColorsOverrideThemeAndKeepDisabledAppearance(t *testing.T) {
	ctx := newTestContext(t)

	button, _ := ctx.NewButton()
	ordinary, _ := ctx.NewButton()
	base := *ordinary.themeStyle()
	button.SetButtonColors(ColorDarkRed, ColorRed)
	style := button.themeStyle()
	if style.Color != ColorDarkRed || style.HoverColor != ColorRed || style.ClickColor != ColorRed {
		t.Fatal("action colors did not reach the rendered button style")
	}
	if ordinary.themeStyle().Color != base.Color || style.TextColor != base.TextColor {
		t.Fatal("action colors changed the shared theme or its caption colors")
	}
	if disabledStyle(style).Color != base.DisabledColor {
		t.Fatal("custom button lost its disabled appearance")
	}
}

func TestPopupActionColorsReachButtonStyle(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	popup := ctx.ShowPopup("Confirm", "", []PopupButton{{Text: "Cancel"}, {Text: "Play", Color: &ColorDarkRed, HoverColor: &ColorRed}})
	t.Cleanup(popup.Close)
	button := popup.Contents[0].Contents[0].Contents[1]
	if style := button.themeStyle(); style.Color != ColorDarkRed || style.HoverColor != ColorRed {
		t.Fatal("popup ignored its explicit action colors")
	}
}

func TestButtonColorStyleTracksThemeEditsWithoutChangingSiblings(t *testing.T) {
	ctx := newTestContext(t)

	theme := *baseTheme
	first, _ := ctx.NewButton()
	second, _ := ctx.NewButton()
	first.Theme, second.Theme = &theme, &theme
	first.SetButtonColors(ColorDarkRed, ColorRed)
	second.SetButtonColors(ColorDarkBlue, ColorBlue)
	firstStyle, secondStyle := first.themeStyle(), second.themeStyle()
	if firstStyle == secondStyle || firstStyle == &theme.Button {
		t.Fatal("custom action style is shared between buttons or with the theme")
	}
	theme.Button.TextColor = NewColor(30, 40, 50, 255)
	theme.Button.DisabledTextColor = NewColor(80, 90, 100, 255)
	theme.Button.Border = 3
	updated := first.themeStyle()
	if updated.TextColor != theme.Button.TextColor || updated.Border != 3 || updated.disabledTextColor() != theme.Button.DisabledTextColor {
		t.Fatal("custom action style did not follow direct theme edits")
	}
	if updated.Color != ColorDarkRed || updated.HoverColor != ColorRed || secondStyle.Color != ColorDarkBlue {
		t.Fatal("refreshing one button changed explicit action colors")
	}
	first.SetButtonColors(ColorDarkGreen, ColorGreen)
	if first.themeStyle().Color != ColorDarkGreen || second.themeStyle().Color != ColorDarkBlue {
		t.Fatal("replacing action colors changed another button")
	}
}

func TestCustomButtonThemeChangePreservesInheritedStyleComparison(t *testing.T) {
	ctx := newTestContext(t)
	old, next := *baseTheme, *baseTheme
	old.Button.OutlineColor, next.Button.OutlineColor = ColorRed, ColorBlue
	old.Button.ShadowColor, next.Button.ShadowColor = ColorDarkRed, ColorDarkBlue
	old.Button.ShadowSize, next.Button.ShadowSize = 2, 4
	old.Button.ShadowFalloff, next.Button.ShadowFalloff = 1, 3
	button := ctx.NewItem(old.Button)
	button.Theme = &old
	button.SetButtonColors(ColorDarkGreen, ColorGreen)
	updateItemThemeTree([]*itemData{button}, &old, &next)
	if button.Theme != &next || button.OutlineColor != ColorBlue || button.ShadowColor != ColorDarkBlue || button.ShadowSize != 4 || button.ShadowFalloff != 3 {
		t.Fatal("reused custom style prevented inherited appearance from following the new theme")
	}
	if style := button.themeStyle(); style.Color != ColorDarkGreen || style.HoverColor != ColorGreen {
		t.Fatal("theme change replaced explicit action colors")
	}
}
