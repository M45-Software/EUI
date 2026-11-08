package eui

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

func (ctx *Context) initializeWatchers() {
	ctx.modCheckTime = time.Now()
	ctx.updateThemePath()
	ctx.updateStylePath()
	ctx.refreshThemeMod()
	ctx.refreshStyleMod()
}

func (ctx *Context) updateThemePath() {
	ctx.themePath = filepath.Join(ctx.themeDirectory, "palettes", ctx.currentThemeName+".json")
}

func (ctx *Context) refreshThemeMod() {
	if info, err := os.Stat(ctx.themePath); err == nil {
		ctx.themeModTime = info.ModTime()
	} else {
		ctx.themeModTime = time.Time{}
	}
}

func (ctx *Context) updateStylePath() {
	ctx.stylePath = filepath.Join(ctx.themeDirectory, "styles", ctx.currentStyleName+".json")
}

func (ctx *Context) refreshStyleMod() {
	if info, err := os.Stat(ctx.stylePath); err == nil {
		ctx.styleModTime = info.ModTime()
	} else {
		ctx.styleModTime = time.Time{}
	}
}

func (ctx *Context) checkThemeStyleMods() {
	if !ctx.AutoReload {
		return
	}
	if time.Since(ctx.modCheckTime) < 500*time.Millisecond {
		return
	}
	ctx.modCheckTime = time.Now()
	if info, err := os.Stat(ctx.themePath); err == nil {
		if info.ModTime().After(ctx.themeModTime) {
			log.Println("Palette reload")
			if err := ctx.LoadTheme(ctx.currentThemeName); err != nil {
				log.Printf("Auto reload theme error: %v", err)
			}
			ctx.themeModTime = info.ModTime()
		}
	} else {
		log.Println("Unable to stat " + ctx.currentThemeName + ": " + err.Error())
	}

	if info, err := os.Stat(ctx.stylePath); err == nil {
		if info.ModTime().After(ctx.styleModTime) {
			log.Println("Style theme reload")
			if err := ctx.LoadStyle(ctx.currentStyleName); err != nil {
				log.Printf("Auto reload style error: %v", err)
			}
			ctx.styleModTime = info.ModTime()
		}
	} else {
		log.Println("Unable to stat " + ctx.currentStyleName + ": " + err.Error())
	}

}
