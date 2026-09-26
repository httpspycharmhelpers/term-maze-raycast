package main

import (
	"bytes"
	"fmt"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ---- 墙面 ASCII 图片 / 视频 ----

type WallImage struct {
	X, Y  int
	Lines []string
	W, H  int
	// Runes 与 Lines 同尺寸的逐字runes，mediaCell 用它们按“字”采样（Lines 是字节，宽字符会错位）
	Runes [][]rune
	// Colors 可选：与 Lines 同尺寸的每格真彩 RGB（r<<16|g<<8|b），0 表示沿用墙体默认色
	Colors [][]uint32
}

type Video struct {
	X, Y    int
	Frames  []WallImage
	Speed   time.Duration
	Playing bool
	Start   time.Time
	// FPS 非 0 表示 ffmpeg 抽取的视频（用于按帧率换帧）
	FPS int
	// Audio 是 ffplay 音频子进程（mp4 音轨），stop/remove/退出时需要杀掉
	Audio *exec.Cmd
}

var wallImages []WallImage
var videos []Video
var nextImageID int

// sanitizeArtPath 校验并返回 ~ 下的完整路径（只允许访问 HOME 目录）
func sanitizeArtPath(path string) (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("无法确定 HOME 目录")
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(home, path)
	}
	full = filepath.Clean(full)
	rel, err := filepath.Rel(home, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("只能访问 ~ 目录下的文件")
	}
	return full, nil
}

// readArtFile 读取 ~/ 下的 ASCII 图片/视频文件（只允许访问 HOME 目录）
func readArtFile(path string) ([]string, error) {
	full, err := sanitizeArtPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	raw := strings.Split(string(data), "\n")
	lines := make([]string, 0, len(raw))
	for _, l := range raw {
		l = strings.TrimRight(l, "\r")
		lines = append(lines, l)
	}
	// 去掉文件末尾换行带出的幽灵空行
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

func readArtBytes(path string) ([]byte, error) {
	full, err := sanitizeArtPath(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// isRealImageExt 判断是否为可直接解码的真实图片/动图文件
func isRealImageExt(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp":
		return true
	}
	return false
}

// asciiImage 把真实图片降采样成 ASCII 字符画（亮度→字符渐变），同时给出每格真彩
func asciiImage(img image.Image, maxCols int) ([]string, [][]uint32) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return []string{""}, [][]uint32{{0}}
	}
	if maxCols < 8 {
		maxCols = 8
	}
	if maxCols > w {
		maxCols = w
	}
	// 终端字符约 2:1（高/宽），按此换算行数
	rows := int(float64(maxCols) * float64(h) / float64(w) * 0.5)
	if rows < 1 {
		rows = 1
	}
	if rows > 60 {
		rows = 60
		maxCols = int(float64(rows) * float64(w) / float64(h) * 2)
		if maxCols < 8 {
			maxCols = 8
		}
	}
	ramp := []rune(" .·:;!%#@")
	lines := make([]string, 0, rows)
	colors := make([][]uint32, 0, rows)
	for r := 0; r < rows; r++ {
		line := make([]rune, maxCols)
		colRow := make([]uint32, maxCols)
		for c := 0; c < maxCols; c++ {
			x0 := b.Min.X + c*w/maxCols
			x1 := b.Min.X + (c+1)*w/maxCols
			y0 := b.Min.Y + r*h/rows
			y1 := b.Min.Y + (r+1)*h/rows
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			var rs, gs, bs, n int64
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					cr, cg, cb, _ := img.At(x, y).RGBA()
					rs += int64(cr >> 8)
					gs += int64(cg >> 8)
					bs += int64(cb >> 8)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			R := byte(rs / n)
			G := byte(gs / n)
			B := byte(bs / n)
			lum := (299*int(R) + 587*int(G) + 114*int(B)) / 1000
			idx := int(lum) * (len(ramp) - 1) / 255
			if idx >= len(ramp) {
				idx = len(ramp) - 1
			}
			line[c] = ramp[idx]
			colRow[c] = uint32(R)<<16 | uint32(G)<<8 | uint32(B)
		}
		lines = append(lines, string(line))
		colors = append(colors, colRow)
	}
	return lines, colors
}

// loadMediaFrames 解码真实图片/GIF 为墙贴图帧；静态图返回 1 帧
func loadMediaFrames(path string) ([]WallImage, error) {
	data, err := readArtBytes(path)
	if err != nil {
		return nil, err
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法解码图片 %s: %v", path, err)
	}
	_ = cfg
	if format == "gif" {
		g, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("无法解码 GIF: %v", err)
		}
		if len(g.Image) == 0 {
			return nil, fmt.Errorf("GIF 没有帧")
		}
		var frames []WallImage
		maxW, maxH := 0, 0
		var ascii [][]string
		var asciiCol [][][]uint32
		for _, f := range g.Image {
			a, col := asciiImage(f, 44)
			ascii = append(ascii, a)
			asciiCol = append(asciiCol, col)
			if len(a) > maxH {
				maxH = len(a)
			}
			for _, l := range a {
				if len(l) > maxW {
					maxW = len(l)
				}
			}
		}
		for fi, a := range ascii {
			pad := make([]string, maxH)
			padCol := make([][]uint32, maxH)
			for i, l := range a {
				pad[i] = l
				rc := asciiCol[fi][i]
				for len(pad[i]) < maxW {
					pad[i] += " "
				}
				rc2 := make([]uint32, maxW)
				copy(rc2, rc)
				for j := len(rc); j < maxW; j++ {
					rc2[j] = 0
				}
				padCol[i] = rc2
			}
			for i := len(a); i < maxH; i++ {
				pad[i] = strings.Repeat(" ", maxW)
				padCol[i] = make([]uint32, maxW)
			}
			frame := makeWallImage(pad, 0, 0)
			frame.Colors = padCol
			frames = append(frames, frame)
		}
		return frames, nil
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("无法解码图片: %v", err)
	}
	lines, col := asciiImage(img, 44)
	frame := makeWallImage(lines, 0, 0)
	frame.Colors = col
	return []WallImage{frame}, nil
}

func makeWallImage(lines []string, x, y int) WallImage {
	runes := make([][]rune, len(lines))
	w := 1
	for i, l := range lines {
		runes[i] = []rune(l)
		if len(runes[i]) > w {
			w = len(runes[i])
		}
	}
	return WallImage{X: x, Y: y, Lines: lines, Runes: runes, W: w, H: len(lines)}
}

func imgLinkAt(x, y int) (int, int, *WallImage) {
	// 播放中的视频优先，其次静态图；返回 (是否视频, 是否音频帧, 图片指针)
	for i := range videos {
		v := &videos[i]
		if v.X != x || v.Y != y || !v.Playing || len(v.Frames) == 0 {
			continue
		}
		idx := 0
		if v.Speed > 0 {
			idx = int(time.Since(v.Start)/v.Speed) % len(v.Frames)
		}
		f := v.Frames[idx]
		return 1, i, &f
	}
	for i := range wallImages {
		im := &wallImages[i]
		if im.X == x && im.Y == y {
			return 0, i, im
		}
	}
	return -1, -1, nil
}

// mediaCell 墙面贴图/彩蛋优先级的字符选择；返回 (字符, 真彩色或0, 是否替换普通墙)
func mediaCell(img *WallImage, egg int, wx float64, y, ds, de int) (rune, uint32, bool) {
	if img != nil {
		if img.H > 0 && img.W > 0 {
			c := int(wx * float64(img.W))
			if c >= img.W {
				c = img.W - 1
			}
			rr := (y - ds) * img.H / (de - ds + 1)
			if rr >= img.H {
				rr = img.H - 1
			}
			var col uint32
			if rr < len(img.Colors) && c < len(img.Colors[rr]) {
				col = img.Colors[rr][c]
			}
			if rr < len(img.Runes) && c < len(img.Runes[rr]) {
				return img.Runes[rr][c], col, true
			}
			return ' ', col, true
		}
		return ' ', 0, true
	}
	if egg > 0 {
		pat := eggPatterns[egg]
		c := int(wx * 8)
		if c > 7 {
			c = 7
		}
		row := (y - ds) * 8 / (de - ds + 1)
		if row > 7 {
			row = 7
		}
		if ch := pat[row][c]; ch != ' ' {
			return ch, 0, true
		}
		return ' ', 0, false
	}
	return 0, 0, false
}

// ---- 彩蛋墙：随机生成但每次都严格对称的像素画 ----

var egGlyphs = []rune{'&', ';', '~', '%', '/', '=', '$', '^', '!', '*', '?'}

// eggPatterns[i] 为第 i 号彩蛋墙的 8×8 图案；每次重建地图时随迷宫一起随机生成
var eggPatterns [][8][8]rune

// genEggPattern 从形状族里随机挑一个“确定的对称形状”，再补随机参数与字符，
// 保证：左右对称 + 上下对称 + 成块成画（不是随机散点）；空白则重抽
func genEggPattern(rng *rand.Rand) [8][8]rune {
	for tries := 0; tries < 24; tries++ {
		p := genEggPatternOnce(rng)
		if eggFilled(p) > 0 {
			return p
		}
	}
	// 兜底：实心菱形
	glyph := egGlyphs[rng.Intn(len(egGlyphs))]
	return genFromPred(func(dx, dy float64) bool { return absF(dx)+absF(dy) <= 3.5 }, glyph)
}

func eggFilled(p [8][8]rune) int {
	n := 0
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			if p[r][c] != ' ' {
				n++
			}
		}
	}
	return n
}

func genFromPred(pred func(dx, dy float64) bool, glyph rune) [8][8]rune {
	var p [8][8]rune
	for r := range p {
		for c := range p[r] {
			p[r][c] = ' '
		}
	}
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			if pred(float64(c)-3.5, float64(r)-3.5) {
				p[r][c] = glyph
			}
		}
	}
	return p
}

func genEggPatternOnce(rng *rand.Rand) [8][8]rune {
	glyph := egGlyphs[rng.Intn(len(egGlyphs))]
	var pred func(dx, dy float64) bool
	fam := rng.Intn(11)
	k := 1.5 + rng.Float64()*2.2 // 核心大小 1.5~3.7
	k2 := 1.0 + rng.Float64()*1.5
	t := 1 + rng.Intn(2) // 条纹/十字粗细

	switch fam {
	case 0: // 实心菱形
		pred = func(dx, dy float64) bool { return absF(dx)+absF(dy) <= k }
	case 1: // 实心圆
		pred = func(dx, dy float64) bool { return dx*dx+dy*dy <= k*k }
	case 2: // 实心方
		pred = func(dx, dy float64) bool { return maxAbs(dx, dy) <= k }
	case 3: // 大方框
		pred = func(dx, dy float64) bool { return maxAbs(dx, dy) <= k && maxAbs(dx, dy) > k-2 }
	case 4: // 十字
		pred = func(dx, dy float64) bool { return absF(dx) <= float64(t) || absF(dy) <= float64(t) }
	case 5: // 菱形环
		pred = func(dx, dy float64) bool {
			d := absF(dx) + absF(dy)
			return d <= k && d > k-1.5
		}
	case 6: // 圆环
		pred = func(dx, dy float64) bool {
			d := math.Sqrt(dx*dx + dy*dy)
			return d <= k && d > k-k2
		}
	case 7: // 沙漏（腰部收窄）
		pred = func(dx, dy float64) bool {
			return absF(dx) <= k-absF(dy)
		}
	case 8: // 四角方块
		pred = func(dx, dy float64) bool { return absF(dx) >= k2 && absF(dy) >= k2 }
	case 9: // 方环双层
		pred = func(dx, dy float64) bool {
			m := maxAbs(dx, dy)
			return m <= k && m > k-0.9
		}
	default: // 斜向交叉带（X 形）
		pred = func(dx, dy float64) bool { return absF(dx-dy) <= 1.1 || absF(dx+dy) <= 1.1 }
	}

	eggDebug = fmt.Sprintf("fam=%d k=%.2f k2=%.2f t=%d glyph=%q", fam, k, k2, t, glyph)
	return genFromPred(pred, glyph)
}

var eggDebug string

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func maxAbs(a, b float64) float64 {
	if absF(a) > absF(b) {
		return absF(a)
	}
	return absF(b)
}

// ---- 命令 ----

// parseWallPos 解析 [x y]；缺省时在玩家附近自动找一面墙放置
func parseWallPos(args []string) (x, y int, ok bool) {
	if len(args) >= 2 {
		if ix, err1 := strconv.Atoi(args[len(args)-2]); err1 == nil {
			if iy, err2 := strconv.Atoi(args[len(args)-1]); err2 == nil {
				return ix, iy, true
			}
		}
	}
	if wx, wy, found := nearbyWallCell(); found {
		return wx, wy, true
	}
	return frontCell()
}

// homeMediaList 列出 ~ 目录下某类媒体文件（img=gif支持多帧，video 含 mp4）
func homeMediaList(kind string) string {
	var exts []string
	if kind == "img" {
		exts = []string{".png", ".jpg", ".jpeg", ".gif"}
	} else {
		exts = []string{".gif", ".mp4"}
	}
	files := homeMediaFiles(exts...)
	if len(files) == 0 {
		return "（~ 目录没有这些文件：" + kind + " → " + strings.Join(exts, " ") + "）"
	}
	return "~/ 下 " + kind + " 文件：\n  " + strings.Join(files, "\n  ")
}

// homeMediaFiles 扫描 ~ 目录，返回指定扩展名（小写、带点）的文件名列表
func homeMediaFiles(exts ...string) []string {
	home := os.Getenv("HOME")
	if home == "" {
		return nil
	}
	var found []string
	dir, err := os.ReadDir(home)
	if err != nil {
		return nil
	}
	for _, e := range dir {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		for _, want := range exts {
			if ext == want {
				found = append(found, name)
				break
			}
		}
	}
	return found
}

func cmdImg(args []string) (string, int) {
	if len(args) < 1 {
		return "用法: img load <文件> [x y] | list | remove <id>", 1
	}
	switch args[0] {
	case "load":
		if len(args) < 2 {
			return "用法: img load <文件> [x y]", 1
		}
		path := args[1]
		rest := args[2:]
		x, y, ok := parseWallPos(rest)
		if !ok {
			return "没有可放置的墙面位置", 1
		}
		var lines []string
		var colors [][]uint32
		note := "ASCII 文本图"
		if isRealImageExt(path) {
			frames, err := loadMediaFrames(path)
			if err != nil {
				return "解码失败: " + err.Error(), 1
			}
			lines = frames[0].Lines
			colors = frames[0].Colors
			note = "真实图片转 ASCII 彩色"
		} else {
			l, err := readArtFile(path)
			if err != nil {
				return "读取失败: " + err.Error(), 1
			}
			lines = l
		}
		im := makeWallImage(lines, x, y)
		im.Colors = colors
		nextImageID++
		wallImages = append(wallImages, im)
		return fmt.Sprintf("图片 #%d 已贴到墙面 (%d,%d)（%s），尺寸 %dx%d", nextImageID, x, y, note, im.W, im.H), 0
	case "list":
		var out []string
		if s := homeMediaList("img"); s != "" {
			out = append(out, s)
		}
		if len(wallImages) > 0 {
			out = append(out, "", "已加载到墙面的图片：")
			for i, im := range wallImages {
				out = append(out, fmt.Sprintf("#%d 图 (%d,%d) %dx%d", i+1, im.X, im.Y, im.W, im.H))
			}
		}
		if len(out) == 0 {
			return "还没有贴墙图片，~/ 下也没有图片文件", 0
		}
		return strings.Join(out, "\n"), 0
	case "remove":
		id, err := strconv.Atoi(args[1])
		if err != nil || id < 1 || id > len(wallImages) {
			return "id 无效", 1
		}
		wallImages = append(wallImages[:id-1], wallImages[id:]...)
		return fmt.Sprintf("已移除图片 #%d", id), 0
	}
	return "未知子命令: " + args[0], 1
}

func cmdVideo(args []string) (string, int) {
	if len(args) < 1 {
		return "用法: video play <文件> [x y] | list | stop <id> | remove <id>", 1
	}
	switch args[0] {
	case "play":
		if len(args) < 2 {
			return "用法: video play <文件> [x y]", 1
		}
		path := args[1]
		rest := args[2:]
		x, y, ok := parseWallPos(rest)
		if !ok {
			return "没有可放置的墙面位置", 1
		}
		ext := strings.ToLower(filepath.Ext(path))
		var frames []WallImage
		var srate time.Duration = 150 * time.Millisecond
		src := ""
		fps := 0
		var audio *exec.Cmd
		switch {
		case ext == ".mp4":
			f, f0, hasAudio0, err := mp4ToFrames(avPath(path))
			if err != nil {
				return "mp4 失败: " + err.Error(), 1
			}
			frames = f
			fps = f0
			srate = time.Second / time.Duration(fps)
			src = fmt.Sprintf("mp4→ASCII 真彩 %dfps %d帧", fps, len(frames))
			if hasAudio0 {
				a, player := startAudio(avPath(path))
				audio = a
				if player != "" {
					src += fmt.Sprintf(" + 音轨(%s播放)", player)
				} else {
					src += "，无音频播放器(ffplay/mpv/termux-media-player)"
				}
			}
			if len(frames) >= maxVideoFrames {
				src += "（超长，截取前 10 分钟）"
			}
		case ext == ".gif":
			f, err := loadMediaFrames(path)
			if err != nil {
				return "解码失败: " + err.Error(), 1
			}
			frames = f
			src = fmt.Sprintf("GIF 动图 %d帧", len(frames))
		case isRealImageExt(path):
			f, err := loadMediaFrames(path)
			if err != nil {
				return "解码失败: " + err.Error(), 1
			}
			// 静态图当成 2 帧循环播放
			frames = []WallImage{f[0], f[0]}
			src = "图片循环"
		default:
			lines, err := readArtFile(path)
			if err != nil {
				return "读取失败: " + err.Error(), 1
			}
			f := splitFrames(lines)
			if len(f) == 0 {
				return "视频文件为空或格式不对（帧之间用 --- 分隔）", 1
			}
			frames = f
			src = "ASCII 文本帧"
		}
		nextImageID++
		v := Video{X: x, Y: y, Frames: frames, Speed: srate, Playing: true, Start: time.Now(), FPS: fps, Audio: audio}
		videos = append(videos, v)
		return fmt.Sprintf("视频 #%d 正在墙面 (%d,%d) 播放（%s，%s，video stop 停止）", nextImageID, x, y, src, srate.Round(time.Millisecond)), 0
	case "stop":
		id, err := strconv.Atoi(args[1])
		if err != nil || id < 1 {
			return "id 无效", 1
		}
		id--
		if id >= len(videos) {
			return "没有该视频", 1
		}
		videos[id].Playing = false
		stopAudio(videos[id].Audio)
		videos[id].Audio = nil
		return fmt.Sprintf("已停止视频 #%d（含音轨）", id+1), 0
	case "list":
		var out []string
		if s := homeMediaList("video"); s != "" {
			out = append(out, s)
			out = append(out, "（.gif/.mp4/.jpg/.png 均可 play，mp4 需要已装 ffmpeg）")
		}
		if len(videos) > 0 {
			out = append(out, "", "已加载到墙面的视频：")
			for i, v := range videos {
				st := "停"
				if v.Playing {
					st = "播"
				}
				aud := "无声"
				if v.Audio != nil {
					aud = "有声"
				}
				fpsS := ""
				if v.FPS > 0 {
					fpsS = fmt.Sprintf(" %dfps", v.FPS)
				}
				out = append(out, fmt.Sprintf("#%d 视频 (%d,%d) %d帧%s %s %s", i+1, v.X, v.Y, len(v.Frames), fpsS, aud, st))
			}
		}
		if len(out) == 0 {
			return "还没有视频，~/ 下也没有视频文件", 0
		}
		return strings.Join(out, "\n"), 0
	case "remove":
		id, err := strconv.Atoi(args[1])
		if err != nil || id < 1 || id > len(videos) {
			return "id 无效", 1
		}
		stopAudio(videos[id-1].Audio)
		videos = append(videos[:id-1], videos[id:]...)
		return fmt.Sprintf("已移除视频 #%d", id), 0
	}
	return "未知子命令: " + args[0], 1
}

// splitFrames 按单独一行 --- 分隔视频帧
func splitFrames(lines []string) []WallImage {
	var frames []WallImage
	var cur []string
	flush := func() {
		// 忽略完全由空行组成的帧
		nonEmpty := false
		for _, l := range cur {
			if strings.TrimSpace(l) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			frames = append(frames, makeWallImage(cur, 0, 0))
		}
		cur = nil
	}
	for _, l := range lines {
		if strings.TrimSpace(l) == "---" {
			flush()
			continue
		}
		cur = append(cur, l)
	}
	flush()
	return frames
}
