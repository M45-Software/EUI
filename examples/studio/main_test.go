package main

import (
	"slices"
	"testing"

	eui "github.com/Distortions81/EUI"
)

func findItem(items []*eui.ItemData, text string) *eui.ItemData {
	for _, item := range items {
		if item.Text == text || item.Label == text {
			return item
		}
		if found := findItem(item.Tabs, text); found != nil {
			return found
		}
		if found := findItem(item.Contents, text); found != nil {
			return found
		}
	}
	return nil
}
func TestStudioControlsDriveSceneAndTheme(t *testing.T) {
	s := &studio{speed: .75, count: 48, playing: true, grid: true, trails: true, sceneName: "Test orbit", palette: "AccentDark", style: "Default"}
	if err := s.build(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, w := range append([]*eui.WindowData(nil), eui.Windows()...) {
			w.RemoveWindow()
		}
		s.canvas.Deallocate()
	})
	find := func(name string) *eui.ItemData {
		t.Helper()
		for _, win := range s.windows {
			if item := findItem(win.Contents, name); item != nil {
				return item
			}
		}
		t.Fatalf("missing control %q", name)
		return nil
	}
	find("Pause motion").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if s.playing {
		t.Fatal("pause did not stop motion")
	}
	find("Resume motion").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if !s.playing {
		t.Fatal("resume did not start motion")
	}
	find("Orbit speed").Handler.Emit(eui.UIEvent{Type: eui.EventSliderChanged, Value: 1.5})
	if s.speed != 1.5 {
		t.Fatal("slider did not change speed")
	}
	find("Guide grid").Handler.Emit(eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: false})
	if s.grid {
		t.Fatal("grid checkbox did not change scene")
	}
	s.nameInput.Handler.Emit(eui.UIEvent{Type: eui.EventInputChanged, Text: "Renamed"})
	if s.sceneName != "Renamed" {
		t.Fatal("input did not rename scene")
	}
	s.paletteDD.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: slices.Index(s.paletteDD.Options, "Paper")})
	if s.palette != "Paper" || !eui.IsLightTheme() {
		t.Fatal("theme dropdown did not apply Paper")
	}
	s.styleDD.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: slices.Index(s.styleDD.Options, "Rounded")})
	if s.style != "Rounded" || eui.CurrentStyleName() != "Rounded" {
		t.Fatal("style dropdown did not apply Rounded")
	}
	s.phase = 99
	find("Reset orbit").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if s.phase != 0 {
		t.Fatal("reset did not reset phase")
	}
	before := len(eui.Windows())
	find("Scene info").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if len(eui.Windows()) != before+1 {
		t.Fatal("scene info did not open a dialog")
	}
}

func TestStudioEditorHistoryAndWrapping(t *testing.T) {
	s := &studio{speed: .75, count: 48, playing: true, sceneName: "Editor test", palette: "AccentDark", style: "Default", initialTab: "Editor"}
	if err := s.build(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, w := range append([]*eui.WindowData(nil), eui.Windows()...) {
			w.RemoveWindow()
		}
		s.canvas.Deallocate()
	})
	if s.notebookTabs.Tabs[s.notebookTabs.ActiveTab].Name != "Editor" {
		t.Fatal("editor tab was not selected")
	}
	original := s.editor.Text
	s.editor.ReplaceText("changed document")
	invoke := func(name string, event eui.UIEvent) {
		t.Helper()
		item := findItem(s.notebookTabs.Tabs, name)
		if item == nil {
			t.Fatalf("missing %s", name)
		}
		item.Handler.Emit(event)
	}
	invoke("Undo", eui.UIEvent{Type: eui.EventClick})
	if s.editor.Text != original {
		t.Fatal("Undo toolbar did not restore document")
	}
	invoke("Redo", eui.UIEvent{Type: eui.EventClick})
	if s.editor.Text != "changed document" {
		t.Fatal("Redo toolbar did not restore edit")
	}
	invoke("Wrap lines", eui.UIEvent{Type: eui.EventCheckboxChanged, Checked: true})
	if !s.editor.WordWrap {
		t.Fatal("wrap checkbox did not enable wrapping")
	}
}
