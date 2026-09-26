package main

import (
	"fmt"
	"math"
	"os"
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
	t0 := time.Now()
	gameMap.regen()
	genMs := time.Since(t0).Milliseconds()

	settings.init()
	screen.init()
	player.init(float64(gameMap.startX), float64(gameMap.startY))
	setStatus(fmt.Sprintf("迷宫 %dx%d 生成 %dms  按键:2前 8后 4/6平移 1/3转向 5/0视高 7/9俯仰 +/-视野 %% 小地图 q退出", gameMap.width, gameMap.height, genMs))

	go player.move()

	fmt.Printf("\x1b[?1049h\x1b[2J\x1b[?25l")
	defer fmt.Printf("\x1b[?25h\x1b[?1049l")

	var lastFpsAt = time.Now()
	var lastTermCheck = time.Now()
	frameCount := 0

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

		frameCount++
		if now.Sub(lastFpsAt) >= time.Second {
			fps := float64(frameCount) / now.Sub(lastFpsAt).Seconds()
			frameCount = 0
			lastFpsAt = now
			if now.UnixNano() >= statusUntil {
				setStatus(fmt.Sprintf("FPS %.0f  位置(%d,%d)  迷宫 %dx%d", fps, int(player.y), int(player.x), gameMap.width, gameMap.height))
			}
		}

		render()

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
			if grid[mapY*mw+mapX] == '#' {
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
				switch {
				case isVertical && isHorizontal:
					ch = '·'
				case isVertical:
					ch = '|'
				case isHorizontal:
					switch {
					case player.pitch > 0.05:
						ch = '/'
					case player.pitch < -0.05:
						ch = '\\'
					default:
						ch = '_'
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
	if time.Now().UnixNano() < statusUntil {
		drawStatus(w, h)
	}

	frameBuf = frameBuf[:0]
	if resized {
		frameBuf = append(frameBuf, '\x1b', '[', '2', 'J')
	}
	frameBuf = append(frameBuf, '\x1b', '[', 'H')
	for y := 0; y < h; y++ {
		r := rows[y]
		for x := 0; x < w; x++ {
			frameBuf = appendRune(frameBuf, r[x])
		}
		if y < h-1 {
			frameBuf = append(frameBuf, '\n')
		}
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
