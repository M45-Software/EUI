package eui

import (
	"testing"
	"time"
)

func TestUserUIScaleIncludesDeviceScale(t *testing.T) {
	ctx := newTestContext(t)

	originalScale := ctx.uiScale
	originalUserScale := ctx.userUIScale
	originalDeviceScale := ctx.lastDeviceScale
	t.Cleanup(func() {
		ctx.userUIScale = originalUserScale
		ctx.lastDeviceScale = originalDeviceScale
		ctx.SetUIScale(originalScale)
	})

	ctx.lastDeviceScale = 2
	ctx.SetUserUIScale(1)
	if got := ctx.UIScale(); got != 2 {
		t.Fatalf("Retina effective UI scale = %v, want 2", got)
	}
	if got := ctx.UserUIScale(); got != 1 {
		t.Fatalf("Retina user UI scale = %v, want 1", got)
	}

	ctx.SetUserUIScale(1.25)
	if got := ctx.UIScale(); got != 2.5 {
		t.Fatalf("Retina effective UI scale after user adjustment = %v, want 2.5", got)
	}

	ctx.SetUserUIScale(4)
	if got := ctx.UIScale(); got != 8 {
		t.Fatalf("Retina effective UI scale at maximum user preference = %v, want 8", got)
	}
}

func TestDeviceScaleCheckIsThrottled(t *testing.T) {
	ctx := newTestContext(t)

	originalLastCheck := ctx.lastScaleCheck
	t.Cleanup(func() { ctx.lastScaleCheck = originalLastCheck })

	checkedAt := time.Now()
	ctx.lastScaleCheck = checkedAt

	if ctx.deviceScaleCheckDue(checkedAt.Add(scaleCheckInterval - time.Nanosecond)) {
		t.Fatal("device scale check became due before the interval elapsed")
	}
	if !ctx.deviceScaleCheckDue(checkedAt.Add(scaleCheckInterval)) {
		t.Fatal("device scale check not due after the interval elapsed")
	}
}
