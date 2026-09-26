package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
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
}

type Video struct {
	X, Y    int
	Frames  []WallImage
	Speed   time.Duration
	Playing bool
	Start   time.Time
}

var wallImages []WallImage
var videos []Video
var nextImageID int

// readArtFile 读取 ~/ 下的 ASCII 图片/视频文件（只允许访问 HOME 目录）
func readArtFile(path string) ([]string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return nil, fmt.Errorf("无法确定 HOME 目录")
	}
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(home, path)
	}
	full = filepath.Clean(full)
	rel, err := filepath.Rel(home, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("只能访问 ~ 目录下的文件")
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

func makeWallImage(lines []string, x, y int) WallImage {
	w := 0
	for _, l := range lines {
		if len(l) > w {
			w = len(l)
		}
	}
	if w == 0 {
		w = 1
	}
	return WallImage{X: x, Y: y, Lines: lines, W: w, H: len(lines)}
}

func imgLinkAt(x, y int) (int, int, *WallImage) {
	// 播放中的视频优先，其次静态图；返回 (是否视频, 是否音频帧, 图片指针)
	for i := range videos {
		v := &videos[i]
		if v.X != x || v.Y != y || !v.Playing || len(v.Frames) == 0 {
			continue
		}
		idx := int(time.Since(v.Start)/v.Speed) % len(v.Frames)
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

// wallRune 墙面贴图/彩蛋优先级的字符选择；返回 (字符, 是否替换普通墙)
func mediaRune(img *WallImage, egg int, wx float64, y, ds, de int) (rune, bool) {
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
			if rr < len(img.Lines) && c < len(img.Lines[rr]) {
				return rune(img.Lines[rr][c]), true
			}
			return ' ', true
		}
		return ' ', true
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
			return ch, true
		}
		return ' ', false
	}
	return 0, false
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

func maxAbs(a, b float64) float64 {
	if absF(a) > absF(b) {
		return absF(a)
	}
	return absF(b)
}

// ---- 命令 ----

func parseWallPos(args []string) (x, y int, ok bool) {
	if len(args) >= 2 {
		if ix, err1 := strconv.Atoi(args[len(args)-2]); err1 == nil {
			if iy, err2 := strconv.Atoi(args[len(args)-1]); err2 == nil {
				return ix, iy, true
			}
		}
	}
	return frontCell()
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
		lines, err := readArtFile(path)
		if err != nil {
			return "读取失败: " + err.Error(), 1
		}
		im := makeWallImage(lines, x, y)
		nextImageID++
		wallImages = append(wallImages, im)
		return fmt.Sprintf("图片 #%d 已贴到墙面 (%d,%d)，尺寸 %dx%d", nextImageID, x, y, im.W, im.H), 0
	case "list":
		if len(wallImages) == 0 {
			return "还没有贴墙图片", 0
		}
		var out []string
		for i, im := range wallImages {
			out = append(out, fmt.Sprintf("#%d 图 (%d,%d) %dx%d", i+1, im.X, im.Y, im.W, im.H))
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
		lines, err := readArtFile(path)
		if err != nil {
			return "读取失败: " + err.Error(), 1
		}
		frames := splitFrames(lines)
		if len(frames) == 0 {
			return "视频文件为空或格式不对（帧之间用 --- 分隔）", 1
		}
		nextImageID++
		v := Video{X: x, Y: y, Frames: frames, Speed: 150 * time.Millisecond, Playing: true, Start: time.Now()}
		videos = append(videos, v)
		return fmt.Sprintf("视频 #%d 正在墙面 (%d,%d) 播放，%d 帧，帧间 150ms（video stop 停止）", nextImageID, x, y, len(frames)), 0
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
		return fmt.Sprintf("已停止视频 #%d", id+1), 0
	case "list":
		if len(videos) == 0 {
			return "还没有视频", 0
		}
		var out []string
		for i, v := range videos {
			st := "停"
			if v.Playing {
				st = "播"
			}
			out = append(out, fmt.Sprintf("#%d 视频 (%d,%d) %d帧 %s", i+1, v.X, v.Y, len(v.Frames), st))
		}
		return strings.Join(out, "\n"), 0
	case "remove":
		id, err := strconv.Atoi(args[1])
		if err != nil || id < 1 || id > len(videos) {
			return "id 无效", 1
		}
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
