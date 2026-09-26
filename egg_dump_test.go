package main

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
)

// 随机生成大量彩蛋图案，逐一断言：严格左右+上下对称、字符来自预设表且非空
func TestEggPatternSymmetry(t *testing.T) {
	rng := rand.New(rand.NewSource(20260926))
	for n := 0; n < 300; n++ {
		p := genEggPattern(rng)
		filled, okGlyph := 0, false
		for _, rw := range p {
			for _, ch := range rw {
				if ch != ' ' {
					filled++
					for _, g := range egGlyphs {
						if ch == g {
							okGlyph = true
						}
					}
				}
			}
		}
		if filled == 0 {
			t.Logf("blank params: %s", eggDebug)
			for r := 0; r < 8; r++ {
				l := ""
				for c := 0; c < 8; c++ {
					if p[r][c] == ' ' {
						l += "."
					} else {
						l += fmt.Sprintf("%c", p[r][c])
					}
				}
				t.Log(l)
			}
			t.Fatalf("生成 %d 个图案出现空白图案", n+1)
		}
		if !okGlyph {
			for r := 0; r < 8; r++ {
				for c := 0; c < 8; c++ {
					if p[r][c] != ' ' {
						bad := true
						for _, g := range egGlyphs {
							if p[r][c] == g {
								bad = false
							}
						}
						if bad {
							t.Logf("offender #%d (%d,%d)=U+%04X", n+1, r, c, p[r][c])
						}
					}
				}
			}
			t.Fatalf("图案 %d 使用了预设之外的字符", n+1)
		}
		for r := 0; r < 8; r++ {
			for c := 0; c < 4; c++ {
				if p[r][c] != p[r][7-c] {
					t.Errorf("图案 #%d 左右不对称: (%d,%d)=%q vs (%d,%d)=%q", n, r, c, p[r][c], r, 7-c, p[r][7-c])
				}
				if p[c][r] != p[7-c][r] {
					t.Errorf("图案 #%d 上下不对称: (%d,%d)=%q vs (%d,%d)=%q", n, c, r, p[c][r], 7-c, r, p[7-c][r])
				}
			}
		}
	}
}

// 手动观察彩蛋墙渲染效果：把玩家放到朝北的某面彩蛋墙前面，打印 60x30 帧
func TestDumpEggWall(t *testing.T) {
	if os.Getenv("EGG_DUMP") == "" {
		t.Skip("设置 EGG_DUMP=1 运行: EGG_DUMP=1 go test -run TestDumpEggWall -v")
	}
	for _, seed := range []int64{42, 7, 9} {
		var m Map
		m.regen(int(seed))
		// 找一个靠近中心的彩蛋墙
		ex, ey := -1, -1
		for y := 2; y < m.height-2 && ex < 0; y++ {
			for x := 2; x < m.width-2; x++ {
				if m.egg[y*m.width+x] > 0 {
					ex, ey = x, y
					break
				}
			}
		}
		if ex < 0 {
			t.Logf("seed %d 无彩蛋", seed)
			continue
		}
		// 挖一条南向走廊正对彩蛋墙，保证视线直达
		for yy := ey + 1; yy <= ey+1; yy++ {
			m.grid[yy*m.width+ex] = ' '
			m.egg[yy*m.width+ex] = 0
		}
		// 玩家贴脸朝北，让彩蛋墙占满画面
		player.init(float64(ex), float64(ey)+1.4)
		player.angle = 0
		player.pitch = 0
		player.camHeight = 0
		screen.width, screen.height = 60, 30
		ensureBuffers(60, 30)
		gameMap = m
		ui.active = false
		ui.closing = false
		render()
		t.Logf("seed %d: 彩蛋格 (%d,%d) egg=%d, 玩家朝北", seed, ex, ey, m.egg[ey*m.width+ex])
		var out []string
		for y := 0; y < 30; y++ {
			line := make([]rune, 60)
			copy(line, rows[y])
			out = append(out, string(line))
		}
		for _, l := range out {
			t.Log("|" + l + "|")
		}
	}
}
