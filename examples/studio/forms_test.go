package main

import (
	"testing"

	eui "github.com/M45-Software/EUI"
)

func TestStudioSceneSettingsApplyAndCancel(t *testing.T) {
	s := &studio{speed: .75, count: 48, sceneName: "Original scene", palette: "AccentDark", style: "Default", initialTab: "Forms"}
	if err := s.build(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.ui.Dispose)
	t.Cleanup(s.canvas.Deallocate)
	slider := findItem(s.viewport.Contents, "Orbit speed")
	picker := findItem(s.viewport.Contents, "48 particles")
	// Dropdown captions are stored in Options, not Text.
	if picker == nil {
		var visit func([]*eui.ItemData) *eui.ItemData
		visit = func(items []*eui.ItemData) *eui.ItemData {
			for _, it := range items {
				if len(it.Options) > 0 && it.Options[0] == "12 particles" {
					return it
				}
				if got := visit(it.Contents); got != nil {
					return got
				}
				if got := visit(it.Tabs); got != nil {
					return got
				}
			}
			return nil
		}
		picker = visit(s.viewport.Contents)
	}
	if slider == nil || picker == nil {
		t.Fatal("missing scene controls")
	}
	win := s.openSceneSettings(slider, picker)
	name := findItem(win.Contents, "Scene name")
	speed := findItem(win.Contents, "Orbit speed")
	count := findItem(win.Contents, "Particles")
	apply := findItem(win.Contents, "Apply")
	name.UpdateText("Changed scene")
	speed.UpdateText("1.5")
	count.UpdateText("201")
	apply.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if !win.Open || s.sceneName != "Original scene" || s.speed != .75 || s.count != 48 {
		t.Fatal("invalid form partially changed scene")
	}
	count.UpdateText("100")
	apply.Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if win.Open || s.sceneName != "Changed scene" || s.speed != 1.5 || s.count != 100 || s.nameInput.Text != s.sceneName || slider.Value != s.speed || picker.Options[picker.Selected] != "100 particles" {
		t.Fatal("Apply failed to synchronize scene controls")
	}
	win = s.openSceneSettings(slider, picker)
	findItem(win.Contents, "Scene name").UpdateText("Discard")
	win.Close()
	if s.sceneName != "Changed scene" || s.ui.ModalWindow() != nil {
		t.Fatal("close committed draft or retained modal")
	}
}
