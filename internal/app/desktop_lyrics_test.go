package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
)

// twoLines is a pair of lyric lines, the second with a translation.
func twoLines() []lyrics.Line {
	return []lyrics.Line{
		{At: 2 * time.Second, End: 4 * time.Second, Text: "first line"},
		{At: 4 * time.Second, End: 8 * time.Second, Text: "second line", Translation: "第二行"},
	}
}

// The bar shows only with lyrics and its setting on: a song without them, or
// the setting off, keeps it away.
func TestDesktopLyricsShowsWithLyrics(t *testing.T) {
	a := newTestApp()
	a.dlyr = &desktopLyrics{a: a}
	a.pl.Cue([]music.Track{song("7", "宝船", "DOUDOU")})

	out := audio.NewPlayer(&silence{})
	out.SetVolume(0)
	a.pl.Attach(out)
	defer out.Close()

	a.settings.DesktopLyrics = true
	if a.dlyr.sync() {
		t.Fatal("showed without lyrics")
	}
	a.pl.SetLyrics(twoLines())
	if !a.dlyr.sync() {
		t.Fatal("did not show with lyrics on and the setting on")
	}
	a.pl.SetLyrics(nil)
	if a.dlyr.sync() {
		t.Fatal("kept showing without lyrics")
	}
	a.pl.SetLyrics(twoLines())
	if !a.dlyr.sync() {
		t.Fatal("did not come back with lyrics")
	}
	a.settings.DesktopLyrics = false
	if a.dlyr.sync() {
		t.Fatal("showed with the setting off")
	}
}

// The bar picks the line at the position: the line itself over its
// translation, or else the line coming next, and nothing before the first.
func TestDesktopLyricsLineChoice(t *testing.T) {
	lines := twoLines()
	if main, _, _ := lyricRows(lines, -1); main != "" {
		t.Fatal("showed a line before the first")
	}
	main, sub, translated := lyricRows(lines, 0)
	if main != "first line" || sub != "second line" || translated {
		t.Fatalf("line 0: got %q / %q / %v", main, sub, translated)
	}
	main, sub, translated = lyricRows(lines, 1)
	if main != "second line" || sub != "第二行" || !translated {
		t.Fatalf("line 1: got %q / %q / %v", main, sub, translated)
	}
	if main, sub, _ := lyricRows(lines[:1], 0); main != "first line" || sub != "" {
		t.Fatalf("last line: got %q / %q", main, sub)
	}
}

// The bar follows the song: nothing sung yet before the first line, the line
// at the position once it is, with its translation under it.
func TestDesktopLyricsViewFollowsTheSong(t *testing.T) {
	a := newTestApp()
	a.dlyr = &desktopLyrics{a: a}
	a.settings.DesktopLyrics = true
	a.pl.Cue([]music.Track{song("7", "宝船", "DOUDOU")})
	a.pl.SetLyrics(twoLines())
	a.pl.Attach(audio.NewPlayer(&silence{}))
	a.dlyr.sync()

	tt := ui.NewTester(a.dlyr.view, desktopLyricsW, barHeight(a.settings.DesktopLyricsSize, false))
	a.pl.Seek(1 * time.Second)
	tt.Frame() // still in the intro
	if tt.HasText("first line") || tt.HasText("second line") {
		t.Fatalf("in the intro: %q", tt.Texts())
	}
	a.pl.Seek(3 * time.Second)
	tt.Frame() // the first line sings
	if !tt.HasText("first line") {
		t.Fatalf("at 3s: %q", tt.Texts())
	}
	a.pl.Seek(5 * time.Second)
	tt.Frame() // the second, with its translation under it
	if !tt.HasText("second line") || !tt.HasText("第二行") {
		t.Fatalf("at 5s: %q", tt.Texts())
	}
}

// A word sings from the color to come to the color sung: pending before it,
// played after, and half way between at half way through.
func TestDesktopLyricsWordColor(t *testing.T) {
	played, pending := ui.Hex("#ff0000"), ui.Hex("#00ff00")
	start, end := 2*time.Second, 4*time.Second
	if got := wordColor(played, pending, 1*time.Second, start, end); got != pending {
		t.Fatalf("before the word: got %v", got)
	}
	if got := wordColor(played, pending, 5*time.Second, start, end); got != played {
		t.Fatalf("after the word: got %v", got)
	}
	if got, want := wordColor(played, pending, 3*time.Second, start, end), pending.Mix(played, 0.5); got != want {
		t.Fatalf("half way: got %v want %v", got, want)
	}
}

// The taskbar's strip is the space below the work area, wide as the display,
// and nothing when the taskbar leaves none below it.
func TestDesktopLyricsTaskbarRect(t *testing.T) {
	bounds := mygo.Rectangle{Width: 1920, Height: 1080}
	x, y, w, h, ok := taskbarRect(bounds, mygo.Rectangle{Width: 1920, Height: 1032})
	if !ok || x != 0 || y != 1032 || w != 1920 || h != 48 {
		t.Fatalf("taskbar at the foot: got %d,%d %dx%d ok=%v", x, y, w, h, ok)
	}
	if _, _, _, _, ok := taskbarRect(bounds, mygo.Rectangle{X: 48, Width: 1872, Height: 1080}); ok {
		t.Fatal("taskbar beside the work: a strip below it")
	}
	if _, _, _, _, ok := taskbarRect(bounds, mygo.Rectangle{Y: 48, Width: 1920, Height: 1032}); ok {
		t.Fatal("taskbar above the work: a strip below it")
	}
	if _, _, _, _, ok := taskbarRect(bounds, mygo.Rectangle{Width: 1920, Height: 1075}); ok {
		t.Fatal("strip too short to sit on")
	}
}

// The bar stands taller for two lines than one and for bigger letters, and
// never shorter than 40 nor taller than 160.
func TestDesktopLyricsBarHeight(t *testing.T) {
	if barHeight(22, false) <= barHeight(22, true) {
		t.Fatal("two lines stand no taller than one")
	}
	if barHeight(48, false) <= barHeight(12, false) {
		t.Fatal("bigger letters stand no taller")
	}
	if h := barHeight(200, true); h < 40 || h > 160 {
		t.Fatalf("huge letters: got %d", h)
	}
	if h := barHeight(0, true); h < 40 {
		t.Fatalf("tiny letters: got %d", h)
	}
}

// The single-line bar shows the line being sung but not the one under it.
func TestDesktopLyricsSingleLine(t *testing.T) {
	a := newTestApp()
	a.dlyr = &desktopLyrics{a: a}
	a.settings.DesktopLyrics = true
	a.settings.DesktopLyricsSingle = true
	a.pl.Cue([]music.Track{song("7", "宝船", "DOUDOU")})
	a.pl.SetLyrics(twoLines())
	a.pl.Attach(audio.NewPlayer(&silence{}))
	a.dlyr.sync()

	tt := ui.NewTester(a.dlyr.view, desktopLyricsW, barHeight(a.settings.DesktopLyricsSize, true))
	a.pl.Seek(5 * time.Second)
	tt.Frame() // the second line sings, alone
	if !tt.HasText("second line") {
		t.Fatalf("at 5s: %q", tt.Texts())
	}
	if tt.HasText("第二行") {
		t.Fatalf("the translation shows under a single line: %q", tt.Texts())
	}
}
