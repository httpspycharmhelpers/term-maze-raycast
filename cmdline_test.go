package main

import (
	"os"
	"strings"
	"testing"
)

func TestParsePipeline(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		opts []string
	}{
		{`echo hi`, 1, nil},
		{`echo hi | grep i`, 1, nil},
		{`echo a && echo b || echo c ; echo d`, 4, []string{"&&", "||", ";"}},
		{`echo "a b" 'c d'`, 1, nil},
		{`echo a\ b`, 1, nil},
	}
	for _, tt := range tests {
		p, err := parse(tt.in)
		if err != nil {
			t.Fatalf("%q: %v", tt.in, err)
		}
		if len(p) != tt.n {
			t.Errorf("%q: 期望 %d 段, 实际 %d", tt.in, tt.n, len(p))
		}
	}
	p, err := parse(`where | seed && fov+`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 2 {
		t.Fatalf("期望 2 段, 实际 %d", len(p))
	}
}

func TestExecuteBuiltins(t *testing.T) {
	ui.out = nil
	var m Map
	m.regen(51)
	gameMap = m
	player.x, player.y = 1.5, 1.5
	player.angle, player.pitch, player.camHeight = 0, 0, 0
	settings.FOV = 1.0

tests := []struct {
		line string
		want string
		code int
	}{
		{`where`, "位置", 0},
		{`seed`, "种子: ", 0},
		{`fov wide`, "视野", 0},
		{`fov +`, "视野", 0},
		{`tp 3 3`, "已传送", 0},
		{`tp -5 -5`, "超出地图范围", 1},
		{`home`, "已传送回出生点", 0},
		{`reset`, "已重置", 0},
		{`flip`, "已上下翻转", 0},
		{`floorinfo`, "迷宫", 0},
	}
	for _, tt := range tests {
		ui.out = nil
		exit := executeLine(tt.line)
		if exit != tt.code {
			t.Errorf("%q: 退出码 %d, 期望 %d", tt.line, exit, tt.code)
		}
		joined := strings.Join(ui.out, "\n")
		if !strings.Contains(joined, tt.want) {
			t.Errorf("%q: 输出 %q 缺少 %q", tt.line, joined, tt.want)
		}
	}

	if s, _ := cmdSeed(nil); !strings.HasPrefix(s, "种子: ") {
		t.Errorf("seed 输出异常: %q", s)
	}
}

func TestExternalForward(t *testing.T) {
	ui.out = nil
	exit := executeLine(`printf hi | tr 'a-z' 'A-Z'`)
	if exit != 0 {
		t.Fatalf("外部命令退出码 %d, 输出 %v", exit, ui.out)
	}
	joined := strings.Join(ui.out, "|")
	if !strings.Contains(joined, "HI") {
		t.Errorf("外部命令输出异常: %q", joined)
	}

	ui.out = nil
	exit = executeLine(`nonexistent_cmd_xyz`)
	if exit == 0 {
		t.Errorf("未知命令应返回非零，实际 输出 %v", ui.out)
	}
}

func TestSaveLoadFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var m Map
	m.regen(51)
	gameMap = m

	ui.out = nil
	if exit := executeLine(`save testmap`); exit != 0 {
		t.Fatalf("save 退出码 %d, 输出 %v", exit, ui.out)
	}
	path := home + "/testmap.rmap"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("存档未写入: %v", err)
	}
	if string(data[:5]) != "RAMAP" {
		t.Errorf("魔数应为 RAMAP, 实际 %q", data[:5])
	}

	ui.out = nil
	if exit := executeLine(`file ` + path); exit != 0 {
		t.Fatalf("file 退出码 %d", exit)
	}
	if !strings.Contains(strings.Join(ui.out, ""), "本游戏存档") {
		t.Errorf("file 识别异常: %v", ui.out)
	}

	gameMap.regen(101)
	ui.out = nil
	if exit := executeLine(`load testmap`); exit != 0 {
		t.Fatalf("load 退出码 %d, 输出 %v", exit, ui.out)
	}
	if gameMap.width != 51 || gameMap.height != 51 {
		t.Errorf("载入后尺寸应为 51x51, 实际 %dx%d", gameMap.width, gameMap.height)
	}
	if gameMap.grid[m.startY*m.width+m.startX] != ' ' {
		t.Errorf("载入后出生点应开放")
	}
}

func TestFileIdentities(t *testing.T) {
	ui.out = nil
	if exit := executeLine(`file /nonexistent`); exit == 0 {
		t.Errorf("不存在文件应返回非零")
	}
}
