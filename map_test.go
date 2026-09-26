package main

import (
	"testing"
	"time"
)

// 验证巨型迷宫：1) 生成耗时可控  2) 所有空地互相连通（无封死道路/无孤岛）  3) 开局北/东开口
func TestRegenConnectivityAndTiming(t *testing.T) {
	start := time.Now()
	var m Map
	m.regen(1501)
	gen := time.Since(start)
	t.Logf("生成 %dx%d 迷宫用时 %v", m.width, m.height, gen)
	if gen > 3*time.Second {
		t.Errorf("生成过慢: %v", gen)
	}

	if m.grid[m.startY*m.width+m.startX] != ' ' {
		t.Fatalf("出生点 (%d,%d) 被墙堵死", m.startX, m.startY)
	}
	if m.grid[m.startY*m.width+m.startX+1] != ' ' || m.grid[(m.startY+1)*m.width+m.startX] != ' ' {
		t.Errorf("出生点北/东侧开口未打通，开局应可直接移动")
	}

	floor := 0
	for _, c := range m.grid {
		if c != '#' {
			floor++
		}
	}

	visited := make([]bool, len(m.grid))
	queue := make([]int, 0, 65536)
	si := m.startY*m.width + m.startX
	visited[si] = true
	queue = append(queue, si)

	reachable := 0
	dirs := [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	for len(queue) > 0 {
		head := queue[0]
		queue = queue[1:]
		reachable++
		x, y := head%m.width, head/m.width
		for _, d := range dirs {
			nx, ny := x+d[0], y+d[1]
			if nx < 0 || nx >= m.width || ny < 0 || ny >= m.height {
				continue
			}
			ni := ny*m.width + nx
			if !visited[ni] && m.grid[ni] != '#' {
				visited[ni] = true
				queue = append(queue, ni)
			}
		}
	}

	t.Logf("空地块 %d，从出生点可达 %d", floor, reachable)
	if reachable != floor {
		t.Fatalf("存在封死道路: 可达 %d / 全部 %d", reachable, floor)
	}
}
