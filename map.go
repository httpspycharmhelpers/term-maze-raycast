package main

import (
	"math"
	"math/rand"
	"time"
)

type Map struct {
	width  int
	height int
	grid   []byte

	startX int
	startY int
	exitX  int
	exitY  int
}

func (m *Map) regen() {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	w, h := 31, 31
	if w%2 == 0 {
		w++
	}
	if h%2 == 0 {
		h++
	}
	m.width, m.height = w, h

	grid := make([]byte, w*h)
	for i := range grid {
		grid[i] = '#'
	}

	type cell struct{ x, y int }
	stack := []cell{{1, 1}}
	grid[1*w+1] = ' '
	dirs := [4][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}}

	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		var nxt []cell
		for _, d := range dirs {
			nx, ny := cur.x+d[0], cur.y+d[1]
			if nx > 0 && nx < w-1 && ny > 0 && ny < h-1 && grid[ny*w+nx] == '#' {
				nxt = append(nxt, cell{nx, ny})
			}
		}
		if len(nxt) == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		nc := nxt[rng.Intn(len(nxt))]
		grid[(cur.y+(nc.y-cur.y)/2)*w+(cur.x+(nc.x-cur.x)/2)] = ' '
		grid[nc.y*w+nc.x] = ' '
		stack = append(stack, nc)
	}

	for i := 0; i < w*h/20; i++ {
		x := rng.Intn(w-4) + 2
		y := rng.Intn(h-4) + 2
		grid[y*w+x] = ' '
	}

	m.grid = grid
	m.startX, m.startY = 1, 1
	m.exitX, m.exitY = w-2, h-2
	grid[m.startY*w+m.startX] = 's'
	grid[m.exitY*w+m.exitX] = 'e'
}

func (m *Map) isWall(x, y int) bool {
	if x < 0 || x >= m.width || y < 0 || y >= m.height {
		return true
	}
	return m.grid[y*m.width+x] == '#'
}

func (m *Map) cell(x, y int) byte {
	if x < 0 || x >= m.width || y < 0 || y >= m.height {
		return '#'
	}
	return m.grid[y*m.width+x]
}

func clampF(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
