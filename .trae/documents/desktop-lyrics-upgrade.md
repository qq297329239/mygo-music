# 桌面歌词浮窗增强实施计划

## Context

现有桌面歌词(`internal/app/ui_desktop_lyrics.go`)是固定 920x76 底部居中的透明条,字号/颜色/行数均硬编码,不可交互。本次增强:任务栏覆盖模式、自定义字体与字号、卡拉OK双色(已播放/未播放)、解锁后可拖动与调整大小、单双行、背景与不透明度、粗细、对齐。用户已确认:任务栏用**覆盖式**(窗口浮在任务栏区域上,非 Win32 嵌入);额外设置项为背景+不透明度、粗细、对齐。

**关键先例/API**(mygo v0.9.0,源码 `C:\Users\aaron\go\pkg\mod\github.com\egoist\mygo@v0.9.0\`):
- 逐词着色先例:`internal/app/ui_nowplaying.go:705-716`(RichText + 每词 TextColor,播放中 `c.After(40ms)`);Span 未设 shadow 时继承元素级 `TextShadow`(richtext.go:413)
- `ui.FontFamilies()`(已注册+系统字体)、`Element.Font(family)`、`ui.Hex(s)`、`Color.Mix(o,t)`
- `ui.ColorWell(c, *Color)`(colorpicker.go,小色块弹 popover,首用)
- `Window.SetResizable(v)` 运行时切换 frameless 隐形 resize 边(左/右/下+topedge 子窗口);`Element.DragWindow()`;`SetMinimumSize`;`SetIgnoreMouseEvents`
- 防抖模式:`session.go:68` `time.AfterFunc(d, func(){ mygo.RunOnMain(fn) })`
- 歌词 `Words` 总被填充(无逐字时 SpreadWords 均摊,CJK 逐字),逐词着色永远可行

## 一、settings.go 新增字段(零值=现状,defaultSettings 不改)

```go
// The desktop lyrics' looks: what they are drawn with, and where.
DesktopLyricsTaskbar    bool    `json:"desktopLyricsTaskbar,omitempty"`    // 贴附任务栏
DesktopLyricsSingleLine bool    `json:"desktopLyricsSingleLine,omitempty"` // 单行
DesktopLyricsFont       string  `json:"desktopLyricsFont,omitempty"`       // ""=默认字体
DesktopLyricsSize       int     `json:"desktopLyricsSize,omitempty"`       // 0=22
DesktopLyricsWeight     int     `json:"desktopLyricsWeight,omitempty"`     // 0=700;400=常规
DesktopLyricsAlign      string  `json:"desktopLyricsAlign,omitempty"`      // ""=居中;"left"
DesktopLyricsPlayed     string  `json:"desktopLyricsPlayed,omitempty"`     // hex;""=accent
DesktopLyricsPending    string  `json:"desktopLyricsPending,omitempty"`    // hex;""=白55%
DesktopLyricsBg         bool    `json:"desktopLyricsBg,omitempty"`         // 背景开关
DesktopLyricsBgColor    string  `json:"desktopLyricsBgColor,omitempty"`    // hex 含alpha;""=黑60%
DesktopLyricsUnlocked   bool    `json:"desktopLyricsUnlocked,omitempty"`   // 解锁位置
DesktopLyricsX          int     `json:"desktopLyricsX,omitempty"`          // 零值=自动 place
DesktopLyricsY          int     `json:"desktopLyricsY,omitempty"`
DesktopLyricsW          int     `json:"desktopLyricsW,omitempty"`
DesktopLyricsH          int     `json:"desktopLyricsH,omitempty"`
```

## 二、ui_desktop_lyrics.go(大改)

**纯函数(可测)**:
- `taskbarRect(bounds, wa mygo.Rectangle) (x, y, w, h int, ok bool)`:任务栏矩形 = WorkArea 底与屏幕底之间的条带,全宽;`h < 28` 或非底部(任务栏在侧边/自动隐藏)→ ok=false 回退浮动
- `barHeight(size int, single bool) int`:双行 `round(size*1.55 + 13*1.55 + 16)` clamp 60..140;单行 clamp 40..90
- `wordColor(played, pending ui.Color, pos, start, end time.Duration) ui.Color`:已完→played;未始→pending;进行中→`pending.Mix(played, lit)`
- 颜色解析 `d.playedColor()/pendingColor()/bgColor()`:hex 非法或空回默认(accent / `white.Alpha(0.55)` / `ui.RGBA(0,0,0,0.6)`)

**place() 分支**:任务栏模式(忽略自定义位置)→ 已解锁且有保存的 X/Y/W/H → 默认浮动条(宽 `min(920, wa.Width-32)`、高 `barHeight`,底部居中)。保存位置 clamp 进屏幕。

**窗口交互(applyMode)**:
- 解锁且非任务栏:`SetIgnoreMouseEvents(false)` + `SetResizable(true)` + `SetMinimumSize(360, 44)`;锁定/任务栏:反向恢复
- `OnResize/OnMove`(主线程、拖动中频发):仅解锁且非任务栏时更新内存字段,`time.AfterFunc(500ms, RunOnMain(save))` 防抖落盘(照 session.go:68)
- `OnDisplaysChanged`:重新 place(自定义位置 clamp);sync 显示时重申 `SetAlwaysOnTop(true)`(任务栏同为 topmost band,点击任务栏后 explorer 可能重排——需手动验证,不足再在播放 tick 重申)

**view() 骨架**:
- 背景:`DesktopLyricsBg` 开 → `c.Root().Background(bgColor)`(否则 Transparent);解锁时根容器 `.DragWindow()`
- 换行 roll 弹簧、`lyricRows`、阴影、截断(`SingleLine().Shrink(1).MinWidth(0)`)保持现状
- 主行改 `ui.RichText`:`.Font(s.DesktopLyricsFont)(空不设).FontSize(默认22).FontWeight(默认700).TextColor(pending).TextShadow(元素级,词继承).TextAlign(默认Center,"left"→Start)`,Children 内每词 `ui.Text(c.Key(i), wd.Text).TextColor(wordColor(...))`
- 播放中 `c.After(40ms)` 逐词刷新(nowplaying 先例;暂停时仍只在下一行起始处要帧)
- 单行/任务栏模式:不画副行;副行字号按 `max(10, round(size*0.6))`、pending 色淡化
- 任务栏模式:字号自动 `min(size, h-16)` 防溢出

## 三、ui_settings.go desktop-lyrics 区行清单

全部 `Disabled(!s.DesktopLyrics)`(首行除外);写法照 row/choice 先例(:109-123、:398-404):

1. Show Desktop Lyrics(现有)
2. Attach to the Taskbar — Switch
3. Unlock Position and Size — Switch(开启后拖动窗口、拉边缘调整,锁定后恢复点击穿透)
4. Font — choice:Default + `ui.FontFamilies()` 名称列表
5. Font Size — `ui.SliderBase(c, &v, 12, 48)`,旁显取整数值
6. Font Weight — choice:Bold / Regular(700/400)
7. Alignment — choice:Center / Left
8. Lines — `segmented(c, &sel, T("Double"), T("Single"))`(ui_kit.go:284 封装;任务栏模式时此行 Disabled)
9. Played Color — `ui.ColorWell(c, &col)`;Pending Color — ColorWell(回写 hex string)
10. Background — Switch + Background Color — ColorWell + Opacity — SliderBase 0.15–1(合成进 bgColor 的 alpha)

`a.changed()`(已接 `dlyr.sync()`)之外,行数/字号/任务栏/位置变化需重设窗口大小位置:在 desktopLyrics 上加 `applySettings()`,由 changed() 中 `dlyr.sync()` 内顺带执行(sync 已每设置变更调用;win 为 nil 的测试路径安全)。

## 四、i18n(internal/i18n/zh_cn.go,按字母序)

约 16 条:桌面歌词各设置行 label/desc(Attach to the Taskbar→贴附任务栏、Unlock Position and Size→解锁位置和大小、Played Color→已播放颜色、Pending Color→未播放颜色、Background→背景、Opacity→不透明度、Lines→行数、Double/Single→双行/单行、Bold/Regular、Center/Left、Default 等)+ 原 desc 更新。

## 五、测试(desktop_lyrics_test.go,遵循 AGENTS.md)

每个测试一句行为注释;现有三个保留:
- `TestDesktopLyricsWordColor`:未开始/进行中(中点=Mix)/已完 三区间
- `TestDesktopLyricsTaskbarRect`:底部任务栏矩形正确;侧边(h=0)与过矮(<28)回退 ok=false
- `TestDesktopLyricsBarHeight`:字号×单双行边界
- `TestDesktopLyricsSingleLine`(NewTester):Seek 到带翻译行,单行开 → `HasText(主行)` 且 `!HasText(翻译)`;双行 → 两者都在
- 不测颜色像素、默认值清单、墙钟;新 UI 测试 `-count=5`

## 六、验证

1. `go build ./... && go test ./internal/...`;`go test ./internal/app -run DesktopLyrics -count=5`
2. `go tool mygo dev` 手动:任务栏模式(底部/左侧任务栏/自动隐藏回退;点击任务栏后是否仍覆盖)、解锁拖动+四边 resize(Win11 透明窗口是否出 DWM 细边)、锁定恢复穿透、字体/字号/粗细/对齐/双色卡拉OK平滑渐变/背景不透明度、重启后位置持久化、多显示器拔插

## 关键文件

- `internal/app/ui_desktop_lyrics.go`(大改)
- `internal/app/settings.go`(字段)
- `internal/app/ui_settings.go`(设置页)
- `internal/i18n/zh_cn.go`(条目)
- `internal/app/desktop_lyrics_test.go`(测试)
- app.go 不改(changed→sync 链路已通)
