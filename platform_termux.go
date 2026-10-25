//go:build termux

package main

import (
	"runtime"
	"time"
)

func platformName() string { return "Termux" }

func audioPlayers() [][]string {
	return [][]string{
		{"termux-media-player", "play"},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "-vn"},
	}
}

func maxWorkers() int {
	n := runtime.GOMAXPROCS(0)
	if n > 4 {
		n = 4
	}
	if n < 1 {
		n = 1
	}
	return n
}

func defaultFrameTime() time.Duration { return 15 * time.Millisecond }

func defaultDrawInterval() time.Duration { return 25 * time.Millisecond }

// ffmpegHWArgs Termux 上硬解不可用，保持软解
func ffmpegHWArgs() []string { return nil }
