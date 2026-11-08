package eui

import "github.com/hajimehoshi/ebiten/v2/text/v2"

func (ctx *Context) textFace(size float32) *text.GoTextFace {
	if ctx.mplusFaceSource == nil {
		return &text.GoTextFace{Size: float64(size)}
	}
	s := float64(size)
	if f, ok := ctx.faceCache[s]; ok {
		return f
	}
	f := &text.GoTextFace{Source: ctx.mplusFaceSource, Size: s}
	ctx.faceCache[s] = f
	return f
}

func (ctx *Context) boldFace(size float32) *text.GoTextFace {
	if ctx.mplusBoldFaceSource == nil {
		return ctx.textFace(size)
	}
	s := float64(size)
	if f, ok := ctx.boldFaceCache[s]; ok {
		return f
	}
	f := &text.GoTextFace{Source: ctx.mplusBoldFaceSource, Size: s}
	ctx.boldFaceCache[s] = f
	return f
}
