package eui

import (
	"image/color"
	"runtime"
	"time"
	"weak"

	"github.com/Distortions81/EUI/renderpool"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/time/rate"
)

// Context owns one independent UI. It must not be copied.
type Context struct {
	self                           *Context
	disposed                       bool
	ownedItems                     map[weak.Pointer[itemData]]struct{}
	ownedWindows                   map[weak.Pointer[windowData]]struct{}
	inputEnabled                   bool
	isWasm                         bool
	ownershipVisit                 map[*itemData]uint8
	nextItemSweep, nextWindowSweep int
	updateDepth, drawDepth         int
	disposePending                 bool
	configuredClipboard            Clipboard
	contextMenus                   []*itemData
	faceCache                      map[float64]*text.GoTextFace
	boldFaceCache                  map[float64]*text.GoTextFace
	screenWidth                    int
	screenHeight                   int
	mplusFaceSource                *text.GoTextFaceSource
	mplusBoldFaceSource            *text.GoTextFaceSource
	windows                        []*windowData
	activeWindow                   *windowData
	focusedItem                    *itemData
	hoveredItem                    *itemData
	uiScale                        float32
	userUIScale                    float32
	currentTheme                   *Theme
	currentThemeName               string
	// DebugMode draws geometry diagnostics.
	DebugMode bool
	// DumpMode saves cached images before the application exits.
	DumpMode bool
	// TreeMode saves the window hierarchy before exit.
	TreeMode bool
	// CacheCheck displays render counts for windows and controls.
	CacheCheck      bool
	windowSnapping  bool
	middleClickMove bool
	whiteImage      *ebiten.Image
	// AutoHiDPI follows the native monitor scale; enabled by default.
	AutoHiDPI       bool
	lastDeviceScale float64
	lastScaleCheck  time.Time
	// WindowStateChanged runs when a window opens or closes.
	WindowStateChanged    func()
	mposOld               point
	cursorShape           ebiten.CursorShapeType
	dragPart              dragType
	dragWin               *windowData
	dragFlow              *itemData
	activeItem            *itemData
	downWin               *windowData
	pointerPressHandled   bool
	activeSearch          *windowData
	selectedTextItem      *itemData
	updateNow             time.Time
	inputWindowOrder      []*windowData
	keyboardInputCaptured bool
	ShiftPressed          bool
	CtrlPressed           bool
	// CapsLockToggleHandler receives unclaimed Caps Lock presses.
	CapsLockToggleHandler func()
	inputSource           InputSource
	sampledInput          inputState
	nativeKeys            []ebiten.Key
	nativeButtons         []ebiten.MouseButton
	nativeTouchIDs        []ebiten.TouchID
	nativeTouches         []TouchPoint
	nativeChars           []rune
	windowFocusSerial     uint64
	focusVisible          bool
	keyboardSlider        *itemData
	keyboardSliderStart   float32
	wheelLimiter          *rate.Limiter
	// UnmanagedImageCreated receives optional allocation diagnostics.
	UnmanagedImageCreated func(width int, height int, elapsed time.Duration, site string)
	potatoMode            bool
	windowShadows         bool
	uiRenderTargets       renderpool.Pool
	uiPrimitives          primitiveCache
	dumpDone              bool
	renderNow             time.Time
	dropdownReuse         []openDropdown
	// RepaintObserver receives this instance's repaint diagnostics.
	RepaintObserver  func(RepaintStats)
	shadowMasks      map[shadowMaskKey]*ebiten.Image
	currentStyle     *StyleTheme
	currentStyleName string
	plainUIText      uiTextCache
	composingItem    *itemData
	nativeTextInput  struct {
		owner         *itemData
		composer      textinput.Composer
		context       imeContext
		geometry      inputGeometry
		pumping       bool
		cancelPending bool
	}
	themeAccentRefs struct {
		WindowActive   bool
		ButtonClick    bool
		TextClick      bool
		CheckboxClick  bool
		RadioClick     bool
		InputClick     bool
		SliderClick    bool
		SliderFilled   bool
		DropdownClick  bool
		DropdownSelect bool
		ProgressSelect bool
		TabClick       bool
	}
	namedColors        map[string]Color
	accentHue          float64
	accentSaturation   float64
	accentValue        float64
	accentAlpha        float64
	tileDividers       []TileDivider
	hoveredTileDivider int
	activeTileDivider  int
	strokeLineFn       func(dst *ebiten.Image, x0 float32, y0 float32, x1 float32, y1 float32, strokeWidth float32, clr color.Color, antialias bool)
	strokeRectFn       func(dst *ebiten.Image, x float32, y float32, width float32, height float32, strokeWidth float32, clr color.Color, antialias bool)
	themeModTime       time.Time
	styleModTime       time.Time
	modCheckTime       time.Time
	themePath          string
	stylePath          string
	themeDirectory     string
	// AutoReload watches this context's editable theme/style paths.
	AutoReload bool
}

// NewContext creates an independent UI with embedded fonts and default
// appearance. Use it on the game goroutine and call Dispose when finished.
func NewContext() (*Context, error) {
	ctx := &Context{}
	ctx.self = ctx
	ctx.inputEnabled = true
	ctx.isWasm = runtime.GOOS == "js" && runtime.GOARCH == "wasm"
	ctx.faceCache = map[float64]*text.GoTextFace{}
	ctx.boldFaceCache = map[float64]*text.GoTextFace{}
	ctx.screenWidth = 1024
	ctx.screenHeight = 1024
	ctx.uiScale = 1.0
	ctx.userUIScale = 1.0
	ctx.currentThemeName = "AccentDark"
	ctx.windowSnapping = true
	ctx.AutoHiDPI = true
	ctx.lastDeviceScale = 1.0
	ctx.wheelLimiter = rate.NewLimiter(rate.Every(125*time.Millisecond), 1)
	ctx.whiteImage = ctx.newManagedImage(3, 3)
	ctx.windowShadows = true
	ctx.uiRenderTargets = renderpool.Pool{MaxFreeBytes: 64 << 20, NewImage: func(w, h int, unmanaged bool) *ebiten.Image {
		if unmanaged {
			return ctx.newUnmanagedImage(w, h)
		}
		return ctx.newManagedImage(w, h)
	}}
	ctx.shadowMasks = map[shadowMaskKey]*ebiten.Image{}
	ctx.currentStyle = defaultStyle
	ctx.currentStyleName = "Default"
	ctx.accentSaturation = 1
	ctx.accentAlpha = 1
	ctx.hoveredTileDivider = -1
	ctx.activeTileDivider = -1
	ctx.strokeLineFn = vector.StrokeLine
	ctx.strokeRectFn = vector.StrokeRect
	ctx.themeDirectory = "themes"
	ctx.uiPrimitives.ctx = ctx
	ctx.sampledInput.ctx = ctx
	ctx.initializeWhiteImage()
	if err := ctx.LoadTheme(ctx.currentThemeName); err != nil {
		ctx.Dispose()
		return nil, err
	}
	if err := ctx.LoadStyle(ctx.currentStyleName); err != nil {
		ctx.Dispose()
		return nil, err
	}
	ctx.initializeWatchers()
	if err := ctx.initFonts(); err != nil {
		ctx.Dispose()
		return nil, err
	}
	return ctx, nil
}
