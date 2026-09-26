package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

var elapsedTime float64
var exitChan = make(chan bool)

var gameMap Map
var player Player
var settings Settings
var screen Screen

var statusMsg string
var statusUntil int64

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
	gameMap.regen()
	settings.init()
	screen.init()
	player.init(float64(gameMap.startX), float64(gameMap.startY))
	setStatus("到达出口 E 生成新迷宫  按键: 2前 8后 4/6平移 1/3转向 5/0视高 7/9俯仰 +/−视野 % 小地图 q 退出")

	go player.move()

	fmt.Printf("\x1b[?1049h\x1b[2J\x1b[?25l")
	defer fmt.Printf("\x1b[?25h\x1b[?1049l")

	time_point_1 := time.Now()
	for {
		time_point_2 := time.Now()
		elapsedTime = time_point_2.Sub(time_point_1).Seconds()
		time_point_1 = time_point_2

		select {
		case <-exitChan:
			fmt.Println("Exiting game loop...")
			return
		default:
		}

		if cw, ch := termSize(); cw > 0 && ch > 0 {
			if cw != screen.width || ch != screen.height {
				screen.width, screen.height = cw, ch
				setStatus(fmt.Sprintf("终端已缩放: %dx%d", cw, ch))
			}
		}
		if screen.width < 20 {
			screen.width = 20
		}
		if screen.height < 10 {
			screen.height = 10
		}

		if gameMap.cell(int(player.x), int(player.y)) == 'e' {
			gameMap.regen()
			player.init(float64(gameMap.startX), float64(gameMap.startY))
			setStatus("到达出口! 新迷宫已生成")
		}

		render()
		time.Sleep(settings.sleepTime)
	}
}

func render() {
	w, h := screen.width, screen.height

	dirX := math.Sin(player.angle)
	dirY := math.Cos(player.angle)
	planeX := math.Cos(player.angle) * planeScale
	planeY := -math.Sin(player.angle) * planeScale

	pitchOffset := int(float64(h) * player.pitch * 0.4)
	heightShift := int(player.camHeight * 6)
	horizon := h/2 + pitchOffset + heightShift

	colStart := make([]int, w)
	colEnd := make([]int, w)
	colWallX := make([]float64, w)

	for x := 0; x < w; x++ {
		cameraX := (2.0*float64(x)/float64(w) - 1.0) * settings.FOV
		rayDirX := dirX + planeX*cameraX
		rayDirY := dirY + planeY*cameraX

		mapX := int(player.x)
		mapY := int(player.y)

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
			stepX = -1
			sideDistX = (player.x - float64(mapX)) * deltaDistX
		} else {
			stepX = 1
			sideDistX = (float64(mapX) + 1.0 - player.x) * deltaDistX
		}
		if rayDirY < 0 {
			stepY = -1
			sideDistY = (player.y - float64(mapY)) * deltaDistY
		} else {
			stepY = 1
			sideDistY = (float64(mapY) + 1.0 - player.y) * deltaDistY
		}

		for {
			if sideDistX < sideDistY {
				sideDistX += deltaDistX
				mapX += stepX
				side = 0
			} else {
				sideDistY += deltaDistY
				mapY += stepY
				side = 1
			}
			if mapX < 0 || mapX >= gameMap.width || mapY < 0 || mapY >= gameMap.height {
				break
			}
			if gameMap.cell(mapX, mapY) == '#' {
				break
			}
		}

		perpWallDist := 0.0
		if side == 0 {
			perpWallDist = sideDistX - deltaDistX
		} else {
			perpWallDist = sideDistY - deltaDistY
		}
		if perpWallDist <= 0 {
			perpWallDist = 0.001
		}

		var wallX float64
		if side == 0 {
			wallX = player.y + perpWallDist*rayDirY
		} else {
			wallX = player.x + perpWallDist*rayDirX
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

	rows := make([][]rune, h)
	for y := 0; y < h; y++ {
		r := make([]rune, w)
		for x := 0; x < w; x++ {
			ds := colStart[x]
			de := colEnd[x]
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
		rows[y] = r
	}

	if settings.showMinimap {
		drawMinimap(rows, w, h)
	}
	if time.Now().UnixNano() < statusUntil {
		drawStatus(rows, w, h)
	}

	var out strings.Builder
	out.WriteString("\x1b[H")
	for y := 0; y < h; y++ {
		out.WriteString(string(rows[y]))
		out.WriteByte('\n')
	}
	fmt.Print(out.String())
}

func drawMinimap(rows [][]rune, w, h int) {
	rw, rh := w/2, h/2
	if rw < 8 || rh < 8 {
		return
	}

	sx := (rw - 2) / gameMap.width
	sy := (rh - 2) / gameMap.height
	if sx < 1 {
		sx = 1
	}
	if sy < 1 {
		sy = 1
	}

	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			if y == 0 || y == rh-1 || x == 0 || x == rw-1 {
				ch := '+'
				if y == 0 && x > 0 && x < rw-1 {
					ch = '-'
				} else if y == rh-1 && x > 0 && x < rw-1 {
					ch = '-'
				} else if x == 0 && y > 0 && y < rh-1 {
					ch = '|'
				} else if x == rw-1 && y > 0 && y < rh-1 {
					ch = '|'
				}
				rows[y][x] = ch
				continue
			}
			mapx := (x - 1) / sx
			mapy := (y - 1) / sy
			if mapx > gameMap.width-1 {
				mapx = gameMap.width - 1
			}
			if mapy > gameMap.height-1 {
				mapy = gameMap.height - 1
			}
			switch gameMap.cell(mapx, mapy) {
			case '#':
				rows[y][x] = '#'
			case 'e':
				rows[y][x] = 'E'
			case 's':
				rows[y][x] = 'S'
			default:
				rows[y][x] = '.'
			}
		}
	}

	px := 1 + int(player.x)*sx + sx/2
	py := 1 + int(player.y)*sy + sy/2
	if px > 0 && px < rw-1 && py > 0 && py < rh-1 {
		rows[py][px] = '@'
	}
}

func drawStatus(rows [][]rune, w, h int) {
	if len(statusMsg) == 0 {
		return
	}
	clear(rows[h-1])
	copy(rows[h-1], []rune(statusMsg))
}
