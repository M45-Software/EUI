package eui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Distortions81/EUI/internal/inputkeys"
	"github.com/hajimehoshi/ebiten/v2"
)

func BenchmarkDocumentEdits(b *testing.B) {
	ctx := newTestContext(b)
	for _, count := range []int{1000, 10000} {
		for _, name := range []string{"MiddleTyping", "MiddleWrappedTyping", "Paste500Lines", "KeyboardBackspace"} {
			b.Run(fmt.Sprintf("%d/%s", count, name), func(b *testing.B) {
				_, item := ctx.benchmarkEditor(b, count)
				ctx.Focus(item)
				if name == "MiddleWrappedTyping" {
					item.SetWordWrap(true)
					item.textDrawSize = Point{X: 320, Y: 600}
					item.editLayout()
				}
				item.editMove(item.editor().runeCount/2, false)
				insert := "!"
				if name == "Paste500Lines" {
					insert = strings.Repeat("\t日本 e\u0301 👩‍💻 pasted text\n", 500)
				}
				b.ReportAllocs()
				for b.Loop() {
					if name == "KeyboardBackspace" {
						item.editKey(ebiten.KeyBackspace, inputkeys.Modifiers{}, false)
					} else {
						item.editInsert(insert, "")
					}
					item.editLayout()
					item.Undo()
					item.editLayout()
				}
			})
		}
		b.Run(fmt.Sprintf("%d/WarmCompositionLayout", count), func(b *testing.B) {
			_, item := ctx.benchmarkEditor(b, count)
			ctx.Focus(item)
			item.editMove(item.editor().runeCount/2, false)
			item.setComposition("日本", 0, 2)
			item.editLayout()
			b.ReportAllocs()
			for b.Loop() {
				item.editLayout()
			}
		})
	}
}
