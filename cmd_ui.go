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
}

var ui commandUI

const cmdPanelRows = 10
const cmdAnimTime = 120 * time.Millisecond

func cmdPrint(s string) {
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
	if h < cmdPanelRows+2 {
		return false
	}
	ph := int(progress * float64(cmdPanelRows))
	if ph < 1 {
		return false
	}
	if ph > cmdPanelRows {
		ph = cmdPanelRows
	}
	top := h - ph
	// 顶部分隔线
	topRow := rows[top]
	for i := range topRow {
		if i == 0 || i == w-1 {
			topRow[i] = '+'
		} else {
			topRow[i] = '-'
		}
	}
	// 历史行
	hist := ui.out
	if n := ph - 3; len(hist) > n && n > 0 {
		hist = hist[len(hist)-n:]
	}
	for i := 0; i < ph-3; i++ {
		r := rows[top+1+i]
		fillPanelRow(r, w, "")
	}
	for i, l := range hist {
		r := rows[top+1+i]
		fillPanelRow(r, w, l)
	}
	// 输入行
	promptRune := []rune("> ")
	full := append([]rune{}, promptRune...)
	full = append(full, ui.buf...)
	cursorRune := rune(0x2588) // █
	promptLen := len(full)
	inRow := rows[top+ph-2]
	fillPanelRow(inRow, w, string(full))
	if promptLen < w-2 {
		inRow[1+promptLen] = cursorRune
	}
	// 底部提示行：面板完全展开后才显示
	if ph >= cmdPanelRows {
		hint := "| 管道 && ; || 引号   ↑↓历史  ESC收起"
		hintRune := []rune(hint)
		hRow := rows[top+ph-1]
		fillPanelRow(hRow, w, "")
		for i := 0; i < len(hintRune) && 1+i < w-1; i++ {
			hRow[1+i] = hintRune[i]
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
