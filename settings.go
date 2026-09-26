package main

import "time"

type Settings struct {
	FOV         float64
	showMinimap bool
	sleepTime   time.Duration
}

func (s *Settings) init() {
	s.FOV = 1.0
	s.showMinimap = false
	s.sleepTime = time.Millisecond * 4
}
