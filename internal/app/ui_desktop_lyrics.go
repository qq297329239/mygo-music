package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
)

// The desktop lyrics: a bar of the song's words above the taskbar, frameless,
// transparent, on top of everything and passed through by the pointer. It
// shows the app's own lyrics, a line at a time as they are sung.

const (
	desktopLyricsW = 920 // the bar's width, shrunk on a narrow display
	desktopLyricsH = 76
)

// desktopLyrics is that bar. Its window is hidden until its setting and the
// lyrics both ask for it, as sync sees on the main thread.
type desktopLyrics struct {
	a     *app
	win   *mygo.Window // nil in tests, which drive shown themselves
	shown bool         // whether the bar shows, as it last asked
}

// newDesktopLyrics makes the bar, hidden until lyrics come.
func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a}
	x, y, w, h := d.place()
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
	mygo.Screen.OnDisplaysChanged(func() {
		x, y, w, h := d.place()
		d.win.SetSize(w, h)
		d.win.SetPosition(x, y)
	})
	return d
}

// place centers the bar at the foot of the primary display's work area, above
// the taskbar or Dock, a little off it.
func (d *desktopLyrics) place() (x, y, w, h int) {
	wa := mygo.Screen.PrimaryDisplay().WorkArea
	w = min(desktopLyricsW, wa.Width-32)
	h = desktopLyricsH
	x = wa.X + (wa.Width-w)/2
	y = wa.Y + wa.Height - h - 6
	return
}

// sync shows or hides the bar as its setting and the current lyrics ask: on
// with a line to show, off without. It runs on the main thread as the player
// or the settings change, and returns whether the bar shows.
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
		} else {
			d.win.Hide()
		}
	}
	return want
}

// lyricRows picks what the bar shows of the line at cur: the line, and under
// it its translation, or else the line coming next. Before the first line
// there is nothing to show yet.
func lyricRows(lines []lyrics.Line, cur int) (main, sub string, translated bool) {
	if cur < 0 || cur >= len(lines) {
		return "", "", false
	}
	main = lines[cur].Text
	if lines[cur].Translation != "" {
		return main, lines[cur].Translation, true
	}
	if cur+1 < len(lines) {
		return main, lines[cur+1].Text, false
	}
	return main, "", false
}

// view draws the bar: the line being sung, its translation or the next line
// under it, both shadowed to read over anything. A new line floats in.
func (d *desktopLyrics) view(c *ui.Context) {
	c.Root().Background(ui.Transparent)
	if !d.shown {
		return
	}
	pl := d.a.pl
	lines, _ := pl.Lyrics()
	pos := pl.Position()
	cur := lyrics.Index(lines, pos)
	if pl.Playing() && len(lines) > 0 {
		// Ask for the frame where the next line starts, a breath after it.
		next := lines[0].At // still in the intro
		if cur >= 0 {
			if cur+1 >= len(lines) {
				next = -1 // the last line sings on
			} else {
				next = lines[cur+1].At
			}
		}
		if next >= 0 && next > pos {
			c.After(next - pos + 20*time.Millisecond)
		}
	}
	if cur < 0 {
		return // nothing sung yet
	}
	main, sub, translated := lyricRows(lines, cur)
	lang := lyrics.Lang(lines)
	subLang := c.Locale() // a translation is in the listener's language
	if !translated {
		subLang = lang
	}
	// The roll springs from the line before to this one: as it settles, in
	// goes 1 (just changed) to 0 (in place).
	roll := ui.Box(c.Key("roll")).Fill().AnimateTo("at", float32(cur),
		ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}})
	in := clamp01(float32(cur) - roll)
	ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Center().
		AlignItems(ui.Center).Gap(6).PassThrough().Children(func() {
		ui.Text(c.Key("line"), main).Lang(lang).
			FontSize(22).FontWeight(700).
			TextColor(d.a.pal.accent.Alpha(1 - 0.55*in)).
			TextShadow(0, 1, 6, ui.RGBA(0, 0, 0, 0.45)).
			TextAlign(ui.Center).SingleLine().Shrink(1).MinWidth(0).
			Translate(0, in*26)
		if sub != "" {
			ui.Text(c.Key("sub"), sub).Lang(subLang).
				FontSize(13).
				TextColor(white.Alpha(0.55 * (1 - 0.45*in))).
				TextShadow(0, 1, 4, ui.RGBA(0, 0, 0, 0.4)).
				TextAlign(ui.Center).SingleLine().Shrink(1).MinWidth(0).
				Translate(0, in*26)
		}
	})
}
