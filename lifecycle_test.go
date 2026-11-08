package eui

import "testing"

func TestWindowRetirementClearsInteractionAndTabResources(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "close", true: "remove"}[remove], func(t *testing.T) {
			previousWindows, previousActive := windows, activeWindow
			previousFocus, previousSelection, previousHover := focusedItem, selectedTextItem, hoveredItem
			previousSearch, previousDown := activeSearch, downWin
			previousDragWin, previousDragFlow, previousDragPart := dragWin, dragFlow, dragPart
			t.Cleanup(func() {
				windows, activeWindow = previousWindows, previousActive
				focusedItem, selectedTextItem, hoveredItem = previousFocus, previousSelection, previousHover
				activeSearch, downWin = previousSearch, previousDown
				dragWin, dragFlow, dragPart = previousDragWin, previousDragFlow, previousDragPart
			})
			windows = nil
			behind := NewWindow()
			behind.MarkOpen()
			defer behind.deallocate()
			input, _ := NewInput()
			input.Text = "draft"
			tab := NewColumn(input)
			tab.Name = "Editor"
			tabs := NewColumn()
			tabs.Tabs = []*ItemData{tab}
			win := NewWindow()
			win.AddItem(tabs)
			win.MarkOpen()
			before := uiRenderTargets.Stats().Active
			input.Render = uiRenderTargets.Acquire(64, 24, false)
			activeWindow, activeSearch, downWin = win, win, win
			win.searchOpen = true
			Focus(input)
			activeSearch = win
			selectedTextItem, hoveredItem = input, input
			dragWin, dragFlow, dragPart = win, tab, PART_SCROLL_V
			if remove {
				win.RemoveWindow()
			} else {
				win.Close()
			}
			if focusedItem != nil || selectedTextItem != nil || hoveredItem != nil || activeSearch != nil || dragWin != nil {
				t.Fatal("retired window retained interaction state")
			}
			if activeWindow != behind {
				t.Fatal("active window did not fall back")
			}
			if downWin != win {
				t.Fatal("press ownership must survive closing to prevent click-through")
			}
			if input.Render != nil || uiRenderTargets.Stats().Active != before {
				t.Fatal("tab render target was not released")
			}
			if !remove {
				win.MarkOpen()
				if !win.Open || input.ParentWindow != win {
					t.Fatal("closed window could not reopen")
				}
				win.RemoveWindow()
			}
		})
	}
}

func TestDetachedSubtreeDropsFocusAndResources(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"remove", "replace", "set", "window-remove", "window-replace"} {
		t.Run(operation, func(t *testing.T) {
			win := NewWindow()
			input, _ := NewInput()
			root := NewColumn(input)
			win.AddItem(root)
			win.MarkOpen()
			defer win.RemoveWindow()
			Focus(input)
			before := uiRenderTargets.Stats().Active
			input.Render = uiRenderTargets.Acquire(64, 24, false)
			switch operation {
			case "remove":
				root.RemoveItem(input)
			case "replace":
				root.ReplaceItem(0, NewLabel("new"))
			case "set":
				root.SetItems([]*ItemData{NewLabel("new")})
			case "window-remove":
				win.RemoveItem(root)
			case "window-replace":
				win.ReplaceItem(0, NewLabel("new"))
			}
			if focusedItem != nil || input.ParentWindow != nil || input.Render != nil || uiRenderTargets.Stats().Active != before {
				t.Fatal("detached subtree retained focus, window, or render target")
			}
			if operation == "remove" || operation == "replace" || operation == "set" {
				if input.Parent != nil {
					t.Fatal("detached item retained parent")
				}
			}
		})
	}
}

func TestUpdateTextReflowsAutosizedWindow(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	win := NewWindow()
	win.AutoSize = true
	first, second := NewLabel("First"), NewLabel("Second")
	win.AddItem(NewColumn(first, second))
	win.MarkOpen()
	defer win.RemoveWindow()
	initial := win.GetSize()
	win.Dirty = false
	first.UpdateText("A considerably longer first label\nwith another line")
	if !win.Dirty || win.GetSize().X <= initial.X || win.GetSize().Y <= initial.Y {
		t.Fatalf("text update did not repaint and reflow: dirty=%v initial=%v got=%v", win.Dirty, initial, win.GetSize())
	}
	ClearFocus(nil)
}

func TestSetItemsKeepsRetainedChildState(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	kept, _ := NewInput()
	removed := NewLabel("Removed")
	parent := NewColumn(kept, removed)
	win := NewWindow()
	win.AddItem(parent)
	win.MarkOpen()
	defer win.RemoveWindow()
	Focus(kept)
	target := uiRenderTargets.Acquire(64, 24, false)
	kept.Render = target
	parent.SetItems([]*ItemData{kept})
	if focusedItem != kept || kept.Parent != parent || kept.ParentWindow != win || kept.Render != target {
		t.Fatal("kept child lost state while removing sibling")
	}
}
