package eui

import (
	"image"
	"math"

	"github.com/Distortions81/EUI/renderpool"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

const maxUITextBytes = 8 << 20
const maxUITextEntries = 256

// Cache plain text independently of its parent window. Chat/status updates can
// repaint a whole pane while most of its lines are unchanged. Client faces
// vary by source (regular/bold/italic) and size; separately constructed
// Chat/Console faces can share entries. Fractional placement and tint are part
// of the key so overlapping/translucent glyphs retain their original coverage.
type uiTextKey struct {
	value          string
	source         *text.GoTextFaceSource
	composedFace   text.Face
	size           float64
	layout         text.LayoutOptions
	phaseX, phaseY float64
	tint           ebiten.ColorScale
}
type uiTextImage struct {
	image  *ebiten.Image
	bounds image.Rectangle
	used   uint64
	bytes  int
}
type uiTextCache struct {
	targets      renderpool.Pool
	entries      map[uiTextKey]uiTextImage
	bytes        int
	clock        uint64
	hits, misses uint64
}

func (cache *uiTextCache) clear() {
	for _, entry := range cache.entries {
		cache.targets.Release(entry.image)
	}
	cache.targets.Clear()
	*cache = uiTextCache{}
}

// Only called with translation-only, nearest-filtered
// DrawOptions. Store actual glyph bounds, including overhangs and diacritics.
func (ctx *Context) drawCachedUIText(dst *ebiten.Image, value string, face text.Face, op *text.DrawOptions) {
	if value == "" {
		return
	}
	plain, ok := face.(*text.GoTextFace)
	if len(value) > 4096 {
		text.Draw(dst, value, face, op)
		return
	}
	x, y := op.GeoM.Apply(0, 0)
	ix, iy := math.Floor(x), math.Floor(y)
	key := uiTextKey{value: value, layout: op.LayoutOptions,
		phaseX: x - ix, phaseY: y - iy, tint: op.ColorScale}
	if ok {
		key.source, key.size = plain.Source, plain.Size
	} else {
		key.composedFace = face
	}
	cache := &ctx.plainUIText
	cache.clock++
	entry, cached := cache.entries[key]
	if cached {
		cache.hits++
	} else {
		cache.misses++
		glyphs := text.AppendGlyphs(nil, value, face, &op.LayoutOptions)
		var bounds image.Rectangle
		for _, glyph := range glyphs {
			if glyph.Image == nil {
				continue
			}
			w, h := glyph.Image.Bounds().Dx(), glyph.Image.Bounds().Dy()
			gx, gy := glyph.X+key.phaseX, glyph.Y+key.phaseY
			bounds = bounds.Union(image.Rect(int(math.Floor(gx)), int(math.Floor(gy)), int(math.Ceil(gx))+w, int(math.Ceil(gy))+h))
		}
		if bounds.Empty() {
			return
		}
		// Large documents keep the clipped direct path and cannot exhaust VRAM.
		if bounds.Dx() > 1024 || bounds.Dy() > 256 {
			// Reuse the layout already computed above instead of shaping twice.
			drawOp := op.DrawImageOptions
			for _, glyph := range glyphs {
				if glyph.Image == nil {
					continue
				}
				setGlyphColorScale(&drawOp, op.ColorScale, glyph.Colored)
				drawOp.GeoM.Reset()
				drawOp.GeoM.Translate(glyph.X, glyph.Y)
				drawOp.GeoM.Concat(op.GeoM)
				dst.DrawImage(glyph.Image, &drawOp)
			}
			return
		}
		if cache.targets.NewImage == nil {
			cache.targets = renderpool.Pool{MaxFreeBytes: 1 << 20, NewImage: func(w, h int, unmanaged bool) *ebiten.Image {
				if unmanaged {
					return ctx.newUnmanagedImage(w, h)
				}
				return ctx.newManagedImage(w, h)
			}}
		}
		// Account for the backing allocation, including size-class padding. Keep
		// a separate, small idle reserve so scrolling can reuse evicted textures.
		bytes := int(renderpool.BytesForSize(bounds.Dx(), bounds.Dy(), ctx.potatoMode))
		for len(cache.entries) >= maxUITextEntries || cache.bytes+bytes > maxUITextBytes {
			cache.evictOldest()
		}
		img := cache.targets.Acquire(bounds.Dx(), bounds.Dy(), ctx.potatoMode)
		bytes = int(cache.targets.Bytes(img))
		// A reusable allocation may be larger than the requested size class.
		for cache.bytes+bytes > maxUITextBytes {
			cache.evictOldest()
		}
		entry = uiTextImage{image: img, bounds: bounds, bytes: bytes}
		drawOp := op.DrawImageOptions
		for _, glyph := range glyphs {
			if glyph.Image == nil {
				continue
			}
			setGlyphColorScale(&drawOp, op.ColorScale, glyph.Colored)
			drawOp.GeoM.Reset()
			drawOp.GeoM.Translate(glyph.X+key.phaseX-float64(bounds.Min.X), glyph.Y+key.phaseY-float64(bounds.Min.Y))
			entry.image.DrawImage(glyph.Image, &drawOp)
		}
		if cache.entries == nil {
			cache.entries = make(map[uiTextKey]uiTextImage)
		}
		cache.bytes += bytes
	}
	entry.used = cache.clock
	cache.entries[key] = entry
	drawOp := ebiten.DrawImageOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true}
	drawOp.GeoM.Translate(ix+float64(entry.bounds.Min.X), iy+float64(entry.bounds.Min.Y))
	dst.DrawImage(entry.image, &drawOp)
}

func setGlyphColorScale(op *ebiten.DrawImageOptions, tint ebiten.ColorScale, colored bool) {
	op.ColorScale = tint
	if colored {
		op.ColorScale.Reset()
		op.ColorScale.ScaleAlpha(tint.A())
	}
}

func (cache *uiTextCache) evictOldest() {
	var oldestKey uiTextKey
	oldest := cache.clock
	for key, candidate := range cache.entries {
		if candidate.used < oldest {
			oldestKey, oldest = key, candidate.used
		}
	}
	victim := cache.entries[oldestKey]
	cache.targets.Release(victim.image)
	cache.bytes -= victim.bytes
	delete(cache.entries, oldestKey)
}
