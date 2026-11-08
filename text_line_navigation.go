package eui

import (
	"sort"
	"strings"
)

func (item *itemData) sourceLine(pos int) editTextLine {
	lines := item.editLayout().logical
	i := sort.Search(len(lines), func(i int) bool { return pos <= lines[i].start+lines[i].length })
	return lines[min(i, len(lines)-1)]
}
func (item *itemData) sourceLineBounds(pos int) (int, int) {
	if item.HideText || item.IsComposing() {
		return editLineBounds(item.editText(), pos)
	}
	line := item.sourceLine(pos)
	return line.start, line.start + line.length
}
func (item *itemData) charBoundary(pos, direction int) int {
	if item.IsComposing() {
		return editCharBoundary(item.editText(), pos, direction)
	}
	line := item.sourceLine(pos)
	local := pos - line.start
	if direction < 0 {
		i := sort.SearchInts(line.stops, local) - 1
		if i >= 0 {
			return line.start + line.stops[i]
		}
		if line.start > 0 && strings.HasSuffix(item.sourceLine(line.start-1).raw, "\r") {
			return max(0, line.start-2)
		}
		return max(0, line.start-1)
	}
	if strings.HasSuffix(line.raw, "\r") && local >= line.length-1 && line.start+line.length < item.editor().runeCount {
		return line.start + line.length + 1
	}
	i := sort.Search(len(line.stops), func(i int) bool { return line.stops[i] > local })
	if i < len(line.stops) {
		return line.start + line.stops[i]
	}
	return min(item.editor().runeCount, line.start+line.length+1)
}
