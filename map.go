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
	seed   int64
	egg    []int // 彩蛋墙：0=普通墙，1+ = eggPatterns 索引

	startX int
	startY int
}

// fillEggs 约 9% 的墙格变成随机对称花纹彩蛋墙
func (m *Map) fillEggs(rng *rand.Rand) {
	m.egg = make([]int, len(m.grid))
	for i, c := range m.grid {
		if c == '#' && rng.Float64() < 0.09 {
			m.egg[i] = 1 + rng.Intn(len(eggPatterns)-1)
		}
	}
}

// regen 生成一个自洽的完美迷宫（递归回溯），保证所有通路互相连通、
// 绝不出现被墙封死的死路或孤岛。之后再随机拆墙（braiding）打通多份回环，
// 让玩家随便逛都不会被困。尺寸自动归一为奇数。
func (m *Map) regen(size int) {
	m.regenSeed(size, time.Now().UnixNano())
}

func (m *Map) regenSeed(size int, seed int64) {
	w, h := size, size
	if w < 7 {
		w = 7
	}
	if h < 7 {
		h = 7
	}
	if w%2 == 0 {
		w++
	}
	if h%2 == 0 {
		h++
	}
	m.width, m.height = w, h
	m.seed = seed

	grid := make([]byte, w*h)
	for i := range grid {
		grid[i] = '#'
	}

	rng := rand.New(rand.NewSource(m.seed))

	// 递归回溯（迭代实现）：从 (1,1) 出发，以 2 格步长打通所有奇数格棋盘格
	stack := make([]int, 1, w*h/8)
	stack[0] = 1*w + 1
	grid[1*w+1] = ' '

	dirs := [4][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}}
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		x, y := idx%w, idx/w
		var cand [4]int
		n := 0
		for _, d := range dirs {
			nx, ny := x+d[0], y+d[1]
			if nx > 0 && nx < w-1 && ny > 0 && ny < h-1 && grid[ny*w+nx] == '#' {
				cand[n] = ny*w + nx
				n++
			}
		}
		if n == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		ni := cand[rng.Intn(n)]
		ny, nx := ni/w, ni%w
		grid[(y+(ny-y)/2)*w+(x+(nx-x)/2)] = ' '
		grid[ni] = ' '
		stack = append(stack, ni)
	}

	// braiding：随机拆掉约 1/16 的内部墙，形成大量回环，消除长死胡同。
	// 只拆「四邻中至少有一格空地」的墙 → 新空格必定并入现有通路，
	// 绝不会拆出四围皆墙的孤立空格（不封死道路）。
	attempts := w * h / 16
	for i := 0; i < attempts; i++ {
		x := rng.Intn(w-4) + 2
		y := rng.Intn(h-4) + 2
		idx := y*w + x
		if grid[idx] != '#' {
			continue
		}
		if grid[(y-1)*w+x] != ' ' && grid[(y+1)*w+x] != ' ' &&
			grid[y*w+x-1] != ' ' && grid[y*w+x+1] != ' ' {
			continue
		}
		grid[idx] = ' '
	}

	m.grid = grid
	// 约 9% 墙格随机变成彩蛋花纹墙（推测用 rng 保证与迷宫同源、可复现）
	m.fillEggs(rng)
	m.startX, m.startY = 1, 1
	// 开局通道：确定性地挖开出生点北、东两侧，
	// 让玩家每次开局都能立刻前进/右移，不会顶着墙面抱怨“动不了”
	grid[m.startY*m.width+(m.startX+1)] = ' '
	grid[(m.startY+1)*m.width+m.startX] = ' '
}

func (m *Map) isWall(x, y int) bool {
	if x < 0 || x >= m.width || y < 0 || y >= m.height {
		return true
	}
	return m.grid[y*m.width+x] == '#'
}

// makeFlat 生成 n×n 的开放平地（四周一圈墙），用于 space 命令
func (m *Map) makeFlat(n int) {
	w, h := n, n
	grid := make([]byte, w*h)
	for i := range grid {
		grid[i] = ' '
	}
	for x := 0; x < w; x++ {
		grid[x] = '#'
		grid[(h-1)*w+x] = '#'
	}
	for y := 0; y < h; y++ {
		grid[y*w] = '#'
		grid[y*w+w-1] = '#'
	}
	m.width, m.height = w, h
	m.grid = grid
	m.egg = make([]int, len(grid))
	m.startX, m.startY = 1, 1
	m.seed = time.Now().UnixNano()
}

func clampF(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
