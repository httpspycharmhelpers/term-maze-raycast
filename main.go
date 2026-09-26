package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

var exitChan = make(chan bool)

var gameMap Map
var player Player
var settings Settings
var screen Screen

var statusMsg string
var statusUntil int64

// 渲染缓冲跨帧复用，避免每帧分配，提升性能
var colStart, colEnd []int
var colWallX []float64
var rows [][]rune
var prevRows [][]rune
var rowDirty []bool
var frameBuf = newFrameWriter()

func termSize() (int, int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0
	}
	return int(ws.Col), int(ws.Row)
}

func setStatus(s string) {
	statusMsg = s
	statusUntil = time.Now().UnixNano() + int64(1500*time.Millisecond)
}

func main() {
	// 保存进入 raw 模式前的终端设置（回显、行模式），退出时必须还原，
	// 否则退出后终端不回显输入（练过 keyboard 库后没有正确恢复 termios）
	stdinFD := int(os.Stdin.Fd())
	origTermios, _ := unix.IoctlGetTermios(stdinFD, unix.TCGETS)
	restoreTermios := func() {
		if origTermios != nil {
			unix.IoctlSetTermios(stdinFD, unix.TCSETS, origTermios)
		}
	}
	defer restoreTermios()

	// Ctrl-C 兜底：进程被信号杀掉时 defer 不执行，必须在此恢复终端
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		restoreTermios()
		fmt.Println("\r\n已按 Ctrl-C 退出")
		os.Exit(0)
	}()

	t0 := time.Now()
	size := 1000
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil && n > 0 {
			size = n
		}
	}
	gameMap.regen(size)
	genMs := time.Since(t0).Milliseconds()

	settings.init()
	screen.init()
	player.init(float64(gameMap.startX), float64(gameMap.startY))
	setStatus(fmt.Sprintf("迷宫 %dx%d 生成 %dms  按键:2前 8后 4/6平移 1/3转向 5/0视高 7/9俯仰 +/-视野 %% 小地图 q退出", gameMap.width, gameMap.height, genMs))

	go player.move()

	fmt.Printf("\x1b[?1049h\x1b[2J\x1b[?25l")
	defer fmt.Printf("\x1b[?25h\x1b[?1049l")

	var lastTermCheck = time.Now()
	var lastFrame = time.Now()
	var lastDraw = time.Now()

	for {
		select {
		case <-exitChan:
			fmt.Println("Exiting game loop...")
			return
		default:
		}

		now := time.Now()

		// 终端尺寸只每 100ms 采样一次并保持到变化稳定，避免瞬间抖动/弹出软键盘
		// 导致尺寸来回跳，从而引起画面跳动
		if now.Sub(lastTermCheck) >= 100*time.Millisecond {
			lastTermCheck = now
			if cw, ch := termSize(); cw > 0 && ch > 0 {
				if cw != screen.width || ch != screen.height {
					screen.width, screen.height = cw, ch
					if screen.width < 20 {
						screen.width = 20
					}
					if screen.height < 10 {
						screen.height = 10
					}
					setStatus(fmt.Sprintf("终端已缩放: %dx%d", screen.width, screen.height))
				}
			}
		}

		// 时间增量驱动的连续移动：帧间隔变化不影响手感，
		// 按住移动/转向键即平滑连续运动，不依赖终端按键自动重复
		dt := now.Sub(lastFrame).Seconds()
		if dt > 0.05 {
			dt = 0.05
		}
		lastFrame = now
		updatePlayer(dt)

		// 增量绘屏 + 重绘节流：画面无变化时不写任何字节；有变化也最多 ~40fps
		// 命令面板滑入/滑出动画期间不节流，保证收放平滑
		if ui.active || ui.closing || time.Since(lastDraw) >= settings.drawInterval {
			render()
			lastDraw = time.Now()
		}

		// 固定帧节拍：一帧超时则不睡（追赶），否则睡到 ~66fps，
		// 帧间隔恒定 → 平滑无卡顿，也避免高帧率全屏重绘导致的抖动
		if sleep := settings.frameTime - time.Since(now); sleep > 0 {
			time.Sleep(sleep)
		}
	}
}

func newFrameWriter() []byte {
	return make([]byte, 0, 64<<10)
}

func ensureBuffers(w, h int) (resized bool) {
	if len(rows) != h || (h > 0 && len(rows[0]) != w) || len(colStart) != w {
		rows = make([][]rune, h)
		for y := 0; y < h; y++ {
			rows[y] = make([]rune, w)
		}
		prevRows = make([][]rune, h)
		for y := 0; y < h; y++ {
			prevRows[y] = make([]rune, w)
		}
		rowDirty = make([]bool, h)
		colStart = make([]int, w)
		colEnd = make([]int, w)
		colWallX = make([]float64, w)
		return true
	}
	return false
}

func render() {
	w, h := screen.width, screen.height
	resized := ensureBuffers(w, h)

	dirX := math.Sin(player.angle)
	dirY := math.Cos(player.angle)
	planeX := math.Cos(player.angle) * planeScale
	planeY := -math.Sin(player.angle) * planeScale

	pitchOffset := int(float64(h) * player.pitch * 0.4)
	heightShift := int(player.camHeight * 6)
	horizon := h/2 + pitchOffset + heightShift

	grid := gameMap.grid
	mw, mh := gameMap.width, gameMap.height
	px, py := player.x, player.y
	rebuildDoorBlock()
	block := doorBlock

	const maxRaySteps = 512

	for x := 0; x < w; x++ {
		cameraX := (2.0*float64(x)/float64(w) - 1.0) * settings.FOV
		rayDirX := dirX + planeX*cameraX
		rayDirY := dirY + planeY*cameraX

		mapX, mapY := int(px), int(py)

		var deltaDistX, deltaDistY, sideDistX, sideDistY float64
		var stepX, stepY int
		side := 0

		if rayDirX == 0 {
			deltaDistX = 1e30
		} else {
			deltaDistX = math.Abs(1.0 / rayDirX)
		}
		if rayDirY == 0 {
			deltaDistY = 1e30
		} else {
			deltaDistY = math.Abs(1.0 / rayDirY)
		}

		if rayDirX < 0 {
			stepX, sideDistX = -1, (px-float64(mapX))*deltaDistX
		} else {
			stepX, sideDistX = 1, (float64(mapX)+1.0-px)*deltaDistX
		}
		if rayDirY < 0 {
			stepY, sideDistY = -1, (py-float64(mapY))*deltaDistY
		} else {
			stepY, sideDistY = 1, (float64(mapY)+1.0-py)*deltaDistY
		}

		for steps := 0; steps < maxRaySteps; steps++ {
			if sideDistX < sideDistY {
				sideDistX += deltaDistX
				mapX += stepX
				side = 0
			} else {
				sideDistY += deltaDistY
				mapY += stepY
				side = 1
			}
			if mapX < 0 || mapX >= mw || mapY < 0 || mapY >= mh {
				break
			}
			if grid[mapY*mw+mapX] == '#' || block[mapY*mw+mapX] != 0 {
				break
			}
		}

		perpWallDist := sideDistX - deltaDistX
		if side == 1 {
			perpWallDist = sideDistY - deltaDistY
		}
		if perpWallDist <= 0 {
			perpWallDist = 0.001
		}

		var wallX float64
		if side == 0 {
			wallX = py + perpWallDist*rayDirY
		} else {
			wallX = px + perpWallDist*rayDirX
		}
		wallX -= math.Floor(wallX)

		lineHeight := int(float64(h) / perpWallDist)
		drawStart := -lineHeight/2 + h/2 + pitchOffset + heightShift
		if drawStart < 0 {
			drawStart = 0
		}
		drawEnd := lineHeight/2 + h/2 + pitchOffset + heightShift
		if drawEnd >= h {
			drawEnd = h - 1
		}

		colStart[x] = drawStart
		colEnd[x] = drawEnd
		colWallX[x] = wallX
	}

	for y := 0; y < h; y++ {
		r := rows[y]
		for x := 0; x < w; x++ {
			ds, de := colStart[x], colEnd[x]
			ch := ' '
			if y >= ds && y <= de {
				wx := colWallX[x]
				edge := wx
				if edge > 0.5 {
					edge = 1.0 - edge
				}
				isVertical := edge < 0.08
				isHorizontal := y == ds || y == de
				pal := wallPalettes[wallStyle]
				switch {
				case isVertical && isHorizontal:
					ch = pal[4]
				case isVertical:
					ch = pal[0]
				case isHorizontal:
					switch {
					case player.pitch > 0.05:
						ch = pal[2]
					case player.pitch < -0.05:
						ch = pal[3]
					default:
						ch = pal[1]
					}
				}
			} else if y >= horizon {
				ch = '.'
			}
			r[x] = ch
		}
	}

	if settings.showMinimap {
		drawMinimap(w, h)
	}
	if editMode {
		drawEditWindow(w, h)
	}
	if !ui.active && time.Now().UnixNano() < statusUntil {
		drawStatus(w, h)
	}
	drawCmdPanel(w, h)

	frameBuf = frameBuf[:0]
	if resized {
		frameBuf = append(frameBuf, '\x1b', '[', '2', 'J')
	}
	anyChange := resized
	for y := 0; y < h; y++ {
		r, p := rows[y], prevRows[y]
		d := false
		for x := 0; x < w; x++ {
			if r[x] != p[x] {
				d = true
				break
			}
		}
		rowDirty[y] = d
		if d {
			anyChange = true
		}
	}
	if !anyChange {
		return
	}

	// 只重写发生变化的行，终端不被整屏刷新淹没
	for y := 0; y < h; y++ {
		if !rowDirty[y] {
			continue
		}
		frameBuf = append(frameBuf, '\x1b', '[')
		frameBuf = append(frameBuf, strconv.Itoa(y+1)...)
		frameBuf = append(frameBuf, ';', '1', 'H')
		r := rows[y]
		for x := 0; x < w; x++ {
			frameBuf = appendRune(frameBuf, r[x])
		}
		copy(prevRows[y], r)
	}
	os.Stdout.Write(frameBuf)
}

func appendRune(buf []byte, r rune) []byte {
	if r < 0x80 {
		return append(buf, byte(r))
	}
	return append(buf, string(r)...)
}

// drawMinimap 在屏幕左上角占 1/4 绘制玩家周围区域的小地图：
// 四周用 | 围边，# 表示墙壁，P 表示玩家，空格表示空地
func drawMinimap(w, h int) {
	rw, rh := w/2, h/2
	if rw < 8 || rh < 8 {
		return
	}
	grid := gameMap.grid
	mw, mh := gameMap.width, gameMap.height

	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			if y == 0 || y == rh-1 || x == 0 || x == rw-1 {
				rows[y][x] = '|'
				continue
			}
			mapx := int(player.x) + (x - rw/2)
			mapy := int(player.y) + (y - rh/2)
			if mapx < 0 || mapx >= mw || mapy < 0 || mapy >= mh {
				rows[y][x] = ' '
				continue
			}
			if doorIdxAt(mapx, mapy) >= 0 {
				rows[y][x] = 'D'
				continue
			}
			if grid[mapy*mw+mapx] == '#' {
				rows[y][x] = '#'
			} else {
				rows[y][x] = ' '
			}
		}
	}
	rows[rh/2][rw/2] = 'P'
}

func drawStatus(w, h int) {
	if len(statusMsg) == 0 {
		return
	}
	r := rows[h-1]
	for i := range r {
		r[i] = ' '
	}
	pos := 0
	for _, c := range statusMsg {
		if pos >= w {
			break
		}
		r[pos] = c
		pos++
	}
}
