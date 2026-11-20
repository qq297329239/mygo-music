package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
)

// The desktop lyrics: a bar of the song's words above the taskbar, frameless,
// transparent, on top of everything and passed through by the pointer. It
// shows the app's own lyrics, a line at a time as they are sung, each word
// lighting from the color to come to the color sung. The listener sets its
// font, its size and its colors, has it sit on the taskbar itself, and
// unlocks it to move and resize it by hand.

// desktopLyricsW is the floating bar's width, shrunk on a narrow display.
const desktopLyricsW = 920

// desktopLyrics is that bar. Its window is hidden until its setting and the
// lyrics both ask for it, as sync sees on the main thread.
type desktopLyrics struct {
	a         *app
	win       *mygo.Window // nil in tests, which drive shown themselves
	shown     bool         // whether the bar shows, as it last asked
	w, h      int          // the window's size as last placed, 0 before that
	applying  bool         // placing it itself, not by the listener's hand
	saveTimer *time.Timer  // debounces keeping the bar's place as it's dragged
}

// newDesktopLyrics makes the bar, hidden until lyrics come.
func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a}
	x, y, w, h := d.place()
	d.w, d.h = w, h
	d.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         Tf("%s Desktop Lyrics", mygo.App.Name()),
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		DisableShadow: true,
		DisableResize: true,
		AlwaysOnTop:   true,
		SkipTaskbar:   true,
		X:             x,
		Y:             y,
		Width:         w,
		Height:        h,
		Content:       ui.View(d.view),
	})
	// Closed, it stays until the app quits: the setting and the lyrics decide
	// when it shows, in sync.
	d.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
		}
	})
	d.win.SetIgnoreMouseEvents(true) // nothing to click: the pointer passes through
	d.applyMode()
	// Moved or resized by hand, the bar keeps where it is left for the next
	// run, once the listener stops dragging it.
	d.win.OnResize(d.movedPlace)
	d.win.OnMove(d.movedPlace)
	mygo.Screen.OnDisplaysChanged(func() {
		d.applySettings()
	})
	return d
}

// place says where the bar sits: on the taskbar itself when asked, where the
// listener left it, else floating at the foot of the work area.
func (d *desktopLyrics) place() (x, y, w, h int) {
	s := &d.a.settings
	disp := mygo.Screen.PrimaryDisplay()
	if s.DesktopLyricsTaskbar {
		if tx, ty, tw, th, ok := taskbarRect(disp.Bounds, disp.WorkArea); ok {
			return tx, ty, tw, th
		}
		// A taskbar beside the work, above it, or hidden until the pointer
		// asks leaves no strip to sit on; float as ever instead.
	}
	wa := disp.WorkArea
	if !s.DesktopLyricsTaskbar && s.DesktopLyricsW > 0 && s.DesktopLyricsH > 0 {
		w, h = s.DesktopLyricsW, s.DesktopLyricsH
		x = min(max(s.DesktopLyricsX, wa.X), wa.X+wa.Width-w)
		y = min(max(s.DesktopLyricsY, wa.Y), wa.Y+wa.Height-h)
		return
	}
	w = min(desktopLyricsW, wa.Width-32)
	h = barHeight(s.DesktopLyricsSize, s.DesktopLyricsSingle || s.DesktopLyricsTaskbar)
	x = wa.X + (wa.Width-w)/2
	y = wa.Y + wa.Height - h - 6
	return
}

// taskbarRect is the strip the taskbar sits on: below the work area, wide as
// the display and tall as the taskbar. It answers ok false with no strip to
// sit on, as when the taskbar is beside the work or above it.
func taskbarRect(bounds, wa mygo.Rectangle) (x, y, w, h int, ok bool) {
	x, y = wa.X, wa.Y+wa.Height
	w, h = wa.Width, bounds.Y+bounds.Height-y
	if h < 28 || wa.Width <= 0 {
		return 0, 0, 0, 0, false
	}
	return x, y, w, h, true
}

// barHeight is how tall the floating bar stands at a font size: room for the
// line sung, and for the second line under it unless it shows one.
func barHeight(size int, single bool) int {
	size = max(size, 12)
	h := size*3/2 + max(10, size*6/10)*3/2 + 22
	if single {
		h = size*3/2 + 26
	}
	return min(max(h, 40), 160)
}

// applySettings puts the window as its settings ask, as they or the displays
// change: where it sits, how big, and whether it takes the pointer.
func (d *desktopLyrics) applySettings() {
	if d.win == nil {
		return
	}
	d.applying = true // its own moves are not the listener's
	defer func() { d.applying = false }()
	x, y, w, h := d.place()
	d.w, d.h = w, h
	d.win.SetSize(w, h)
	d.win.SetPosition(x, y)
	d.applyMode()
}

// applyMode takes the pointer while the bar floats unlocked, to move it by
// its face and resize it by its edges; locked, the pointer passes through as
// ever.
func (d *desktopLyrics) applyMode() {
	if d.win == nil {
		return
	}
	free := d.a.settings.DesktopLyricsUnlocked && !d.a.settings.DesktopLyricsTaskbar
	d.win.SetIgnoreMouseEvents(!free)
	d.win.SetResizable(free)
	if free {
		d.win.SetMinimumSize(360, 40)
	}
}

// movedPlace keeps where the listener leaves the bar, saved once they stop
// dragging it.
func (d *desktopLyrics) movedPlace() {
	s := &d.a.settings
	if d.applying || s.DesktopLyricsTaskbar || !s.DesktopLyricsUnlocked {
		return
	}
	if d.saveTimer != nil {
		d.saveTimer.Stop()
	}
	d.saveTimer = time.AfterFunc(500*time.Millisecond, func() {
		mygo.RunOnMain(d.savePlace)
	})
}

// savePlace writes the bar's place to the settings, there for the next run.
func (d *desktopLyrics) savePlace() {
	if d.win == nil {
		return
	}
	s := &d.a.settings
	s.DesktopLyricsX, s.DesktopLyricsY = d.win.Position()
	s.DesktopLyricsW, s.DesktopLyricsH = d.win.Size()
	s.save()
}

// sync shows the bar when its setting and the lyrics both ask for it, and
// hides it when either goes. It says whether the bar shows now.
func (d *desktopLyrics) sync() bool {
	lines, ok := d.a.pl.Lyrics()
	want := d.a.settings.DesktopLyrics && ok && len(lines) > 0
	if want == d.shown {
		return want
	}
	d.shown = want
	if d.win != nil {
		if want {
			d.win.ShowInactive() // without taking the focus
			// Again: the taskbar is on top too, and may have come above.
			d.win.SetAlwaysOnTop(true)
		} else {
			d.win.Hide()
		}
	}
	return want
}

// playedColor is what the sung words draw in: the listener's pick, or the
// theme's accent.
func (d *desktopLyrics) playedColor() ui.Color {
	if v, ok := parseHexColor(d.a.settings.DesktopLyricsPlayed); ok {
		return v
	}
	return d.a.pal.accent
}

// pendingColor is what the words to come draw in: the listener's pick, or a
// translucent white.
func (d *desktopLyrics) pendingColor() ui.Color {
	if v, ok := parseHexColor(d.a.settings.DesktopLyricsPending); ok {
		return v
	}
	return white.Alpha(0.55)
}

// bgColor is what shows behind the words when asked: the listener's pick
// with its opacity, or a translucent black.
func (d *desktopLyrics) bgColor() ui.Color {
	v := ui.RGB(0, 0, 0)
	if got, ok := parseHexColor(d.a.settings.DesktopLyricsBgColor); ok {
		got.A = 255 // the opacity says how much shows through, not the pick
		v = got
	}
	op := d.a.settings.DesktopLyricsOpacity
	if op == 0 {
		op = 0.6
	}
	return v.Alpha(float32(op))
}

// parseHexColor reads a color written "#rgb", "#rgba", "#rrggbb" or
// "#rrggbbaa", saying whether it is one. Settings keep their colors as such
// strings, which the listener may edit by hand.
func parseHexColor(s string) (ui.Color, bool) {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(h) != 3 && len(h) != 4 && len(h) != 6 && len(h) != 8 {
		return ui.Color{}, false
	}
	for i := 0; i < len(h); i++ {
		c := h[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return ui.Color{}, false
		}
	}
	return ui.Hex("#" + h), true
}

// colorHex writes a color for the settings to keep.
func colorHex(v ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x%02x", v.R, v.G, v.B, v.A)
}

// wordColor is a word's color at pos: played once sung, pending before, and
// the mix across it as it sings.
func wordColor(played, pending ui.Color, pos, start, end time.Duration) ui.Color {
	switch {
	case pos >= end:
		return played
	case pos <= start:
		return pending
	}
	lit := clamp01(float32(pos-start) / float32(max(end-start, 50*time.Millisecond)))
	return pending.Mix(played, lit)
}

// lyricRows picks the bar's rows at a line: the line itself, and under it
// its translation, or else the line coming next. Nothing sings before the
// first line.
func lyricRows(lines []lyrics.Line, cur int) (main, sub string, translated bool) {
	if cur < 0 || cur >= len(lines) {
		return
	}
	main = lines[cur].Text
	if lines[cur].Translation != "" {
		return main, lines[cur].Translation, true
	}
	if cur+1 < len(lines) {
		sub = lines[cur+1].Text
	}
	return
}

// view draws the bar: the line being sung word by word from the color to
// come to the color sung, its translation or the next line under it, both
// shadowed to read over anything. A new line floats in.
func (d *desktopLyrics) view(c *ui.Context) {
	s := &d.a.settings
	free := s.DesktopLyricsUnlocked && !s.DesktopLyricsTaskbar
	bg := ui.Transparent
	if s.DesktopLyricsBg {
		bg = d.bgColor()
	} else if free {
		bg = ui.RGBA(0, 0, 0, 0.2) // by hand: show the bar's edges to take
	}
	c.Root().Background(bg)
	if !d.shown {
		return
	}
	pl := d.a.pl
	lines, _ := pl.Lyrics()
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	if pl.Playing() && len(lines) > 0 {
		// The words light and the lines turn over as the song plays.
		c.After(40 * time.Millisecond)
	}
	if cur < 0 {
		return // nothing sung yet
	}
	main, sub, translated := lyricRows(lines, cur)
	if s.DesktopLyricsSingle || s.DesktopLyricsTaskbar {
		sub = "" // the bar on the taskbar has no room for two lines
	}
	lang := lyrics.Lang(lines)
	subLang := c.Locale() // a translation is in the listener's language
	if !translated {
		subLang = lang
	}
	size := float32(max(s.DesktopLyricsSize, 12))
	if s.DesktopLyricsTaskbar && d.h > 0 {
		size = max(12, min(size, float32(d.h)-14)) // fit the taskbar's strip
	}
	weight := s.DesktopLyricsWeight
	if weight == 0 {
		weight = 700
	}
	align, items := ui.Center, ui.Center
	if s.DesktopLyricsAlign == "left" {
		align, items = ui.Start, ui.Start
	}
	played, pending := d.playedColor(), d.pendingColor()
	// The roll springs from the line before to this one: as it settles, in
	// goes 1 (just changed) to 0 (in place).
	roll := ui.Box(c.Key("roll")).Fill().AnimateTo("at", float32(cur),
		ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}})
	in := clamp01(float32(cur) - roll)
	col := ui.Column(c.Key("col")).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().
		AlignItems(items).Gap(6)
	if free {
		col.DragWindow() // the unlocked bar moves by its face
	} else {
		col.PassThrough()
	}
	col.Children(func() {
		font, subSize := s.DesktopLyricsFont, max(float32(10), size*6/10)
		line := ui.RichText(c.Key("line")).Lang(lang).
			FontSize(size).FontWeight(weight).
			TextColor(pending).
			TextShadow(0, 1, 6, ui.RGBA(0, 0, 0, 0.45)).
			TextAlign(align).SingleLine().Shrink(1).MinWidth(0).
			Translate(0, in*26)
		if font != "" {
			line.Font(font)
		}
		line.Children(func() {
			if words := lines[cur].Words; len(words) > 0 {
				for wi, wd := range words {
					ui.Text(c.Key(wi), wd.Text).
						TextColor(wordColor(played, pending, pos, wd.Start, wd.End))
				}
			} else {
				ui.Text(c.Key("plain"), main).TextColor(pending)
			}
		})
		if sub != "" {
			subText := ui.Text(c.Key("sub"), sub).Lang(subLang).
				FontSize(subSize).
				TextColor(pending.Alpha(0.7 * (1 - 0.45 * in))).
				TextShadow(0, 1, 4, ui.RGBA(0, 0, 0, 0.4)).
				TextAlign(align).SingleLine().Shrink(1).MinWidth(0).
				Translate(0, in*26)
			if font != "" {
				subText.Font(font)
			}
		}
	})
}
