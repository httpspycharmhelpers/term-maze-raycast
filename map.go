package main

import (
	"math"
	"math/rand"
	"runtime"
	"sync"
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

// eggPoolSize 彩蛋图案池大小；每面彩蛋墙从池中随机取一个图案。
// 池化后大图不再为每格单独生成图案：1501² 的蛋墙图案内存从 ~25MB 降到 ~256KB。
const eggPoolSize = 1024

// fillEggs 约 9% 的墙格随机变成对称像素画彩蛋墙。
// 图案池按 CPU 核数并行生成（每个槽位用派生种子，结果与核数无关、随迷宫种子可复现）。
func (m *Map) fillEggs(rng *rand.Rand) {
	m.egg = make([]int, len(m.grid))
	pool := make([][8][8]rune, eggPoolSize+1) // 0 号为占位，蛋墙索引从 1 开始
	workers := runtime.NumCPU()
	if workers > eggPoolSize {
		workers = eggPoolSize
	}
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	chunk := (eggPoolSize + workers - 1) / workers
	for w := 0; w < workers; w++ {
		lo := 1 + w*chunk
		hi := lo + chunk
		if hi > eggPoolSize+1 {
			hi = eggPoolSize + 1
		}
		if lo >= hi {
			continue
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			seed := int64(uint64(m.seed) ^ uint64(lo)*0x9E3779B97F4A7C15)
			prng := rand.New(rand.NewSource(seed))
			for i := lo; i < hi; i++ {
				pool[i] = genEggPattern(prng)
			}
		}(lo, hi)
	}
	wg.Wait()
	eggPatterns = pool
	for i, c := range m.grid {
		if c == '#' && rng.Float64() < 0.09 {
			m.egg[i] = 1 + rng.Intn(eggPoolSize)
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

	// 递归回溯（迭代实现）：按 CPU 核数把房间列切成若干竖条，多核并行挖通道，
	// 再在条带边界随机开门把各条带连起来。每个条带内部用派生种子，
	// 结果与核数无关、随迷宫种子可复现。
	workers := maxWorkers()
	if w*h < 600*600 {
		// 小图并行收益抵不过调度开销，直接串行
		workers = 1
	}
	roomsX := (w - 1) / 2
	if workers > roomsX {
		workers = roomsX
	}
	if workers < 1 {
		workers = 1
	}
	per := (roomsX + workers - 1) / workers
	var wg sync.WaitGroup
	for k := 0; k < workers; k++ {
		rx0, rx1 := k*per, (k+1)*per
		if rx1 > roomsX {
			rx1 = roomsX
		}
		if rx0 >= rx1 {
			continue
		}
		wg.Add(1)
		go func(k, rx0, rx1 int) {
			defer wg.Done()
			srng := rand.New(rand.NewSource(int64(uint64(m.seed) ^ uint64(k+1)*0x9E3779B97F4A7C15)))
			carveRooms(grid, w, h, rx0, rx1, srng)
		}(k, rx0, rx1)
	}
	wg.Wait()

	rng := rand.New(rand.NewSource(m.seed))
	// 条带边界：每个边界列按概率开门，并强制一扇保证连通
	for k := 1; k < workers; k++ {
		rx := k * per
		if rx >= roomsX {
			break
		}
		bx := 2 * rx
		forced := 1 + 2*(k*37%((h-1)/2))
		for y := 1; y < h-1; y += 2 {
			if y == forced || rng.Float64() < 0.15 {
				grid[y*w+bx] = ' '
			}
		}
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

// carveRooms 在一个竖条的房间范围 [rx0,rx1) 内做递归回溯挖通道
func carveRooms(grid []byte, w, h, rx0, rx1 int, rng *rand.Rand) {
	x0, x1 := 1+2*rx0, 1+2*(rx1-1)
	start := 1*w + x0
	grid[start] = ' '
	stack := make([]int, 1, (x1-x0+1)*(h/2))
	stack[0] = start
	dirs := [4][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}}
	for len(stack) > 0 {
		idx := stack[len(stack)-1]
		x, y := idx%w, idx/w
		var cand [4]int
		n := 0
		for _, d := range dirs {
			nx, ny := x+d[0], y+d[1]
			if nx >= x0 && nx <= x1 && ny > 0 && ny < h-1 && grid[ny*w+nx] == '#' {
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
