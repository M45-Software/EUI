package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	eui "github.com/M45-Software/EUI"
)

// Each opening starts with the scene's current values. The application model
// changes only after the whole draft validates successfully.
func (s *studio) openSceneSettings(speedSlider, countPicker *eui.ItemData) *eui.WindowData {
	name := s.ui.NewFormField("name", "Scene name", s.sceneName, eui.Required)
	speed := s.ui.NewFormField("speed", "Orbit speed", fmt.Sprintf("%g", s.speed), eui.Required, eui.NumberRange(0, 2))
	count := s.ui.NewFormField("count", "Particles", strconv.Itoa(s.count), eui.Required, eui.IntegerRange(1, 200))
	form := s.ui.NewForm(name, speed, count)
	form.View().Size.X = max(220, min(340, float32(s.layoutWidth)-72))
	form.View().MaxSize.X = form.View().Size.X
	win := s.ui.NewWindow()
	win.Title = "Scene settings"
	win.AutoSize, win.Movable, win.Closable = true, true, true
	win.Padding = 12
	win.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)
	apply := s.ui.NewActionButton("Apply", func() {
		if form.Apply() != nil {
			return
		}
		values := form.Values()
		s.sceneName = values["name"]
		n, _ := strconv.ParseFloat(strings.TrimSpace(values["speed"]), 32)
		s.speed = float32(n)
		s.count, _ = strconv.Atoi(strings.TrimSpace(values["count"]))
		s.nameInput.UpdateText(s.sceneName)
		speedSlider.SetValue(s.speed)
		caption := fmt.Sprintf("%d particles", s.count)
		options := slices.Clone(countPicker.Options)
		index := slices.Index(options, caption)
		if index < 0 {
			options = append(options, caption)
			countPicker.SetOptions(options)
			index = len(options) - 1
		}
		countPicker.SetSelected(index)
		s.log("Scene settings applied")
		win.Close()
	})
	cancel := s.ui.NewActionButton("Cancel", win.Close)
	win.DefaultButton, win.CancelButton = apply, cancel
	win.OnClose = func() {
		form.Cancel()
		win.RemoveWindow()
	}
	buttons := s.ui.NewRow(cancel, apply)
	buttons.Gap = 8
	root := s.ui.NewColumn(form.View(), buttons)
	root.Gap = 16
	win.AddItem(root)
	win.MarkOpenModal()
	return win
}
