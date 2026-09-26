package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/eiannone/keyboard"
)

// ---- 门 ----

type Door struct {
	X, Y int
	Open bool
}

var doors []Door
var doorBlock []byte // 0=空地/已开门，1=关闭的门（挡射线与移动）

func doorIdxAt(x, y int) int {
	for i := range doors {
		if doors[i].X == x && doors[i].Y == y {
			return i
		}
	}
	return -1
}

func closedDoorAt(x, y int) bool {
	if i := doorIdxAt(x, y); i >= 0 && !doors[i].Open {
		return true
	}
	return false
}

func isBlocked(x, y int) bool {
	if gameMap.isWall(x, y) {
		return true
	}
	return closedDoorAt(x, y)
}

// rebuildDoorBlock 每帧重建关门遮挡表（门数量少，一次线性扫描即可）
func rebuildDoorBlock() {
	if len(doorBlock) != len(gameMap.grid) {
		doorBlock = make([]byte, len(gameMap.grid))
	}
	for i := range doorBlock {
		doorBlock[i] = 0
	}
	for i := range doors {
		d := &doors[i]
		if d.Open {
			continue
		}
		idx := d.Y*gameMap.width + d.X
		if idx >= 0 && idx < len(doorBlock) {
			doorBlock[idx] = 1
		}
	}
}

func frontCell() (int, int, bool) {
	fx := int(player.x + math.Sin(player.angle)*0.8)
	fy := int(player.y + math.Cos(player.angle)*0.8)
	if fx < 0 || fy < 0 || fx >= gameMap.width || fy >= gameMap.height {
		return 0, 0, false
	}
	return fx, fy, true
}

func removeDoorAt(x, y int) {
	for i := range doors {
		if doors[i].X == x && doors[i].Y == y {
			doors = append(doors[:i], doors[i+1:]...)
			return
		}
	}
}

func placeDoor(x, y int) (string, int) {
	if gameMap.isWall(x, y) {
		return fmt.Sprintf("(%d,%d) 是墙，不能放门", x, y), 1
	}
	if doorIdxAt(x, y) >= 0 {
		return fmt.Sprintf("(%d,%d) 已有一扇门", x, y), 1
	}
	doors = append(doors, Door{X: x, Y: y})
	return fmt.Sprintf("门已放置在 (%d,%d)", x, y), 0
}

func doorListText() string {
	if len(doors) == 0 {
		return "没有任何门"
	}
	var out []string
	for i, d := range doors {
		state := "开"
		if !d.Open {
			state = "关"
		}
		out = append(out, fmt.Sprintf("#%d 门 (%d,%d) %s", i, d.X, d.Y, state))
	}
	return strings.Join(out, "\n")
}

// ---- 楼层与书签 ----

var currentFloor = 1

type bookmark struct {
	Name  string
	X, Y  float64
	Floor int
	Seed  int64
}

var bookmarks []bookmark

func floorSeed(f int) int64 {
	return int64(f)*48271 + 0xCAFEBABE
}

func enterFloor(f int) {
	mazeBase := mazeSize
	if mazeBase < 7 {
		mazeBase = gameMap.width
	}
	gameMap.regenSeed(mazeBase, floorSeed(f))
	currentFloor = f
	openMode = false
	doors = nil
	player.x = float64(gameMap.startX) + 0.5
	player.y = float64(gameMap.startY) + 0.5
}

// ---- 编辑模式 ----

var editMode bool
var curX, curY int
var grabbedCell byte = ' '
var grabbedDoor bool
var grabDoorID = -1

func setEditMode(on bool) string {
	if on {
		editMode = true
		curX, curY = int(player.x), int(player.y)
		return "已进入编辑模式：2/8/4/6或方向键移动  0空地 1墙 空格切换 D门 G抓 P放 Q退出"
	}
	editMode = false
	return "已退出编辑模式"
}

// handleEditKey 编辑模式下处理按键，返回 true 表示已消费
func handleEditKey(char rune, key keyboard.Key) bool {
	switch {
	case key == keyboard.KeyArrowUp || char == '8':
		if curY > 0 {
			curY--
		}
		return true
	case key == keyboard.KeyArrowDown || char == '2':
		if curY < gameMap.height-1 {
			curY++
		}
		return true
	case key == keyboard.KeyArrowLeft || char == '4':
		if curX > 0 {
			curX--
		}
		return true
	case key == keyboard.KeyArrowRight || char == '6':
		if curX < gameMap.width-1 {
			curX++
		}
		return true
	case char == 'Q' || char == 'q' || key == keyboard.KeyEsc || char == 27:
		setEditMode(false)
		setStatus("已退出编辑模式")
		return true
	case char == ' ':
		i := curY*gameMap.width + curX
		if gameMap.grid[i] == '#' {
			gameMap.grid[i] = ' '
		} else {
			gameMap.grid[i] = '#'
		}
		removeDoorAt(curX, curY)
		return true
	case char == '0':
		gameMap.grid[curY*gameMap.width+curX] = ' '
		removeDoorAt(curX, curY)
		return true
	case char == '1':
		gameMap.grid[curY*gameMap.width+curX] = '#'
		removeDoorAt(curX, curY)
		return true
	case char == 'D' || char == 'd':
		placeDoor(curX, curY)
		return true
	case char == 'G' || char == 'g':
		grabbedCell = gameMap.grid[curY*gameMap.width+curX]
		grabDoorID = doorIdxAt(curX, curY)
		grabbedDoor = grabDoorID >= 0
		setStatus(fmt.Sprintf("抓手: %q (%s)", grabbedCell, map[bool]string{true: "含门", false: "无门"}[grabbedDoor]))
		return true
	case char == 'P' || char == 'p':
		gameMap.grid[curY*gameMap.width+curX] = grabbedCell
		removeDoorAt(curX, curY)
		if grabbedDoor {
			placeDoor(curX, curY)
			if i := doorIdxAt(curX, curY); i >= 0 {
				doors[i].Open = doors[grabDoorID].Open
			}
		}
		setStatus("已放置")
		return true
	case key == keyboard.KeyEnter:
		return true
	}
	return false
}

// ---- 命令 ----

func cmdDoor(args []string) (string, int) {
	if len(args) == 0 {
		if x, y, ok := frontCell(); ok {
			return placeDoor(x, y)
		}
		return "面前没有可放置的位置", 1
	}
	switch args[0] {
	case "open":
		if len(args) > 1 && args[1] == "all" {
			n := 0
			for i := range doors {
				if !doors[i].Open {
					doors[i].Open = true
					n++
				}
			}
			return fmt.Sprintf("已打开 %d 扇门", n), 0
		}
		if x, y, ok := frontCell(); ok {
			if i := doorIdxAt(x, y); i >= 0 {
				doors[i].Open = true
				return fmt.Sprintf("门 #%d (%d,%d) 已打开", i, x, y), 0
			}
			return fmt.Sprintf("面前 (%d,%d) 没有门", x, y), 1
		}
		return "面前没有位置", 1
	case "close":
		if len(args) > 1 && args[1] == "all" {
			n := 0
			for i := range doors {
				if doors[i].Open {
					doors[i].Open = false
					n++
				}
			}
			return fmt.Sprintf("已关闭 %d 扇门", n), 0
		}
		if x, y, ok := frontCell(); ok {
			if i := doorIdxAt(x, y); i >= 0 {
				doors[i].Open = false
				return fmt.Sprintf("门 #%d (%d,%d) 已关闭", i, x, y), 0
			}
			return fmt.Sprintf("面前 (%d,%d) 没有门", x, y), 1
		}
		return "面前没有位置", 1
	case "list":
		return doorListText(), 0
	default:
		if len(args) == 2 {
			x, err1 := strconv.Atoi(args[0])
			y, err2 := strconv.Atoi(args[1])
			if err1 == nil && err2 == nil {
				return placeDoor(x, y)
			}
			return "坐标必须是整数", 1
		}
		return "用法: door [x y | open|close [all] | list]", 1
	}
}

func cmdBookmark(args []string) (string, int) {
	name := strings.ToLower(strings.Join(args, "_"))
	if name == "" {
		name = fmt.Sprintf("点%d", len(bookmarks)+1)
	}
	for i := range bookmarks {
		if bookmarks[i].Name == name {
			bookmarks[i].X, bookmarks[i].Y = player.x, player.y
			bookmarks[i].Floor = currentFloor
			bookmarks[i].Seed = gameMap.seed
			return fmt.Sprintf("书签 %s 已更新到 (%d,%d) 楼层%d", name, int(player.x), int(player.y), currentFloor), 0
		}
	}
	bookmarks = append(bookmarks, bookmark{Name: name, X: player.x, Y: player.y, Floor: currentFloor, Seed: gameMap.seed})
	return fmt.Sprintf("书签 %s 已记录 (%d,%d) 楼层%d", name, int(player.x), int(player.y), currentFloor), 0
}

func cmdGoto(args []string) (string, int) {
	if len(args) < 1 {
		if len(bookmarks) == 0 {
			return "还没有书签（用 bookmark [名称] 添加当前位置）", 0
		}
		var names []string
		for _, b := range bookmarks {
			names = append(names, fmt.Sprintf("%s:%d楼", b.Name, b.Floor))
		}
		return "书签: " + strings.Join(names, ","), 0
	}
	name := strings.ToLower(strings.Join(args, "_"))
	for _, b := range bookmarks {
		if b.Name != name {
			continue
		}
		if b.Floor != currentFloor {
			mazeBase := mazeSize
			if mazeBase < 7 {
				mazeBase = gameMap.width
			}
			gameMap.regenSeed(mazeBase, b.Seed)
			currentFloor = b.Floor
			openMode = false
			doors = nil
		}
		player.x, player.y = b.X, b.Y
		return fmt.Sprintf("已传送到书签 %s (楼层%d)", name, currentFloor), 0
	}
	return fmt.Sprintf("没有书签 %s", name), 1
}

func cmdElevator(args []string) (string, int) {
	f := currentFloor + 1
	if len(args) > 0 {
		switch {
		case strings.EqualFold(args[0], "list"):
			return "每层迷宫种子固定，可稳定重现", 0
		case strings.EqualFold(args[0], "home"):
			f = 1
		default:
			if n, err := strconv.Atoi(args[0]); err == nil && n >= 1 {
				f = n
			} else {
				return "用法: elevator [楼层|home|list]", 1
			}
		}
	}
	enterFloor(f)
	return fmt.Sprintf("电梯到达楼层 %d", currentFloor), 0
}

func cmdEdit(args []string) (string, int) {
	if len(args) > 0 && (args[0] == "off" || args[0] == "0") {
		return setEditMode(false), 0
	}
	return setEditMode(true), 0
}

// drawEditWindow 编辑模式：左上 1/4 显示光标附近的 2D 网格
func drawEditWindow(w, h int) {
	rw, rh := w/2, h/2
	if rw < 8 || rh < 8 {
		return
	}
	mw, mh := gameMap.width, gameMap.height
	grid := gameMap.grid
	for y := 0; y < rh; y++ {
		for x := 0; x < rw; x++ {
			if y == 0 || y == rh-1 || x == 0 || x == rw-1 {
				rows[y][x] = '|'
				continue
			}
			gx := curX + (x - rw/2)
			gy := curY + (y - rh/2)
			if gx < 0 || gx >= mw || gy < 0 || gy >= mh {
				rows[y][x] = ' '
				continue
			}
			if doorIdxAt(gx, gy) >= 0 {
				rows[y][x] = 'D'
				continue
			}
			if grid[gy*mw+gx] == '#' {
				rows[y][x] = '#'
			} else {
				rows[y][x] = ' '
			}
		}
	}
	rows[rh/2][rw/2] = '@'
	legend := "编辑窗口 光标@ 门D 用 2/8/4/6 或方向键移动"
	if rh > 1 && rw > 2 {
		r := rows[1]
		for i := 1; i < rw-1 && i-1 < len(legend); i++ {
			r[i] = rune(legend[i-1])
		}
	}
}
