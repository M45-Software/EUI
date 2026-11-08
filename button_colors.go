package eui

type buttonColorOverride struct {
	normal, hover Color
	style         itemData
}

// SetButtonColors sets action-specific colors while retaining the current
// theme's text contrast, geometry, and disabled appearance.
func (item *itemData) SetButtonColors(normal, hover Color) {
	if item == nil || item.ItemType != ITEM_BUTTON {
		return
	}
	if previous := item.buttonColors; previous != nil && previous.normal == normal && previous.hover == hover {
		return
	}
	item.buttonColors = &buttonColorOverride{normal: normal, hover: hover}
	item.runtimeChanged(false)
}
