package eui

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestWidgetHandlersDefaultToCallbacksOnly(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	constructors := []func() (*itemData, *EventHandler){ctx.NewButton, ctx.NewCheckbox, ctx.NewRadio, ctx.NewInput, ctx.NewTextArea, ctx.NewSlider, ctx.NewDropdown, ctx.NewColorWheel, ctx.NewText}
	for _, construct := range constructors {
		item, handler := construct()
		if handler == nil || item.Handler != handler || handler.Events() != nil || handler.DroppedEvents() != 0 {
			t.Fatalf("control %v did not start with callback-only delivery", item.ItemType)
		}
		called := 0
		handler.Handle = func(UIEvent) { called++ }
		for range 20 {
			handler.Emit(UIEvent{Item: item})
		}
		if called != 20 || handler.DroppedEvents() != 0 {
			t.Fatal("callback-only events were lost or counted as dropped")
		}
	}
	menu := ctx.ShowContextMenu([]string{"Choose"}, 0, 0, nil)
	t.Cleanup(ctx.CloseContextMenus)
	if menu.Handler == nil || menu.Handler.Events() != nil {
		t.Fatal("context menu omitted callback-only handler")
	}
}

func TestEventQueueFIFOOverflowAndCallbackDelivery(t *testing.T) {
	var handler EventHandler
	queue := handler.EnableQueue(2)
	calls := []int{}
	handler.Handle = func(ev UIEvent) { calls = append(calls, ev.Index) }
	for i := 1; i <= 4; i++ {
		handler.Emit(UIEvent{Index: i})
	}
	if handler.Events() != queue || len(queue) != 2 || handler.DroppedEvents() != 2 || !reflect.DeepEqual(calls, []int{1, 2, 3, 4}) {
		t.Fatalf("queue/callback mismatch: pending=%d dropped=%d callbacks=%v", len(queue), handler.DroppedEvents(), calls)
	}
	if (<-queue).Index != 1 {
		t.Fatal("overflow displaced oldest pending event")
	}
	handler.Emit(UIEvent{Index: 5})
	if (<-queue).Index != 2 || (<-queue).Index != 5 || handler.DroppedEvents() != 2 {
		t.Fatal("queue did not recover in FIFO order after draining")
	}
	handler.DisableQueue()
}

func TestEventQueueLifecycleAndInvalidCapacity(t *testing.T) {
	var handler EventHandler
	old := handler.EnableQueue(1)
	handler.Emit(UIEvent{Index: 1})
	handler.Emit(UIEvent{Index: 2})
	next := handler.EnableQueue(3)
	if ev, ok := <-old; !ok || ev.Index != 1 {
		t.Fatal("reconfiguration discarded buffered events")
	}
	if _, ok := <-old; ok {
		t.Fatal("replaced queue was not closed")
	}
	if next == old || cap(next) != 3 || len(next) != 0 || handler.DroppedEvents() != 0 {
		t.Fatal("new queue did not reset its state")
	}
	handler.Emit(UIEvent{Index: 3})
	handler.DisableQueue()
	handler.DisableQueue()
	if handler.Events() != nil {
		t.Fatal("disabled queue remains exposed")
	}
	if ev, ok := <-next; !ok || ev.Index != 3 {
		t.Fatal("disable discarded buffered event")
	}
	if _, ok := <-next; ok {
		t.Fatal("disabled queue was not closed")
	}
	called := 0
	handler.Handle = func(UIEvent) { called++ }
	handler.Emit(UIEvent{})
	if called != 1 {
		t.Fatal("disabling queue disabled callback")
	}
	for _, capacity := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("capacity %d did not panic", capacity)
				}
			}()
			handler.EnableQueue(capacity)
		}()
	}
	var absent *EventHandler
	absent.Emit(UIEvent{})
	absent.DisableQueue()
	if absent.Events() != nil || absent.DroppedEvents() != 0 || absent.EnableQueue(1) != nil {
		t.Fatal("nil handler was not harmless")
	}
}

func TestEventQueueDropCountSurvivesDisable(t *testing.T) {
	handler := newHandler()
	handler.EnableQueue(1)
	handler.Emit(UIEvent{})
	handler.Emit(UIEvent{})
	handler.DisableQueue()
	if handler.DroppedEvents() != 1 {
		t.Fatal("disable erased overflow evidence")
	}
	handler.EnableQueue(1)
	if handler.DroppedEvents() != 0 {
		t.Fatal("enable did not reset overflow count")
	}
	handler.DisableQueue()
}

func TestRecursiveEventsQueueBeforeSynchronousCallbacks(t *testing.T) {
	var handler EventHandler
	queue := handler.EnableQueue(4)
	var order []string
	handler.Handle = func(ev UIEvent) {
		order = append(order, fmt.Sprintf("start%d", ev.Index))
		if len(queue) != ev.Index {
			t.Fatal("callback ran before its event was queued")
		}
		if ev.Index == 1 {
			handler.Emit(UIEvent{Index: 2})
		}
		order = append(order, fmt.Sprintf("end%d", ev.Index))
	}
	handler.Emit(UIEvent{Index: 1})
	if !reflect.DeepEqual(order, []string{"start1", "start2", "end2", "end1"}) || (<-queue).Index != 1 || (<-queue).Index != 2 {
		t.Fatalf("recursive ordering: %v", order)
	}
	handler.DisableQueue()
}

func TestCallbackCanReconfigureQueueAndReemit(t *testing.T) {
	var handler EventHandler
	old := handler.EnableQueue(1)
	var next <-chan UIEvent
	handler.Handle = func(ev UIEvent) {
		if ev.Index == 1 {
			next = handler.EnableQueue(2)
			handler.Emit(UIEvent{Index: 2})
		} else {
			handler.DisableQueue()
		}
	}
	handler.Emit(UIEvent{Index: 1})
	if (<-old).Index != 1 || (<-next).Index != 2 {
		t.Fatal("callback reconfiguration lost pending event")
	}
	if _, ok := <-old; ok {
		t.Fatal("old queue not closed")
	}
	if _, ok := <-next; ok {
		t.Fatal("new queue not closed")
	}
}

func TestActivationSnapshotsCallbacksAndQueuesBeforeAction(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	button, handler := ctx.NewButton()
	win.AddItem(button)
	queue := handler.EnableQueue(4)
	var order []string
	button.Action = func() { order = append(order, "original") }
	handler.Handle = func(ev UIEvent) {
		if ev.Item != button || ev.Type != EventClick || len(queue) != 1 {
			t.Fatal("callback preceded event or lost target")
		}
		order = append(order, "handle")
		button.Action = func() { order = append(order, "replacement") }
	}
	ctx.key(t, button, ebiten.KeySpace)
	<-queue
	button.DrawRect = Rect{X1: 100, Y1: 40}
	ctx.activeItem = nil
	button.clickItem(Point{X: 10, Y: 10}, true)
	if !reflect.DeepEqual(order, []string{"handle", "original", "handle", "replacement"}) {
		t.Fatalf("callback mutation changed in-flight action: %v", order)
	}
	handler.DisableQueue()
}

func TestDropdownDeliveryMatchesPointerAndKeyboard(t *testing.T) {
	ctx := newTestContext(t)

	for _, context := range []bool{false, true} {
		for _, pointer := range []bool{false, true} {
			t.Run(fmt.Sprintf("context=%t/pointer=%t", context, pointer), func(t *testing.T) {
				win := ctx.keyboardFixture(t)
				ctx.CloseContextMenus()
				t.Cleanup(ctx.CloseContextMenus)
				var item *ItemData
				if context {
					item = ctx.ShowContextMenu([]string{"Header", "First", "Second"}, 20, 20, nil)
				} else {
					item, _ = ctx.NewDropdown()
					item.SetOptions([]string{"Header", "First", "Second"})
					win.AddItem(item)
					item.DrawRect = Rect{X0: 20, Y0: 20, X1: 220, Y1: 50}
				}
				item.HeaderCount = 1
				item.Open = true
				item.HoverIndex = 2
				queue := item.Handler.EnableQueue(4)
				var order []string
				item.OnHover = func(index int) {
					if index != 1 || item.Open || item.HoverIndex != -1 {
						t.Fatal("selection did not end preview before notifying")
					}
					order = append(order, "preview")
				}
				item.Handler.Handle = func(ev UIEvent) {
					if ev.Type != EventDropdownSelected || ev.Index != 1 || item.Selected != 1 || item.Open || len(queue) != 1 {
						t.Fatal("selection event did not follow committed state")
					}
					order = append(order, "handle")
				}
				item.OnSelect = func(index int) {
					if index != 1 {
						t.Fatal("selection payload changed")
					}
					order = append(order, "select")
					if context {
						ctx.ShowContextMenu([]string{"Follow-up"}, 20, 20, nil)
					}
				}
				if pointer {
					r, _ := ctx.dropdownOpenRect(item, Point{X: 20, Y: 20})
					pos := Point{X: r.X0 + 1, Y: r.Y0 + 1.5*ctx.dropdownOptionHeight(item)}
					if context {
						ctx.handleContextMenus(pos, true)
					} else {
						item.clickItem(pos, true)
					}
				} else {
					item.HoverIndex = 1
					ctx.key(t, item, ebiten.KeyEnter)
				}
				if !reflect.DeepEqual(order, []string{"preview", "handle", "select"}) {
					t.Fatalf("delivery order: %v", order)
				}
				if context && (len(ctx.contextMenus) != 1 || ctx.contextMenus[0] == item) {
					t.Fatal("selection discarded follow-up menu")
				}
				item.Handler.DisableQueue()
			})
		}
	}
}

func TestColorDeliveryUsesSnapshotAndIgnoresUnchangedValue(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	wheel, handler := ctx.NewColorWheel()
	win.AddItem(wheel)
	original := NewColor(200, 30, 50, 255)
	replacement := NewColor(20, 90, 210, 255)
	queue := handler.EnableQueue(2)
	var order []string
	handler.Handle = func(ev UIEvent) {
		if ev.Color != original || wheel.WheelColor != original {
			t.Fatal("color event preceded model update")
		}
		order = append(order, "handle")
		wheel.SetWheelColor(replacement)
		wheel.OnColorChange = func(Color) { t.Fatal("replacement callback handled current event") }
	}
	wheel.OnColorChange = func(value Color) {
		if value != original {
			t.Fatal("color callback read mutated live state")
		}
		order = append(order, "color")
	}
	wheel.Action = func() { order = append(order, "action") }
	wheel.setUserColor(original)
	wheel.setUserColor(replacement)
	if !reflect.DeepEqual(order, []string{"handle", "color", "action"}) || len(queue) != 1 || (<-queue).Color != original {
		t.Fatalf("color deliveries: %v", order)
	}
	handler.DisableQueue()
}

func TestSliderChangedActionIsNotLostWhenHandlerResetsValue(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	for _, vertical := range []bool{false, true} {
		slider, handler := ctx.NewSlider()
		slider.Vertical = vertical
		slider.HideValue = true
		slider.SetRange(0, 100)
		slider.SetValue(0)
		win.AddItem(slider)
		slider.DrawRect = Rect{X1: 240, Y1: 240}
		slider.clickItem(Point{X: 30, Y: 30}, true)
		changes, actions, releases := 0, 0, 0
		handler.Handle = func(ev UIEvent) {
			if ev.Type == EventSliderChanged {
				changes++
				slider.SetValue(0)
			}
			if ev.Type == EventSliderReleased {
				releases++
			}
		}
		slider.Action = func() { actions++ }
		slider.setSliderValueWithModifiers(Point{X: 120, Y: 120}, false, 1)
		slider.emitSliderReleased()
		if changes != 1 || actions != 1 || releases != 0 {
			t.Fatalf("vertical=%t changed=%d actions=%d release=%d", vertical, changes, actions, releases)
		}
	}
}

func TestQueuedTextSnapshotsAndMaskedPasswords(t *testing.T) {
	ctx := newTestContext(t)

	ctx.keyboardFixture(t)
	for _, password := range []bool{false, true} {
		input, handler := ctx.NewInput()
		input.HideText = password
		input.UpdateText("")
		queue := handler.EnableQueue(4)
		input.editReplace(0, 0, "one", "")
		input.editReplace(0, 3, "two!", "")
		first, second := <-queue, <-queue
		wantFirst, wantSecond := "one", "two!"
		if password {
			wantFirst, wantSecond = "***", "****"
		}
		if first.Type != EventInputChanged || first.Text != wantFirst || second.Text != wantSecond || first.Item != input {
			t.Fatalf("password=%t queued snapshots: %q %q", password, first.Text, second.Text)
		}
		handler.DisableQueue()
	}
}

func ExampleEventHandler_EnableQueue() {
	var handler EventHandler
	events := handler.EnableQueue(2)
	handler.Emit(UIEvent{Type: EventClick})
	handler.DisableQueue()
	for event := range events {
		fmt.Println(event.Type == EventClick)
	}
	fmt.Println(handler.DroppedEvents())
	// Output:
	// true
	// 0
}

func TestColorPointerAndKeyboardUseSameCallbackOrder(t *testing.T) {
	ctx := newTestContext(t)

	for _, pointer := range []bool{false, true} {
		t.Run(fmt.Sprintf("pointer=%t", pointer), func(t *testing.T) {
			win := ctx.keyboardFixture(t)
			wheel, handler := ctx.NewColorWheel()
			wheel.SetSize(Point{X: 100, Y: 100})
			wheel.SetWheelColor(NewColorHSV(120, .5, .5, 1))
			win.AddItem(wheel)
			wheel.DrawRect = Rect{X1: 100, Y1: 100}
			queue := handler.EnableQueue(1)
			var order []string
			var payload Color
			handler.Handle = func(ev UIEvent) {
				if len(queue) != 1 || ev.Type != EventColorChanged || ev.Color != wheel.WheelColor {
					t.Fatal("color delivery preceded state or queue")
				}
				payload = ev.Color
				order = append(order, "handle")
			}
			wheel.OnColorChange = func(value Color) {
				if value != payload {
					t.Fatal("color callback payload mismatch")
				}
				order = append(order, "color")
			}
			wheel.Action = func() { order = append(order, "action") }
			if pointer {
				wheel.clickItem(Point{X: 80, Y: 50}, true)
			} else {
				ctx.key(t, wheel, ebiten.KeyArrowRight)
			}
			if !reflect.DeepEqual(order, []string{"handle", "color", "action"}) {
				t.Fatalf("color order: %v", order)
			}
			handler.DisableQueue()
		})
	}
}

func TestFullWidgetQueueDoesNotSuppressCallbacks(t *testing.T) {
	ctx := newTestContext(t)

	win := ctx.keyboardFixture(t)
	check, handler := ctx.NewCheckbox()
	win.AddItem(check)
	queue := handler.EnableQueue(1)
	callbacks, actions := 0, 0
	handler.Handle = func(UIEvent) { callbacks++ }
	check.Action = func() { actions++ }
	for range 10 {
		ctx.key(t, check, ebiten.KeySpace)
	}
	first := <-queue
	if callbacks != 10 || actions != 10 || handler.DroppedEvents() != 9 || !first.Checked || check.Checked {
		t.Fatalf("overflow lost callbacks or changed snapshots: callbacks=%d actions=%d dropped=%d", callbacks, actions, handler.DroppedEvents())
	}
	handler.DisableQueue()
}
