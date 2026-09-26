package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eiannone/keyboard"
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

func TestExternalDenied(t *testing.T) {
	// 只允许内置命令，外部命令一律拒绝
	for _, line := range []string{`printf hi`, `printf hi | tr 'a-z' 'A-Z'`, `ls`, `sh`} {
		ui.out = nil
		exit := executeLine(line)
		if exit == 0 {
			t.Errorf("%q 应被拒绝（只允许内置命令）", line)
		}
		joined := strings.Join(ui.out, "\n")
		if !strings.Contains(joined, "未知命令") {
			t.Errorf("%q 应提示未知命令, 实际 %q", line, joined)
		}
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

func TestDoors(t *testing.T) {
	var m Map
	m.regen(51)
	gameMap = m
	doors = nil

	// 无法放在墙上（(0,0) 是外墙）
	ui.out = nil
	if exit := executeLine(`door 0 0`); exit == 0 {
		t.Errorf("墙上放门应失败")
	}
	// 放在空地
	if exit := executeLine(`door 3 3`); exit != 0 {
		t.Fatalf("放门失败: %v", ui.out)
	}
	if doorIdxAt(3, 3) < 0 {
		t.Fatalf("门未登记")
	}
	// 默认关闭：阻挡移动
	if !isBlocked(3, 3) {
		t.Errorf("关闭的门应阻挡移动")
	}
	// door list
	ui.out = nil
	if exit := executeLine(`door list`); exit != 0 {
		t.Fatalf("door list 失败")
	}
	if !strings.Contains(strings.Join(ui.out, ""), "3") {
		t.Errorf("door list 缺门坐标: %v", ui.out)
	}
	// door open 不存在门的位置
	if exit := executeLine(`door open all`); exit != 0 {
		t.Fatalf("door open all 失败")
	}
	if isBlocked(3, 3) {
		t.Errorf("打开的门不应阻挡移动")
	}
	// door close all
	if exit := executeLine(`door close all`); exit != 0 {
		t.Fatalf("door close all 失败")
	}
	if !isBlocked(3, 3) {
		t.Errorf("关闭的门应再次阻挡")
	}
}

func TestBookmarkGotoElevator(t *testing.T) {
	var m Map
	m.regen(101)
	gameMap = m
	doors = nil
	currentFloor = 3
	player.x, player.y = 3.5, 5.5

	if exit := executeLine(`bookmark 基地`); exit != 0 {
		t.Fatalf("bookmark 失败")
	}
	player.x, player.y = 40.5, 50.5
	if exit := executeLine(`goto 基地`); exit != 0 {
		t.Fatalf("goto 失败: %v", ui.out)
	}
	if int(player.x) != 3 || int(player.y) != 5 {
		t.Errorf("goto 应回书签坐标, 实际 %.1f,%.1f", player.x, player.y)
	}
	// elevator 换层会产生新地图
	got := executeLine(`elevator 5`)
	if got != 0 {
		t.Fatalf("elevator 失败")
	}
	if currentFloor != 5 {
		t.Errorf("elevator 后楼层应为 5, 实际 %d", currentFloor)
	}
	// goto 会返回书签楼层
	executeLine(`goto 基地`)
	if currentFloor != 3 {
		t.Errorf("goto 后应回 3 楼, 实际 %d", currentFloor)
	}
}

func TestSaveLoadComplete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	var m Map
	m.regen(41)
	gameMap = m
	doors = []Door{{X: 3, Y: 3, Open: true}, {X: 5, Y: 7, Open: false}}
	currentFloor = 2
	player.x, player.y, player.angle = 12.5, 14.5, 1.1

	ui.out = nil
	if exit := executeLine(`save full`); exit != 0 {
		t.Fatalf("save 失败: %v", ui.out)
	}
	doors = nil
	currentFloor = 1
	player.x, player.y, player.angle = 1.5, 1.5, 0
	if exit := executeLine(`load full`); exit != 0 {
		t.Fatalf("load 失败: %v", ui.out)
	}
	if currentFloor != 2 {
		t.Errorf("载入后楼层应为 2, 实际 %d", currentFloor)
	}
	if len(doors) != 2 || doorIdxAt(3, 3) < 0 || !closedDoorAt(5, 7) {
		t.Errorf("载入后门不完整: %v", doors)
	}
	if int(player.x) != 12 || int(player.y) != 14 || player.angle != 1.1 {
		t.Errorf("载入后玩家状态不完整: %.1f,%.1f,%.1f", player.x, player.y, player.angle)
	}
}

func TestPanelAnimNoPanic(t *testing.T) {
	var m Map
	m.regen(51)
	gameMap = m
	screen.width, screen.height = 177, 63
	ensureBuffers(177, 63)
	for i := 0; i < 200; i++ {
		ui.out = append(ui.out, "xxxx")
	}

	// 打开动画：每 0.5ms 一个采样点，确保覆盖 ph=1/2 的极小高度
	ui.active = false
	ui.closing = false
	var firstLine = time.Now().Add(-time.Second)
	ui.openT = firstLine
	for off := 0; off < 160; off++ {
		ui.openT = time.Now().Add(-time.Duration(off) * 500 * time.Microsecond)
		ui.active = true
		_ = drawCmdPanel(177, 63)
		if off == 0 { // 完全展开也要正常
			ui.openT = firstLine
		}
	}

	// 收起动画：逐步逼近完成时刻，覆盖 ph 递减到 1
	ui.closing = true
	for off := 0; off < 160; off++ {
		ui.closeT = time.Now().Add(-time.Duration(off) * 500 * time.Microsecond)
		_ = drawCmdPanel(177, 63)
	}
	ui.active = false
	ui.closing = false
}

func TestEditModePersistence(t *testing.T) {
	var m Map
	m.regen(51)
	gameMap = m
	doors = nil

	executeLine(`edit`)
	if !editMode {
		t.Fatalf("edit 未进入编辑模式")
	}
	curX, curY = 2, 2
	if !handleEditKey('1', keyboard.Key(0)) {
		t.Errorf("设墙应消费按键")
	}
	if gameMap.grid[2*51+2] != '#' {
		t.Errorf("光标处应为墙")
	}
	handleEditKey('0', keyboard.Key(0))
	if gameMap.grid[2*51+2] != ' ' {
		t.Errorf("光标处应为空地")
	}
	handleEditKey('D', keyboard.Key(0))
	if doorIdxAt(2, 2) < 0 {
		t.Errorf("D 应放门")
	}
	handleEditKey('G', keyboard.Key(0))
	if grabbedCell != ' ' || !grabbedDoor {
		t.Errorf("G 抓取错误: %q 门=%v", grabbedCell, grabbedDoor)
	}
	curX, curY = 4, 4
	handleEditKey('P', keyboard.Key(0))
	if doorIdxAt(4, 4) < 0 {
		t.Errorf("P 应放置抓取的门")
	}
	handleEditKey('Q', keyboard.Key(0))
	if editMode {
		t.Errorf("Q 应退出编辑模式")
	}
}

func TestEggWalls(t *testing.T) {
	var m Map
	m.regen(61)
	if m.egg == nil || len(m.egg) != len(m.grid) {
		t.Fatalf("egg 数组未初始化")
	}
	total, hit := 0, 0
	inside := 0
	for i, c := range m.grid {
		if c == '#' {
			total++
			if m.egg[i] > 0 {
				hit++
				if m.egg[i] > len(eggPatterns) {
					t.Errorf("彩蛋索引越界 %d (池大小 %d)", m.egg[i], len(eggPatterns))
				}
			}
			if i >= m.width && i < len(m.grid)-m.width && i%m.width != 0 && i%m.width != m.width-1 {
				inside++
			}
		}
	}
	_ = inside
	if total == 0 {
		t.Fatalf("迷宫没有墙")
	}
	if hit == 0 {
		t.Errorf("61 号种子没有任何彩蛋墙")
	}
	f := float64(hit) / float64(total)
	if f < 0.02 || f > 0.2 {
		t.Errorf("彩蛋比例 %.3f 偏离约 9%% 太多", f)
	}
	// 彩蛋只出现在墙格
	for i, e := range m.egg {
		if e > 0 && m.grid[i] != '#' {
			t.Fatalf("非墙格 %d 被标为彩蛋", i)
		}
	}
}

func TestImgVideoCommands(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)
	defer func() { wallImages = nil; videos = nil }()

	art := "AA\n BB\n"
	vid := "111\n111\n---\n222\n222\n"
	if err := os.WriteFile(filepath.Join(dir, "art.txt"), []byte(art), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vid.txt"), []byte(vid), 0o644); err != nil {
		t.Fatal(err)
	}

	msg, code := cmdImg([]string{"load", "art.txt", "3", "3"})
	if code != 0 {
		t.Fatalf("img load 失败: %s", msg)
	}
	if len(wallImages) != 1 || wallImages[0].X != 3 || wallImages[0].Y != 3 {
		t.Fatalf("贴图未生效: %+v", wallImages)
	}
	if wallImages[0].W != 3 || wallImages[0].H != 2 {
		t.Fatalf("尺寸错误: %+v", wallImages[0])
	}

	_, code = cmdVideo([]string{"play", "vid.txt", "5", "5"})
	if code != 0 {
		t.Fatalf("video play 失败")
	}
	if len(videos) != 1 || !videos[0].Playing || len(videos[0].Frames) != 2 {
		t.Fatalf("视频未生效: %+v", videos)
	}
	kt, _, im := imgLinkAt(5, 5)
	if kt != 1 || im == nil {
		t.Fatalf("播放中视频应优先命中的墙面")
	}

	_, code = cmdVideo([]string{"stop", "1"})
	if code != 0 || videos[0].Playing {
		t.Fatalf("stop 失败")
	}
	_, code = cmdVideo([]string{"remove", "1"})
	if code != 0 || len(videos) != 0 {
		t.Fatalf("remove 失败")
	}
	_, code = cmdImg([]string{"remove", "1"})
	if code != 0 || len(wallImages) != 0 {
		t.Fatalf("img remove 失败")
	}
	if _, code := cmdImg([]string{"load", "not-exist.txt", "1", "1"}); code == 0 {
		t.Fatalf("缺失文件应报错")
	}
	// 绝对路径在 HOME 之外被拒绝
	if _, code := cmdImg([]string{"load", "/etc/hosts", "1", "1"}); code == 0 {
		t.Fatalf("应阻止访问 HOME 之外")
	}
	if _, code := cmdImg([]string{"load", "../etc/hosts", "1", "1"}); code == 0 {
		t.Fatalf("应阻止越界路径")
	}
}

func TestJump(t *testing.T) {
	player.init(1, 1)
	px := player.x
	player.jumpV = jumpVel
	maxH := 0.0
	for i := 0; i < 200; i++ {
		updatePlayer(0.02)
		if player.jumpY > maxH {
			maxH = player.jumpY
		}
	}
	if maxH < 0.3 {
		t.Errorf("跳跃高度应至少 0.3，实际 %.3f", maxH)
	}
	if player.jumpY != 0 || player.jumpV != 0 {
		t.Errorf("跳跃应落回地面，实际 jumpY=%.3f jumpV=%.3f", player.jumpY, player.jumpV)
	}
	if player.x != px {
		t.Errorf("跳跃不应改变水平位置")
	}
}
