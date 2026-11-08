package eui

import "testing"

func TestWindowRetirementClearsInteractionAndTabResources(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "close", true: "remove"}[remove], func(t *testing.T) {
			previousWindows, previousActive := ctx.windows, ctx.activeWindow
			previousFocus, previousSelection, previousHover := ctx.focusedItem, ctx.selectedTextItem, ctx.hoveredItem
			previousSearch, previousDown := ctx.activeSearch, ctx.downWin
			previousDragWin, previousDragFlow, previousDragPart := ctx.dragWin, ctx.dragFlow, ctx.dragPart
			t.Cleanup(func() {
				ctx.windows, ctx.activeWindow = previousWindows, previousActive
				ctx.focusedItem, ctx.selectedTextItem, ctx.hoveredItem = previousFocus, previousSelection, previousHover
				ctx.activeSearch, ctx.downWin = previousSearch, previousDown
				ctx.dragWin, ctx.dragFlow, ctx.dragPart = previousDragWin, previousDragFlow, previousDragPart
			})
			ctx.windows = nil
			behind := ctx.NewWindow()
			behind.MarkOpen()
			defer behind.deallocate()
			input, _ := ctx.NewInput()
			input.Text = "draft"
			tab := ctx.NewColumn(input)
			tab.Name = "Editor"
			tabs := ctx.NewColumn()
			tabs.Tabs = []*ItemData{tab}
			win := ctx.NewWindow()
			win.AddItem(tabs)
			win.MarkOpen()
			before := ctx.uiRenderTargets.Stats().Active
			input.Render = ctx.uiRenderTargets.Acquire(64, 24, false)
			ctx.activeWindow, ctx.activeSearch, ctx.downWin = win, win, win
			win.searchOpen = true
			ctx.Focus(input)
			ctx.activeSearch = win
			ctx.selectedTextItem, ctx.hoveredItem = input, input
			ctx.dragWin, ctx.dragFlow, ctx.dragPart = win, tab, PART_SCROLL_V
			if remove {
				win.RemoveWindow()
			} else {
				win.Close()
			}
			if ctx.focusedItem != nil || ctx.selectedTextItem != nil || ctx.hoveredItem != nil || ctx.activeSearch != nil || ctx.dragWin != nil {
				t.Fatal("retired window retained interaction state")
			}
			if ctx.activeWindow != behind {
				t.Fatal("active window did not fall back")
			}
			if ctx.downWin != win {
				t.Fatal("press ownership must survive closing to prevent click-through")
			}
			if input.Render != nil || ctx.uiRenderTargets.Stats().Active != before {
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
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"remove", "replace", "set", "window-remove", "window-replace"} {
		t.Run(operation, func(t *testing.T) {
			win := ctx.NewWindow()
			input, _ := ctx.NewInput()
			root := ctx.NewColumn(input)
			win.AddItem(root)
			win.MarkOpen()
			defer win.RemoveWindow()
			ctx.Focus(input)
			before := ctx.uiRenderTargets.Stats().Active
			input.Render = ctx.uiRenderTargets.Acquire(64, 24, false)
			switch operation {
			case "remove":
				root.RemoveItem(input)
			case "replace":
				root.ReplaceItem(0, ctx.NewLabel("new"))
			case "set":
				root.SetItems([]*ItemData{ctx.NewLabel("new")})
			case "window-remove":
				win.RemoveItem(root)
			case "window-replace":
				win.ReplaceItem(0, ctx.NewLabel("new"))
			}
			if ctx.focusedItem != nil || input.ParentWindow != nil || input.Render != nil || ctx.uiRenderTargets.Stats().Active != before {
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
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	win := ctx.NewWindow()
	win.AutoSize = true
	first, second := ctx.NewLabel("First"), ctx.NewLabel("Second")
	win.AddItem(ctx.NewColumn(first, second))
	win.MarkOpen()
	defer win.RemoveWindow()
	initial := win.GetSize()
	win.Dirty = false
	first.UpdateText("A considerably longer first label\nwith another line")
	if !win.Dirty || win.GetSize().X <= initial.X || win.GetSize().Y <= initial.Y {
		t.Fatalf("text update did not repaint and reflow: dirty=%v initial=%v got=%v", win.Dirty, initial, win.GetSize())
	}
	ctx.ClearFocus(nil)
}

func TestSetItemsKeepsRetainedChildState(t *testing.T) {
	ctx := newTestContext(t)

	if err := ctx.initFonts(); err != nil {
		t.Fatal(err)
	}
	kept, _ := ctx.NewInput()
	removed := ctx.NewLabel("Removed")
	parent := ctx.NewColumn(kept, removed)
	win := ctx.NewWindow()
	win.AddItem(parent)
	win.MarkOpen()
	defer win.RemoveWindow()
	ctx.Focus(kept)
	target := ctx.uiRenderTargets.Acquire(64, 24, false)
	kept.Render = target
	parent.SetItems([]*ItemData{kept})
	if ctx.focusedItem != kept || kept.Parent != parent || kept.ParentWindow != win || kept.Render != target {
		t.Fatal("kept child lost state while removing sibling")
	}
}
