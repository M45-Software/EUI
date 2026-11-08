package eui

import "unicode/utf8"

// editBuffer keeps a UTF-8 gap at the last edit. Public Text/undo snapshots are
// immutable strings; publishing a changed document still copies its bytes once.
// Rune offsets are translated relative to the gap, without decoding the whole
// document or allocating a whole-document []rune for each local edit.
type editBuffer struct {
	data           []byte
	lo, hi         int
	gapRune, runes int
}

func newEditBuffer(value string) *editBuffer {
	if !utf8.ValidString(value) {
		value = string([]rune(value))
	}
	gap := bufferReserve(len(value))
	b := &editBuffer{data: make([]byte, len(value)+gap), lo: len(value), hi: len(value) + gap, runes: utf8.RuneCountInString(value)}
	b.gapRune = b.runes
	copy(b.data, value)
	return b
}
func bufferReserve(size int) int    { return max(64, min(64<<10, size/8)) }
func (b *editBuffer) len() int      { return len(b.data) - (b.hi - b.lo) }
func (b *editBuffer) value() string { return string(b.data[:b.lo]) + string(b.data[b.hi:]) }

func (b *editBuffer) moveRune(pos int) {
	pos = max(0, min(pos, b.runes))
	left, right := b.lo, b.hi
	for b.gapRune > pos {
		_, n := utf8.DecodeLastRune(b.data[:left])
		left -= n
		b.gapRune--
	}
	for b.gapRune < pos {
		_, n := utf8.DecodeRune(b.data[right:])
		right += n
		b.gapRune++
	}
	if left < b.lo {
		n := b.lo - left
		copy(b.data[b.hi-n:b.hi], b.data[left:b.lo])
		b.lo, b.hi = left, b.hi-n
		return
	}
	if right > b.hi {
		n := right - b.hi
		copy(b.data[b.lo:b.lo+n], b.data[b.hi:right])
		b.lo, b.hi = b.lo+n, right
	}
}
func (b *editBuffer) reserve(needed int) {
	if b.hi-b.lo >= needed {
		return
	}
	data := make([]byte, b.len()+needed+bufferReserve(b.len()+needed))
	hi := len(data) - (len(b.data) - b.hi)
	copy(data, b.data[:b.lo])
	copy(data[hi:], b.data[b.hi:])
	b.data, b.hi = data, hi
}
func (b *editBuffer) replace(start, end int, insert string) string {
	if !utf8.ValidString(insert) {
		insert = string([]rune(insert))
	}
	b.moveRune(start)
	for range end - start {
		_, n := utf8.DecodeRune(b.data[b.hi:])
		b.hi += n
	}
	count := utf8.RuneCountInString(insert)
	b.runes += count - (end - start)
	b.reserve(len(insert))
	copy(b.data[b.lo:], insert)
	b.lo += len(insert)
	b.gapRune += count
	b.compact()
	return b.value()
}
func (b *editBuffer) compact() {
	// A large replacement/deletion should not retain the former peak capacity.
	if len(b.data) <= max(4096, b.len()*4) {
		return
	}
	data := make([]byte, b.len()+bufferReserve(b.len()))
	hi := len(data) - (len(b.data) - b.hi)
	copy(data, b.data[:b.lo])
	copy(data[hi:], b.data[b.hi:])
	b.data, b.hi = data, hi
}

// Restore the differing region of an immutable undo snapshot. Byte comparisons
// find the edit without decoding the shared prefix/suffix into rune slices.
func (b *editBuffer) restore(previous, value string) {
	if !utf8.ValidString(previous) || !utf8.ValidString(value) {
		*b = *newEditBuffer(value)
		return
	}
	prefix := 0
	for prefix < min(len(previous), len(value)) && previous[prefix] == value[prefix] {
		prefix++
	}
	for prefix > 0 && prefix < len(previous) && !utf8.RuneStart(previous[prefix]) {
		prefix--
	}
	suffix := 0
	for suffix < min(len(previous), len(value))-prefix && previous[len(previous)-1-suffix] == value[len(value)-1-suffix] {
		suffix++
	}
	for suffix > 0 && !utf8.RuneStart(previous[len(previous)-suffix]) {
		suffix--
	}
	// Compute the prefix's rune index using only its distance from the gap.
	start := b.gapRune
	if prefix < b.lo {
		start -= utf8.RuneCount(b.data[prefix:b.lo])
	} else if prefix > b.lo {
		start += utf8.RuneCount(b.data[b.hi : b.hi+prefix-b.lo])
	}
	end := start + utf8.RuneCountInString(previous[prefix:len(previous)-suffix])
	b.moveRune(start)
	b.hi += len(previous) - prefix - suffix
	insert := value[prefix : len(value)-suffix]
	count := utf8.RuneCountInString(insert)
	b.runes += count - (end - start)
	b.reserve(len(insert))
	copy(b.data[b.lo:], insert)
	b.lo += len(insert)
	b.gapRune += count
	b.compact()
}
