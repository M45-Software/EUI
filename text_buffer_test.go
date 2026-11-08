package eui

import (
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestEditBufferRandomizedAgainstRuneOracle(t *testing.T) {
	rng := rand.New(rand.NewPCG(47, 91))
	values := []string{"", "ascii", "é日本", "e\u0301 👩‍💻\n", "\t\n", strings.Repeat("abc日本\n", 500)}
	for _, initial := range values {
		buffer := newEditBuffer(initial)
		value := initial
		for i := 0; i < 800; i++ {
			runes := []rune(value)
			start, end := rng.IntN(len(runes)+1), rng.IntN(len(runes)+1)
			if start > end {
				start, end = end, start
			}
			insert := values[rng.IntN(len(values)-1)]
			want := string(runes[:start]) + insert + string(runes[end:])
			old := value
			value = buffer.replace(start, end, insert)
			if value != want || buffer.value() != want || buffer.runes != utf8.RuneCountInString(want) || buffer.gapRune != start+utf8.RuneCountInString(insert) {
				t.Fatalf("edit %d mismatch", i)
			}
			if rng.IntN(3) == 0 {
				buffer.restore(value, old)
				if buffer.value() != old || buffer.runes != len(runes) {
					t.Fatalf("undo %d mismatch", i)
				}
				buffer.restore(old, value)
				if buffer.value() != value || buffer.runes != utf8.RuneCountInString(value) {
					t.Fatalf("redo %d mismatch", i)
				}
			}
		}
	}
}
func TestEditBufferReplacementReleasesPeakStorage(t *testing.T) {
	buffer := newEditBuffer(strings.Repeat("日本", 1<<18))
	buffer.replace(0, buffer.runes, "é")
	if buffer.value() != "é" || len(buffer.data) > 4096 {
		t.Fatal("large deletion retained peak storage")
	}
	buffer.restore("é", "日本")
	if buffer.value() != "日本" || buffer.runes != 2 {
		t.Fatal("UTF-8 common-byte prefix/suffix split a rune")
	}
}

func TestEditBufferLocalEditsAndArbitraryHistoryRestore(t *testing.T) {
	rng := rand.New(rand.NewPCG(912, 14))
	value := strings.Repeat("café e\u0301 日本 👩‍💻\n", 4000)
	buffer := newEditBuffer(value)
	history := []string{value}
	for i := 0; i < 200; i++ {
		runes := []rune(value)
		start := rng.IntN(len(runes) + 1)
		end := min(len(runes), start+rng.IntN(8))
		insert := []string{"x", "\n日本\t", "👩‍💻e\u0301", ""}[rng.IntN(4)]
		want := string(runes[:start]) + insert + string(runes[end:])
		value = buffer.replace(start, end, insert)
		if value != want {
			t.Fatal("large local edit differs from oracle")
		}
		if i%7 == 0 {
			target := history[rng.IntN(len(history))]
			buffer.restore(value, target)
			value = target
			if buffer.value() != value || buffer.runes != utf8.RuneCountInString(value) {
				t.Fatal("restore across multiple edits differs from oracle")
			}
		}
		history = append(history, value)
	}
}

func TestEditBufferPublishedValuesRemainImmutable(t *testing.T) {
	buffer := newEditBuffer("abcdef")
	first := buffer.replace(6, 6, "X")
	second := buffer.replace(0, 1, "Z")
	buffer.replace(1, 6, "日本")
	if first != "abcdefX" || second != "ZbcdefX" {
		t.Fatal("editing mutated a previously published string")
	}
}
