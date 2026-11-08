package main

import (
	"slices"
	"testing"

	eui "github.com/Distortions81/EUI"
	"github.com/hajimehoshi/ebiten/v2"
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
	t.Cleanup(s.ui.Dispose)
	t.Cleanup(func() {
		for _, w := range append([]*eui.WindowData(nil), s.ui.Windows()...) {
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
	if s.palette != "Paper" || !s.ui.IsLightTheme() {
		t.Fatal("theme dropdown did not apply Paper")
	}
	s.styleDD.Handler.Emit(eui.UIEvent{Type: eui.EventDropdownSelected, Index: slices.Index(s.styleDD.Options, "Rounded")})
	if s.style != "Rounded" || s.ui.CurrentStyleName() != "Rounded" {
		t.Fatal("style dropdown did not apply Rounded")
	}
	s.phase = 99
	find("Reset orbit").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if s.phase != 0 {
		t.Fatal("reset did not reset phase")
	}
	before := len(s.ui.Windows())
	find("Scene info").Handler.Emit(eui.UIEvent{Type: eui.EventClick})
	if len(s.ui.Windows()) != before+1 {
		t.Fatal("scene info did not open a dialog")
	}
}

func TestStudioEditorHistoryAndWrapping(t *testing.T) {
	s := &studio{speed: .75, count: 48, playing: true, sceneName: "Editor test", palette: "AccentDark", style: "Default", initialTab: "Editor"}
	if err := s.build(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.ui.Dispose)
	t.Cleanup(func() {
		for _, w := range append([]*eui.WindowData(nil), s.ui.Windows()...) {
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

func TestStudioResponsiveBreakpointsPreserveEditorAndReachability(t *testing.T) {
	s := &studio{speed: .75, count: 48, playing: true, sceneName: "Responsive test", palette: "AccentDark", style: "Default", initialTab: "Editor"}
	if err := s.build(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.ui.Dispose)
	t.Cleanup(func() { s.viewport.RemoveWindow(); s.canvas.Deallocate() })
	original := s.editor.Text
	s.editor.ReplaceText("A draft retained across responsive layout changes.")
	s.ui.Focus(s.editor)
	s.editor.SelectStart, s.editor.SelectEnd, s.editor.CursorPos = 2, 8, 8
	for _, size := range [][2]int{{1360, 900}, {960, 800}, {480, 800}, {360, 640}, {760, 600}, {1140, 900}, {1360, 900}} {
		w, h := s.Layout(size[0], size[1])
		if w != size[0] || h != size[1] {
			t.Fatal("Layout forced the desktop canvas size")
		}
		if s.ui.FocusedItem() != s.editor || s.editor.SelectStart != 2 || s.editor.SelectEnd != 8 || s.editor.CursorPos != 8 {
			t.Fatalf("%dx%d resize cleared focus/caret/selection", w, h)
		}
		seen := map[*eui.ItemData]bool{}
		var checkTree func(*eui.ItemData, *eui.ItemData)
		checkTree = func(item, parent *eui.ItemData) {
			if seen[item] || item.Parent != parent || item.ParentWindow != s.viewport {
				t.Fatal("breakpoint left duplicate nodes or stale ownership")
			}
			seen[item] = true
			for _, child := range item.Contents {
				checkTree(child, item)
			}
			for _, page := range item.Tabs {
				checkTree(page, item)
			}
		}
		checkTree(s.page, nil)
		if !seen[s.editor] || !seen[s.accent] || !seen[s.imageItem] {
			t.Fatal("breakpoint omitted a section")
		}
		if s.page.GetSize().X > s.viewport.GetSize().X+.01 {
			t.Fatalf("%dx%d workspace overflows horizontally", w, h)
		}
		screen := ebiten.NewImage(w, h)
		s.Draw(screen)
		editor := s.editor.DrawRect
		page := s.viewport.GetPos()
		if editor.X1 <= editor.X0 || editor.Y1 <= editor.Y0 || editor.X0 < page.X || editor.X1 > page.X+s.viewport.GetSize().X+.01 || editor.Y0 < page.Y || editor.Y1 > page.Y+s.viewport.GetSize().Y+.01 {
			t.Fatalf("%dx%d focused editor unreachable: %v", w, h, editor)
		}
		if s.canvas.Bounds().Dx() != int(s.imageItem.GetSize().X) {
			t.Fatal("canvas raster was not resized with its layout box")
		}
		screen.Deallocate()
	}
	s.editor.Undo()
	if s.editor.Text != original {
		t.Fatal("resizing discarded editor history")
	}
}
