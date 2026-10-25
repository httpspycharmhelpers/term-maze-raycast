//go:build !termux

package main

import (
	"runtime"
	"time"
)

func platformName() string { return "Linux" }

func audioPlayers() [][]string {
	return [][]string{
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "-vn"},
		{"termux-media-player", "play"},
	}
}

func maxWorkers() int {
	n := runtime.GOMAXPROCS(0)
	if n > 16 {
		n = 16
	}
	if n < 1 {
		n = 1
	}
	return n
}

func defaultFrameTime() time.Duration { return 8 * time.Millisecond }

func defaultDrawInterval() time.Duration { return 16 * time.Millisecond }

// ffmpegHWArgs 让桌面 ffmpeg 优先用显卡硬解，失败会自动回退软解
func ffmpegHWArgs() []string { return []string{"-hwaccel", "auto"} }
