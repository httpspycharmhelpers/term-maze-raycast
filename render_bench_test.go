package main

import (
	"os"
	"testing"
)

func BenchmarkRenderFrame(b *testing.B) {
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err == nil {
		old := os.Stdout
		os.Stdout = devnull
		defer func() { os.Stdout = old; devnull.Close() }()
	}
	gameMap.regen(301)
	screen.width, screen.height = 200, 50
	player.x, player.y = float64(gameMap.startX)+0.5, float64(gameMap.startY)+0.5
	settings.init()
	render()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		player.angle = float64(i%360) * 3.141592653589793 / 180
		render()
	}
}

func BenchmarkRegen501(b *testing.B) {
	for i := 0; i < b.N; i++ {
		gameMap.regen(501)
	}
}
