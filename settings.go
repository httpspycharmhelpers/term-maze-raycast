package main

import "time"

type Settings struct {
	FOV          float64
	showMinimap  bool
	frameTime    time.Duration
	drawInterval time.Duration
}

func (s *Settings) init() {
	s.FOV = 1.0
	s.showMinimap = false
	// 输入更新与重绘节拍按平台取默认值：
	// Linux 桌面走高帧率，Termux 走保守节流
	s.frameTime = defaultFrameTime()
	s.drawInterval = defaultDrawInterval()
}
