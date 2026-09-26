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
	// 输入更新节拍 ~60Hz：手感不受影响
	s.frameTime = time.Millisecond * 15
	// 重绘上限 ~40fps + 增量绘屏（无变化不写字节），
	// 终端不会被每秒数百 KB 的整屏刷新淹没 → 画面更稳更顺
	s.drawInterval = time.Millisecond * 25
}
