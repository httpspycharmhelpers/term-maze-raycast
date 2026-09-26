package main

type Screen struct {
	height int
	width  int
}

func (s *Screen) init() {
	s.height = 40
	s.width = 120
}
