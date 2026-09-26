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
	jumpY     float64 // 跳跃高度偏移（格）
	jumpV     float64 // 垂直速度（格/秒）
}

func (player *Player) init(sx, sy float64) {
	player.x = sx
	player.y = sy
	player.angle = 0.0
	player.pitch = 0.0
	player.camHeight = 0.0
	player.jumpY = 0
	player.jumpV = 0
}

const (
	moveStep   = 0.5                    // 每按一次「前进/平移」立即移动的格数
	rotStep    = 0.15                   // 每按一次「转向」立即转动的弧度
	glideTime  = 250 * time.Millisecond // 按下一次后的平滑滑动时长
	glideSpeed = 2.5                    // 松开后仍短缓移动的速率，格/秒
	glideRot   = 0.6                    // 松开后仍短缓转动的速率，弧度/秒
	heightStep = 0.15
	pitchSpeed = 0.05
	planeScale = 0.66
	jumpVel    = 5.5 // 起跳垂直速度，格/秒
	jumpG      = 20  // 重力加速度
)

var heldMu sync.Mutex
var pressCount = make(map[rune]uint32)
var seenPress = make(map[rune]uint32)
var glideStart = make(map[rune]time.Time)

// jumpNext 由 move() 的空格键触发，updatePlayer 每帧消费一次并接上重力
var jumpNext bool

// updatePlayer 由主循环每帧调用：每个新按键立即走一步（即时响应），
// 之后短时间按恒定速率平滑滑动，连续按住则无缝衔接成连续移动
func updatePlayer(dt float64) {
	dirX := math.Sin(player.angle)
	dirY := math.Cos(player.angle)
	planeX := math.Cos(player.angle) * planeScale
	planeY := -math.Sin(player.angle) * planeScale

	heldMu.Lock()
	var dx, dy, dAngle float64
	move := func(r rune, sx, sy float64, vel float64) {
		if n := pressCount[r] - seenPress[r]; n > 0 {
			seenPress[r] = pressCount[r]
			dx += sx * float64(n) * moveStep
			dy += sy * float64(n) * moveStep
			glideStart[r] = time.Now()
		} else if t, ok := glideStart[r]; ok && time.Since(t) < glideTime {
			dx += sx * vel * dt
			dy += sy * vel * dt
		}
	}
	turn := func(r rune, sign float64) {
		if n := pressCount[r] - seenPress[r]; n > 0 {
			seenPress[r] = pressCount[r]
			dAngle += sign * float64(n) * rotStep
			glideStart[r] = time.Now()
		} else if t, ok := glideStart[r]; ok && time.Since(t) < glideTime {
			dAngle += sign * glideRot * dt
		}
	}
	move('2', dirX, dirY, glideSpeed)
	move('8', -dirX, -dirY, glideSpeed)
	move('4', -planeX, -planeY, glideSpeed)
	move('6', planeX, planeY, glideSpeed)
	turn('1', -1)
	turn('3', +1)
	// 空格 = 跳跃（跳跃中不可再起跳）
	if jumpNext {
		jumpNext = false
		if player.jumpY <= 0 && player.jumpV <= 0 {
			player.jumpV = jumpVel
		}
	}
	// 重力积分
	if player.jumpV != 0 || player.jumpY > 0 {
		player.jumpV -= jumpG * dt
		player.jumpY += player.jumpV * dt
		if player.jumpY < 0 {
			player.jumpY = 0
			player.jumpV = 0
		}
	}
	heldMu.Unlock()

	player.angle += dAngle

	if spinOn {
		player.angle += 0.25 * dt
	}

	nx := player.x + dx
	ny := player.y + dy
	if !isBlocked(int(nx), int(player.y)) {
		player.x = nx
	}
	if !isBlocked(int(player.x), int(ny)) {
		player.y = ny
	}
}

func (player *Player) move() {
	if err := keyboard.Open(); err != nil {
		log.Fatal(err)
	}
	defer keyboard.Close()

	for {
		char, keyCode, err := keyboard.GetKey()
		if err != nil {
			log.Fatal(err)
		}

		// 命令模式：按键全部交给命令行处理
		if ui.active {
			handleCmdKey(char, keyCode)
			continue
		}

		// 编辑模式：光标类按键交给编辑窗口
		if editMode {
			if handleEditKey(char, keyCode) {
				continue
			}
		}

		// 游戏内任意键停止自动旋转
		if spinOn && char != 0 {
			spinOn = false
		}

		switch char {
		case 'q':
			fmt.Println("Exiting...")
			exitChan <- true
			return

		case '\x1b':
			if settings.showMinimap {
				settings.showMinimap = false
			} else {
				fmt.Println("Exiting...")
				exitChan <- true
				return
			}

		case '/':
			ui.buf = ui.buf[:0]
			ui.active = true
			ui.closing = false
			ui.openT = time.Now()
			ui.histIdx = len(ui.hist)

		case '1', '3', '2', '8', '4', '6':
			// 记一次按键：主循环按帧立即走一步并衔接平滑滑动
			heldMu.Lock()
			pressCount[char]++
			glideStart[char] = time.Now()
			heldMu.Unlock()

		case ' ':
			jumpNext = true

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
