package main

import "time"

type Settings struct {
	FOV         float64
	showMinimap bool
	frameTime   time.Duration
}

func (s *Settings) init() {
	s.FOV = 1.0
	s.showMinimap = false
	// 固定帧节拍 ~66fps：写屏压力比原来(4ms≈250fps)降低约4倍，消除终端抖动/卡顿
	s.frameTime = time.Millisecond * 15
}
