package main

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
)

type Player struct {
	x         float64
	y         float64
	angle     float64
	pitch     float64
	camHeight float64
}

func (player *Player) init(sx, sy float64) {
	player.x = sx
	player.y = sy
	player.angle = 0.0
	player.pitch = 0.0
	player.camHeight = 0.0
}

const (
	moveSpeed  = 4.0 // 格/秒
	rotSpeed   = 0.7 // 弧度/秒
	heightStep = 0.15
	pitchSpeed = 0.05
	planeScale = 0.66
	// 按键被视作“按住”的窗口时长：窗口内的每帧都按时间增量平滑移动，
	// 兼容没有按键自动重复的终端（安卓/Termux 常见），单次点按也有一段平滑滑动
	holdWindow = 400 * time.Millisecond
)

var heldMu sync.Mutex
var heldAt = make(map[rune]time.Time)

func keyHeld(r rune) bool {
	heldMu.Lock()
	defer heldMu.Unlock()
	t, ok := heldAt[r]
	return ok && time.Since(t) < holdWindow
}

// updatePlayer 由主循环每帧调用，按时间增量连续驱动移动与转向
func updatePlayer(dt float64) {
	dirX := math.Sin(player.angle)
	dirY := math.Cos(player.angle)
	planeX := math.Cos(player.angle) * planeScale
	planeY := -math.Sin(player.angle) * planeScale

	if keyHeld('1') {
		player.angle -= rotSpeed * dt
	}
	if keyHeld('3') {
		player.angle += rotSpeed * dt
	}

	move := moveSpeed * dt
	if keyHeld('2') {
		nx := player.x + dirX*move
		ny := player.y + dirY*move
		if !gameMap.isWall(int(nx), int(player.y)) {
			player.x = nx
		}
		if !gameMap.isWall(int(player.x), int(ny)) {
			player.y = ny
		}
	}
	if keyHeld('8') {
		nx := player.x - dirX*move
		ny := player.y - dirY*move
		if !gameMap.isWall(int(nx), int(player.y)) {
			player.x = nx
		}
		if !gameMap.isWall(int(player.x), int(ny)) {
			player.y = ny
		}
	}
	if keyHeld('4') {
		nx := player.x - planeX*move
		ny := player.y - planeY*move
		if !gameMap.isWall(int(nx), int(player.y)) {
			player.x = nx
		}
		if !gameMap.isWall(int(player.x), int(ny)) {
			player.y = ny
		}
	}
	if keyHeld('6') {
		nx := player.x + planeX*move
		ny := player.y + planeY*move
		if !gameMap.isWall(int(nx), int(player.y)) {
			player.x = nx
		}
		if !gameMap.isWall(int(player.x), int(ny)) {
			player.y = ny
		}
	}
}

func (player *Player) move() {
	if err := keyboard.Open(); err != nil {
		log.Fatal(err)
	}
	defer keyboard.Close()

	for {
		char, _, err := keyboard.GetKey()
		if err != nil {
			log.Fatal(err)
		}

		switch char {
		case 'q', '\x1b':
			fmt.Println("Exiting...")
			exitChan <- true
			return

		case '1', '3', '2', '8', '4', '6':
			// 记录按下时刻，由主循环按帧持续时间平滑移动/转向
			heldMu.Lock()
			heldAt[char] = time.Now()
			heldMu.Unlock()

		case '5':
			player.camHeight = clampF(player.camHeight+heightStep, -2.0, 2.0)

		case '0':
			player.camHeight = clampF(player.camHeight-heightStep, -2.0, 2.0)

		case '7':
			player.pitch = clampF(player.pitch+pitchSpeed, -0.8, 0.8)

		case '9':
			player.pitch = clampF(player.pitch-pitchSpeed, -0.8, 0.8)

		case '+':
			settings.FOV = clampF(settings.FOV+0.1, 0.3, 2.0)
			setStatus(fmt.Sprintf("视野 %.1f", settings.FOV))

		case '-':
			settings.FOV = clampF(settings.FOV-0.1, 0.3, 2.0)
			setStatus(fmt.Sprintf("视野 %.1f", settings.FOV))

		case '%':
			settings.showMinimap = !settings.showMinimap
			if settings.showMinimap {
				setStatus("小地图: 开")
			} else {
				setStatus("小地图: 关")
			}
		}
	}
}
