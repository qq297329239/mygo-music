# 桌面歌词浮窗实现计划

## Context

用户希望 mygo-music 像网易云等播放器一样，在屏幕底部（任务栏上方）显示跟随播放进度的歌词。经讨论选定"桌面歌词浮窗"方案：一个无边框、透明背景、置顶、点击穿透、不进任务栏的歌词窗口，使用应用自己的歌词数据（含本地 .lrc、翻译），全平台生效（Windows/macOS/Linux 共用 mygo API）。不采用任务栏内嵌（Win32 SetParent，脆弱且仅 Windows）和外部对接（taskbar-lyrics-displayer 只认四大播放器且歌词在线搜索、与应用内不同源）。

**现成模板**：notch 第二窗口（`internal/app/ui_notch.go:49-94`）展示了 WindowOptions/ShowInactive/SetIgnoreMouseEvents/Invalidate 全套模式；歌词数据流为 `pl.Lyrics()` + `lyrics.Index(lines, pos)`（ui_nowplaying.go:634-638）。

**已核实的关键 API**：
- `mygo.Screen.PrimaryDisplay().WorkArea`（mygo modules.go:265-303，WorkArea 已排除任务栏/Dock/菜单栏，全平台一致）、`mygo.Screen.OnDisplaysChanged`
- `Player.Playing()/Position()/Current()/Lyrics()`（player.go:357-385）
- `lyrics.Line{At, End, Text, Translation, Words}`、`lyrics.Index`（lyrics.go:17-30, 215）、`Lang()`（:203，日语配日文字体）
- 文本超宽截断用项目惯例 `.SingleLine().Shrink(1).MinWidth(0)`

## 文件改动

### 新增 `internal/app/ui_desktop_lyrics.go`

```go
type desktopLyrics struct {
    a     *app
    win   *mygo.Window // 测试中为 nil
    shown bool         // 有歌词且开关开时为 true（win==nil 也维护此状态）
}
const desktopLyricsW = 920; const desktopLyricsH = 76
```

- `newDesktopLyrics(a *app)`：
  - `mygo.WindowOptions{Title: "<App> Desktop Lyrics", Panel: true, Hidden: true, Frameless: true, Transparent: true, DisableShadow: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true, X/Y/Width/Height: place(), Content: ui.View(d.view)}`
  - `OnClose` 非退出时阻止（照 ui_notch.go:69）；`SetIgnoreMouseEvents(true)` 恒穿透（无 poll）
  - `mygo.Screen.OnDisplaysChanged` → 重新 `place()` 并 `win.SetPosition`（不重建窗口）
- `place()`：`wa := mygo.Screen.PrimaryDisplay().WorkArea`；`w = min(920, wa.Width-32)`；底部居中 `x = wa.X+(wa.Width-w)/2`，`y = wa.Y+wa.Height-h-6`（贴任务栏上方，留 6px 边距）
- `setEnabled(on bool)`：照 notch（ui_notch.go:134）——on → `ShowInactive()`（不抢焦点），off → `Hide()` 并 `shown=false`
- `sync() bool`：view 开头调用。`want := a.settings.DesktopLyrics && 歌词就绪（pl.Lyrics() ok 且非空）`；`want != shown` 时切换 Show/Hide（win 为 nil 只改状态）；返回 want；false 时 view 返回透明空内容
- `view(c *ui.Context)`：
  - `c.Root().Background(ui.Transparent)`；`pos := pl.Position()`，`cur := lyrics.Index(lines, pos)`；`cur < 0`（前奏）不画文字
  - **行组合提为纯函数** `lyricRows(lines []lyrics.Line, cur int) (main, sub string)`：主行 = 行 cur 的 `Text`；副行 = `Translation` 非空时用翻译，否则用行 cur+1 的 `Text`（无则空）——便于测试
  - 布局：居中两行。主行字号 22、`a.pal.accent`、粗体；副行字号 13、白 0.55。均 `.SingleLine().Shrink(1).MinWidth(0)` 截断。`Lang()` 为日语时按 nowplaying 先例配日文字体
  - 换行平滑过渡：`ui.Box(...).AnimateTo("at", float32(max(cur,0)), ui.Animation{Spring: ui.SpringOptions{Duration: 400ms}})`，行间用 frac 做轻量位移/透明过渡（简单实现，不追求卡拉OK逐字）
  - 推进：`if pl.Playing() && cur+1 < len(lines) { c.After(lines[cur+1].At - pos + 20ms) }`（照 ui_nowplaying.go:637-638）——暂停时不安排帧、静止显示
  - v1 不做逐词 Words 高亮，留后续

### 修改 `internal/app/app.go`

- `app` 结构体 notch 字段旁加 `dlyr *desktopLyrics`
- `redraw()`（app.go:192）加：`if a.dlyr != nil && a.dlyr.win != nil { a.dlyr.win.Invalidate() }`
- `Run()` 中 notch 创建块（app.go:384）之后**无条件**创建（不限 darwin）：`a.dlyr = newDesktopLyrics(a)`

### 修改 `internal/app/settings.go`

- `settings` 加 `DesktopLyrics bool \`json:"desktopLyrics"\``（默认 false，风格同 Notch :34）

### 修改 `internal/app/ui_settings.go`

- `changed()`（:70）加 `if a.dlyr != nil { a.dlyr.setEnabled(a.settings.DesktopLyrics) }`
- notch 开关（:397-402）后加一行 `ui.Switch(c.Key("desktop-lyrics"), &s.DesktopLyrics).Label(T("Show Desktop Lyrics")).Changed() { a.changed() }` 的开关行（row 样式照抄）

### 修改 `internal/i18n/zh_cn.go`

- 加 3 条映射："Desktop Lyrics"→"桌面歌词"；"Show Desktop Lyrics"→"显示桌面歌词"；描述句 "A bar of lyrics above the taskbar, following the song as it plays"→"屏幕底部任务栏上方跟随歌曲滚动的歌词条"

## 测试（新 `internal/app/desktop_lyrics_test.go`，遵循 AGENTS.md）

每个测试开头一句注释说明验证的行为：

1. `TestDesktopLyricsShowsWithLyrics`：`newTestApp` + `d := &desktopLyrics{a: a}`（win nil）；无歌词时 `sync()` false；`fake(a)` 带歌词的歌 `Cue` 后 `runPost` 等歌词加载完成，开开关后 `sync()` true；换无歌词的歌回 false
2. `TestDesktopLyricsLineChoice`：测 `lyricRows`——pos 落在行内取对行、前奏（cur<0）返回空、翻译优先于下一行作副行
3. `TestDesktopLyricsSettingSwitch`：`ui.NewTester` + 按 accessibility label 点击开关，断言 `a.settings.DesktopLyrics` 翻转（设置开关先例见 frames_test.go / ui_kit 的 Switch 用法）

不测动画数值、不测视觉文案列表、不依赖 wall clock（`c.After` 的推进由框架处理）。

## 验证

1. `go test ./internal/...`
2. `go test ./internal/app -run DesktopLyrics -count=5`（新 UI 测试按 AGENTS.md 跑 5 次）
3. `go tool mygo dev` 手动验证：设置开关即时显隐；有歌词播放换行平滑推进；暂停静止；无歌词/切歌隐藏；点击穿透到桌面；置顶且不进任务栏；改分辨率/换显示器后位置仍正确

## 关键参考文件

- `internal/app/ui_notch.go`（第二窗口全套模式：setEnabled/ShowInactive/OnClose）
- `internal/app/app.go:178-192`（post/redraw 接线）、`app.go:384`（创建时机）
- `internal/app/ui_nowplaying.go:634-638`（歌词行计算与 c.After 推进）
- `internal/app/settings.go:34`、`internal/app/ui_settings.go:70-77,397-402`（设置接入）
- mygo 源码 `modules.go:260-336`（Screen/WorkArea/OnDisplaysChanged）
