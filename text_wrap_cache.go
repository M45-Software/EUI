package eui

import (
	"math"
	"sort"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

type editWrapResult struct {
	minimum, maximum float32        // same break partition for [minimum, maximum)
	rows             []editTextLine // source starts relative to the logical line
}

// Keep two partitions per paragraph, independent of its absolute source offset.
// Cached row data is immutable; published layouts never alias a mutable row slice.
type editLineWrapCache struct{ recent, previous editWrapResult }

func (cache *editLineWrapCache) appendRows(dst []editTextLine, line editTextLine, face text.Face, width float32) []editTextLine {
	result := cache.recent
	if result.rows == nil || width < result.minimum || width >= result.maximum {
		result = cache.previous
		if result.rows == nil || width < result.minimum || width >= result.maximum {
			local := line
			local.start = 0
			rows := wrapEditLine(local, line.raw, face, width)
			minimum, maximum := wrapWidthInterval(local, rows, face)
			for i := range rows {
				rows[i].raw = ""
			}
			// Conservatively retain an exact-width result if unusual font metrics
			// do not satisfy the monotonic advances assumed by the wrapping search.
			if width < minimum || width >= maximum {
				minimum, maximum = width, math.Nextafter32(width, float32(math.Inf(1)))
			}
			result = editWrapResult{minimum: minimum, maximum: maximum, rows: rows}
		}
		cache.previous, cache.recent = cache.recent, result
	}
	for _, row := range result.rows {
		row.start += line.start
		dst = append(dst, row)
	}
	return dst
}

// A row survives narrowing while its last chosen grapheme fits. It survives
// widening until the next possible whitespace break (or next grapheme inside a
// long word) fits. Intersect these intervals for all visual rows. This preserves
// exact layout during smooth resize without re-shaping every paragraph per pixel.
func wrapWidthInterval(line editTextLine, rows []editTextLine, face text.Face) (float32, float32) {
	minimum, maximum := float32(0), float32(math.Inf(1))
	runes := []rune(line.raw)
	for _, row := range rows {
		if len(row.stops) > 2 {
			minimum = max(minimum, row.width)
		}
		if !row.softBreak {
			continue
		}
		end := row.start + row.length
		next := sort.SearchInts(line.stops, end) + 1
		if end > 0 && unicode.IsSpace(runes[end-1]) {
			for next < len(line.stops)-1 && !unicode.IsSpace(runes[line.stops[next]-1]) {
				next++
			}
		}
		value := line.display[line.bytes[row.start]:line.bytes[line.stops[next]]]
		limit := float32(text.AdvanceAt(value, len(value), face))
		maximum = min(maximum, limit)
	}
	return minimum, maximum
}
