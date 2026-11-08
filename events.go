package eui

// UIEventType defines the kind of event emitted by widgets.
type UIEventType int

const (
	EventClick UIEventType = iota
	EventSliderChanged
	EventDropdownSelected
	EventCheckboxChanged
	EventRadioSelected
	EventColorChanged
	EventInputChanged
	// EventSliderReleased ends a changed pointer or keyboard slider interaction.
	EventSliderReleased
)

// UIEvent snapshots a user interaction after the widget state is updated.
// Payload fields are values; Item is a live handle, not a copy of widget state.
// Password input Text contains the masked display, not the secret value.
type UIEvent struct {
	Item    *ItemData
	Type    UIEventType
	Value   float32
	Index   int
	Checked bool
	Color   Color
	Text    string
}

// EventHandler delivers events synchronously through Handle. Its zero value is
// ready to use and allocates no event queue. Configure handlers and emit events
// on the EUI/game goroutine; a queue may be received from another goroutine, but
// the event's Item still belongs to the EUI/game goroutine. Do not copy a handler
// after enabling its queue.
type EventHandler struct {
	owner   *Context
	Handle  func(UIEvent)
	events  chan UIEvent
	dropped uint64
}

// EnableQueue opts into bounded FIFO delivery in addition to Handle. Capacity
// must be positive or EnableQueue panics. Reconfiguring closes the previous
// channel (which can still be drained), creates an empty queue, and resets the
// dropped-event count. EUI owns the channel and never waits for a receiver.
func (h *EventHandler) EnableQueue(capacity int) <-chan UIEvent {
	if capacity <= 0 {
		panic("eui: event queue capacity must be positive")
	}
	if h == nil {
		return nil
	}
	next := make(chan UIEvent, capacity)
	h.DisableQueue()
	h.events = next
	h.dropped = 0
	return h.events
}

// Events returns the current receive-only queue, or nil when queueing is off.
func (h *EventHandler) Events() <-chan UIEvent {
	if h == nil {
		return nil
	}
	return h.events
}

// DisableQueue stops queueing and closes its channel. Buffered events remain
// readable. Handle continues receiving events, and DroppedEvents is retained
// until the next EnableQueue call. Repeated calls are harmless.
func (h *EventHandler) DisableQueue() {
	if h == nil || h.events == nil {
		return
	}
	close(h.events)
	h.events = nil
}

// DroppedEvents counts events rejected by a full queue since EnableQueue.
// Overflow discards the newest event, preserving the pending FIFO sequence.
// Handle still receives every event. Read this count on the EUI/game goroutine.
func (h *EventHandler) DroppedEvents() uint64 {
	if h == nil {
		return 0
	}
	return h.dropped
}

// Emit enqueues a value snapshot before synchronously calling Handle. A full
// queue drops this event without blocking or skipping Handle. Recursive emits
// run immediately: their callbacks finish before the outer callback returns,
// while each queue retains emit order. A nil handler is a no-op.
// Emit only delivers to this handler; it does not activate a widget or invoke
// its convenience callbacks.
func (h *EventHandler) Emit(ev UIEvent) {
	if h != nil && h.owner != nil && (h.owner.disposed || h.owner.disposePending) {
		return
	}

	if h == nil {
		return
	}
	if h.events != nil {
		select {
		case h.events <- ev:
		default:
			h.dropped++
		}
	}
	if h.Handle != nil {
		h.Handle(ev)
	}
}

func newHandler() *EventHandler { return &EventHandler{} }

// emitEvent runs the event handler before the control's convenience callback.
// Capture callbacks and payloads so changes made inside Handle affect the next
// interaction, not the one already being delivered.
func (item *itemData) emitEvent(ev UIEvent) {
	ctx := item.ui()

	ev.Item = item
	action, onSelect, onColor := item.Action, item.OnSelect, item.OnColorChange
	item.Handler.Emit(ev)
	switch ev.Type {
	case EventClick, EventCheckboxChanged, EventRadioSelected, EventSliderChanged:
		if action != nil {
			action()
		}
	case EventDropdownSelected:
		if onSelect != nil {
			onSelect(ev.Index)
		}
	case EventColorChanged:
		if onColor != nil {
			onColor(ev.Color)
		} else {
			ctx.SetAccentColor(ev.Color)
		}
		if action != nil {
			action()
		}
	}
}

// setUserColor shares change-only semantics for pointer and keyboard input.
func (item *itemData) setUserColor(value Color) {
	if item.WheelColor == value {
		return
	}
	item.WheelColor = value
	invalidateFocus(item)
	item.emitEvent(UIEvent{Type: EventColorChanged, Color: value})
}
