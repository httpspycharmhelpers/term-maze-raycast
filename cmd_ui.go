package main

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/eiannone/keyboard"
)

// ---- 命令模式 UI 状态 ----

type commandUI struct {
	active  bool
	closing bool
	openT   time.Time
	closeT  time.Time
	buf     []rune
	hist    []string
	histIdx int
	out     []string
	scroll  int // 输出区上滚的行数（0 = 底部最新）
	visible int // 最近一次绘制时输出区能显示的行数
}

var ui commandUI

const cmdAnimTime = 150 * time.Millisecond

// panelHeight 面板完全展开的高度：屏幕的 1/3，至少 10 行
func panelHeight(h int) int {
	ph := h / 3
	if ph < 10 {
		ph = 10
	}
	if ph > h-4 {
		ph = h - 4
	}
	return ph
}

func cmdPrint(s string) {
	ui.scroll = 0
	for _, l := range strings.Split(s, "\n") {
		ui.out = append(ui.out, l)
	}
	if len(ui.out) > 200 {
		ui.out = ui.out[len(ui.out)-200:]
	}
}

// handleCmdKey 处理命令模式下的按键，返回 true 表示已消费
func handleCmdKey(char rune, key keyboard.Key) bool {
	// 收起动画期间仍然吞掉按键，避免按键漏进游戏
	if ui.closing {
		return true
	}
	switch {
	case key == keyboard.KeyEsc || char == 27:
		ui.closing = true
		ui.closeT = time.Now()
		ui.buf = ui.buf[:0]
		return true
	case key == keyboard.KeyEnter || char == '\r' || char == '\n':
		line := strings.TrimSpace(string(ui.buf))
		ui.buf = ui.buf[:0]
		if line == "" {
			return true
		}
		ui.hist = append(ui.hist, line)
		ui.histIdx = len(ui.hist)
		executeLine(line)
		return true
	case key == keyboard.KeyArrowUp:
		if ui.histIdx > 0 {
			ui.histIdx--
			ui.buf = []rune(ui.hist[ui.histIdx])
		}
		return true
	case key == keyboard.KeyArrowDown:
		if ui.histIdx < len(ui.hist)-1 {
			ui.histIdx++
			ui.buf = []rune(ui.hist[ui.histIdx])
		} else {
			ui.histIdx = len(ui.hist)
			ui.buf = ui.buf[:0]
		}
		return true
	case key == keyboard.KeyPgup || char == 0x15 || key == keyboard.KeyCtrlU: // PgUp 或 Ctrl+U 上滚
		scrollOutput(+1)
		return true
	case key == keyboard.KeyPgdn || char == 0x04 || key == keyboard.KeyCtrlD: // PgDn 或 Ctrl+D 下滚
		scrollOutput(-1)
		return true
	case key == keyboard.KeySpace:
		// 某些终端会把空格解析成特殊键（char=0, key=KeySpace）
		ui.buf = append(ui.buf, ' ')
		return true
	case key == keyboard.KeyBackspace || key == keyboard.KeyBackspace2 || char == 8 || char == 127:
		if len(ui.buf) > 0 {
			ui.buf = ui.buf[:len(ui.buf)-1]
		}
		return true
	case char >= 32 && char != 127:
		ui.buf = append(ui.buf, char)
		return true
	}
	return true
}

// scrollOutput 输出区滚动：dir>0 上滚（看旧内容），dir<0 下滚
func scrollOutput(dir int) {
	step := ui.visible / 2
	if step < 1 {
		step = 1
	}
	ui.scroll += dir * step
	if ui.scroll < 0 {
		ui.scroll = 0
	}
}

// drawCmdPanel 画底部悬浮命令行面板，/ 打开时从底部滑入、ESC 收起时滑出
// 返回 true 表示本帧绘制了面板（部分高度也算），供外层判断
func drawCmdPanel(w, h int) bool {
	now := time.Now()
	var progress float64
	if ui.closing {
		el := now.Sub(ui.closeT)
		if el >= cmdAnimTime {
			ui.active = false
			ui.closing = false
			return false
		}
		progress = 1 - float64(el)/float64(cmdAnimTime)
	} else if !ui.active {
		return false
	} else {
		progress = float64(now.Sub(ui.openT)) / float64(cmdAnimTime)
		if progress > 1 {
			progress = 1
		}
	}
	// easeOutCubic：开始快、收尾慢，观感更顺
	if progress < 1 {
		e := 1 - progress
		progress = 1 - e*e*e
	}
	total := panelHeight(h)
	if total < 3 {
		return false
	}
	ph := int(progress * float64(total))
	if ph < 1 {
		return false
	}
	top := h - ph
	// 面板覆盖区一律不继承墙体真彩色
	for yy := top; yy < top+ph && yy < h; yy++ {
		for i := range colorRows[yy] {
			colorRows[yy][i] = 0
		}
	}
	borderRow := func(idx int) {
		r := rows[idx]
		for i := range r {
			if i == 0 || i == w-1 {
				r[i] = '+'
			} else {
				r[i] = '-'
			}
		}
	}
	// 顶部分隔线
	borderRow(top)
	// 底部对称分隔线：ph>=2 时也画，保证上下边框成对
	if ph >= 2 {
		borderRow(top + ph - 1)
	}
	if ph < 3 {
		return true
	}
	// 面板高度至少 3 行才有输出区/输入行/提示行的空间；
	// ph<3 时只画分隔线，防止 rows 越界
	nvis := ph - 3
	ui.visible = nvis
	totalOut := len(ui.out)
	maxScroll := totalOut - nvis
	if maxScroll < 0 {
		maxScroll = 0
	}
	if ui.scroll > maxScroll {
		ui.scroll = maxScroll
	}
	start := totalOut - nvis - ui.scroll
	if start < 0 {
		start = 0
	}
	for i := 0; i < nvis; i++ {
		r := rows[top+1+i]
		fillPanelRow(r, w, "")
	}
	for i := 0; i < nvis && start+i < totalOut; i++ {
		r := rows[top+1+i]
		fillPanelRow(r, w, ui.out[start+i])
	}
	// 输入行（在底边框上一行）
	promptRune := []rune("> ")
	full := append([]rune{}, promptRune...)
	full = append(full, ui.buf...)
	cursorRune := rune(0x2588) // █
	promptLen := len(full)
	inRow := rows[top+ph-2]
	fillPanelRow(inRow, w, string(full))
	for i := range colorRows[top+ph-2] {
		colorRows[top+ph-2][i] = 0
	}
	if promptLen < w-2 {
		inRow[1+promptLen] = cursorRune
	}
	// 底部提示行并入输入行右侧：滚动范围提示紧跟 prompt，避免单独占一行
	hint := ""
	if ui.scroll > 0 || totalOut > nvis {
		hint = fmt.Sprintf("[%d..%d/%d↑]", start+1, start+nvis, totalOut)
	}
	if hint != "" {
		hintRune := []rune(hint)
		base := 1 + promptLen + 1
		for i := 0; i < len(hintRune) && base+i < w-2; i++ {
			inRow[base+i] = hintRune[i]
		}
	}
	// 按键帮助放在顶分隔线下一行的行尾
	if ph >= total && ph >= 4 {
		keys := "PgUp/PgDn或Ctrl+U/D滚动 ↑↓历史 ESC收起"
		keysRune := []rune(keys)
		r := rows[top+1]
		base := w - 2 - len(keysRune)
		if base < 2 {
			base = 2
		}
		for i := 0; i < len(keysRune) && base+i < w-1; i++ {
			r[base+i] = keysRune[i]
		}
	}
	return true
}

func fillPanelRow(r []rune, w int, text string) {
	for i := range r {
		r[i] = ' '
	}
	if w <= 2 {
		return
	}
	r[0] = '|'
	r[w-1] = '|'
	t := []rune(text)
	for i := 0; i < len(t) && 1+i < w-1; i++ {
		r[1+i] = t[i]
	}
}

// ---- 内置命令 ----

var wallStyle int

var wallPalettes = [][5]rune{
	{'|', '_', '/', '\\', '·'},
	{'│', '─', '╱', '╲', '┼'},
	{'█', '█', '█', '█', '█'},
}

var spinOn bool

var openMode bool
var mazeSize int

type builtinFunc func([]string) (string, int)

var builtins = map[string]builtinFunc{
	"help":      cmdHelp,
	"clear":     cmdClear,
	"where":     cmdWhere,
	"home":      cmdHome,
	"tp":        cmdTP,
	"seed":      cmdSeed,
	"fov":       cmdFov,
	"height":    cmdHeight,
	"reset":     cmdReset,
	"spin":      cmdSpin,
	"flip":      cmdFlip,
	"party":     cmdParty,
	"floorinfo": cmdFloorInfo,
	"space":     cmdSpace,
	"save":      cmdSave,
	"load":      cmdLoad,
	"file":      cmdFile,
	"door":      cmdDoor,
	"bookmark":  cmdBookmark,
	"goto":      cmdGoto,
	"elevator":  cmdElevator,
	"edit":      cmdEdit,
	"img":       cmdImg,
	"video":     cmdVideo,
}

func cmdHelp(_ []string) (string, int) {
	return `可用命令：
  help           显示帮助     clear 清空历史
  where          位置/楼层/朝向/高度/视野
  home           回出生点     tp x y 传送
  seed           显示地图种子
  fov [值|wide|normal|+|-]  视野 0.5~5
  height [值|high|low|ground|+|-]
  reset          重置俯仰与高度
  spin [on|off]  自动旋转视角   flip 上下翻转
  party          更换墙体样式   floorinfo 楼层信息
  space          200x200 平地 / close 回迷宫
  door [x y|open|close [all]|list]  门
  bookmark [名]  标记当前位置    goto [名] 传送书签
  elevator [楼层|home|list]  电梯换层
  edit           进入/退出编辑模式（Q退出）
  save [名]      存为 ~/ 名.rmap (RAMAP)
  load [名]      载入 ~/ 名.rmap
  file <路径>    识别文件是否为本游戏存档
  img load <文件> [x y] 彩色ASCII(.png/.jpg/.gif)，缺省就近找墙
  img list|remove <id>   list列出~/图片文件；贴图墙小地图显示I
  video play <文件> [x y] mp4转真彩带音轨(需ffmpeg)/gif/图/文本
  video stop|list|remove <id> 停止含音轨；list列出~/ gif/mp4
  |  管道   && 和   || 或   ; 依次   引号 "  '`, 0
}

func cmdClear(_ []string) (string, int) {
	ui.out = nil
	return "", 0
}

func dirText(a float64) string {
	// 0 度朝 +y；每 90° 一个方位
	deg := int(math.Mod(a*(180/math.Pi)+720, 360))
	switch {
	case deg >= 315 || deg < 45:
		return "北"
	case deg < 135:
		return "东"
	case deg < 225:
		return "南"
	default:
		return "西"
	}
}

func cmdWhere(_ []string) (string, int) {
	return fmt.Sprintf("位置 (%d,%d)  楼层 %d  朝向 %s (%.1f°)  俯仰 %.2f  高度 %.2f  视野 %.2f",
		int(player.x), int(player.y), currentFloor, dirText(player.angle), player.angle*180/math.Pi,
		player.pitch, player.camHeight, settings.FOV), 0
}

func cmdHome(_ []string) (string, int) {
	player.x = float64(gameMap.startX)
	player.y = float64(gameMap.startY)
	return "已传送回出生点", 0
}

func cmdTP(args []string) (string, int) {
	if len(args) < 2 {
		return "用法: tp x y", 1
	}
	x, err1 := strconv.Atoi(args[0])
	y, err2 := strconv.Atoi(args[1])
	if err1 != nil || err2 != nil {
		return "坐标必须是整数", 1
	}
	if x < 1 || y < 1 || x >= gameMap.width || y >= gameMap.height {
		return fmt.Sprintf("(%d,%d) 超出地图范围 %dx%d", x, y, gameMap.width, gameMap.height), 1
	}
	if gameMap.isWall(x, y) {
		return fmt.Sprintf("(%d,%d) 是墙，不能传送", x, y), 1
	}
	player.x = float64(x) + 0.5
	player.y = float64(y) + 0.5
	return fmt.Sprintf("已传送到 (%d,%d)", x, y), 0
}

func cmdSeed(_ []string) (string, int) {
	return fmt.Sprintf("种子: %d", gameMap.seed), 0
}

func cmdFov(args []string) (string, int) {
	if len(args) == 0 {
		return fmt.Sprintf("视野 %.2f", settings.FOV), 0
	}
	switch args[0] {
	case "wide":
		settings.FOV = 4.0
	case "normal":
		settings.FOV = 1.0
	case "+":
		settings.FOV = clampF(settings.FOV+0.2, 0.5, 5.0)
	case "-":
		settings.FOV = clampF(settings.FOV-0.2, 0.5, 5.0)
	default:
		v, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return "用法: fov [数值|wide|normal|+| -]", 1
		}
		settings.FOV = clampF(v, 0.5, 5.0)
	}
	return fmt.Sprintf("视野 %.2f", settings.FOV), 0
}

func cmdHeight(args []string) (string, int) {
	if len(args) == 0 {
		return fmt.Sprintf("高度 %.2f", player.camHeight), 0
	}
	switch args[0] {
	case "high":
		player.camHeight = 2.0
	case "low":
		player.camHeight = -1.0
	case "ground":
		player.camHeight = 0.0
	case "+":
		player.camHeight = clampF(player.camHeight+0.2, -5.0, 5.0)
	case "-":
		player.camHeight = clampF(player.camHeight-0.2, -5.0, 5.0)
	default:
		v, err := strconv.ParseFloat(args[0], 64)
		if err != nil {
			return "用法: height [数值|high|low|ground|+| -]", 1
		}
		player.camHeight = clampF(v, -5.0, 5.0)
	}
	return fmt.Sprintf("高度 %.2f", player.camHeight), 0
}

func cmdReset(_ []string) (string, int) {
	player.pitch = 0
	player.camHeight = 0
	return "已重置俯仰与高度", 0
}

func cmdSpin(args []string) (string, int) {
	if len(args) > 0 && (args[0] == "off" || args[0] == "0") {
		spinOn = false
		return "已停止自动旋转", 0
	}
	spinOn = true
	return "自动旋转中（游戏内按任意键停止）", 0
}

func cmdFlip(_ []string) (string, int) {
	player.pitch = -player.pitch
	return fmt.Sprintf("已上下翻转，俯仰 %.2f", player.pitch), 0
}

func cmdParty(_ []string) (string, int) {
	wallStyle = rand.Intn(len(wallPalettes))
	return fmt.Sprintf("墙体样式已更换为 #%d", wallStyle), 0
}

func cmdFloorInfo(_ []string) (string, int) {
	if openMode {
		return fmt.Sprintf("当前楼层 %d：开放平地 (200×200)，自由漫步", currentFloor), 0
	}
	return fmt.Sprintf("当前楼层 %d：迷宫 (%dx%d，种子 %d)，所有通路互相连通", currentFloor, gameMap.width, gameMap.height, gameMap.seed), 0
}

func cmdSpace(args []string) (string, int) {
	if len(args) > 0 && args[0] == "close" {
		if !openMode {
			return "当前本来就不是平地模式", 0
		}
		openMode = false
		gameMap.regen(mazeSize)
		return fmt.Sprintf("已返回迷宫模式 (%dx%d)", gameMap.width, gameMap.height), 0
	}
	if openMode {
		return "已在平地模式", 0
	}
	mazeSize = gameMap.width
	gameMap.makeFlat(200)
	openMode = true
	return "已进入平地模式 200×200（space close 返回迷宫）", 0
}
