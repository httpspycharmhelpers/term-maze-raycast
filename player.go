package main

import (
	"fmt"
	"log"
	"math"

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
	moveSpeed  = 0.5
	rotSpeed   = 0.15
	heightStep = 0.15
	pitchSpeed = 0.05
	planeScale = 0.66
)

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

		dirX := math.Sin(player.angle)
		dirY := math.Cos(player.angle)
		planeX := math.Cos(player.angle) * planeScale
		planeY := -math.Sin(player.angle) * planeScale

		switch char {
		case 'q', '\x1b':
			fmt.Println("Exiting...")
			exitChan <- true
			return

		case '1':
			player.angle += rotSpeed

		case '3':
			player.angle -= rotSpeed

		case '2':
			nx := player.x + dirX*moveSpeed
			ny := player.y + dirY*moveSpeed
			if !gameMap.isWall(int(nx), int(player.y)) {
				player.x = nx
			}
			if !gameMap.isWall(int(player.x), int(ny)) {
				player.y = ny
			}

		case '8':
			nx := player.x - dirX*moveSpeed
			ny := player.y - dirY*moveSpeed
			if !gameMap.isWall(int(nx), int(player.y)) {
				player.x = nx
			}
			if !gameMap.isWall(int(player.x), int(ny)) {
				player.y = ny
			}

		case '4':
			nx := player.x - planeX*moveSpeed
			ny := player.y - planeY*moveSpeed
			if !gameMap.isWall(int(nx), int(player.y)) {
				player.x = nx
			}
			if !gameMap.isWall(int(player.x), int(ny)) {
				player.y = ny
			}

		case '6':
			nx := player.x + planeX*moveSpeed
			ny := player.y + planeY*moveSpeed
			if !gameMap.isWall(int(nx), int(player.y)) {
				player.x = nx
			}
			if !gameMap.isWall(int(player.x), int(ny)) {
				player.y = ny
			}

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
