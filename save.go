package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// ---- RAMAP 存档格式 ----
// 魔数 "RAMAP"(5) + 版本(1) + 种子 int64(8) + 宽 int32(4) + 高 int32(4) + 楼层 int32(4)
// + 玩家 x/y/角/俯仰/视高 float64(8*5) + 门数量 int32(4)
// + 门 x,y,open(8*n) + grid w*h 字节
// 版本 1 = 早期格式（22 字节头 + grid），版本 2 = 完整游戏数据

const rmapMagic = "RAMAP"
const rmapVersion = 0x02

// saveBodySize 完整头（含门区）之后接 grid；每扇门 9 字节：x,y(4+4) + open(1)
func saveHeaderSize(doorCount int) int {
	return 70 + 9*doorCount
}

func writeFloat64(b []byte, off int, v float64) {
	binary.LittleEndian.PutUint64(b[off:], math.Float64bits(v))
}

func readFloat64(b []byte, off int) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(b[off:]))
}

// savePath 把用户给的存档名解析为 ~/ 目录下的绝对路径（只允许访问 Home）
func savePath(name string) (string, error) {
	home := os.Getenv("HOME")
	if home == "" {
		return "", fmt.Errorf("无法确定 HOME 目录")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "world"
	}
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", fmt.Errorf("存档名不能包含路径或 ..，只能保存在 ~/ 目录")
	}
	if !strings.HasSuffix(name, ".rmap") {
		name += ".rmap"
	}
	return filepath.Join(home, name), nil
}

func cmdSave(args []string) (string, int) {
	p, err := savePath(joinArgs(args))
	if err != nil {
		return err.Error(), 1
	}
	grid := append([]byte{}, gameMap.grid...)
	hs := saveHeaderSize(len(doors))
	buf := make([]byte, hs+len(grid))
	copy(buf, rmapMagic)
	buf[5] = rmapVersion
	binary.LittleEndian.PutUint64(buf[6:], uint64(gameMap.seed))
	binary.LittleEndian.PutUint32(buf[14:], uint32(gameMap.width))
	binary.LittleEndian.PutUint32(buf[18:], uint32(gameMap.height))
	binary.LittleEndian.PutUint32(buf[22:], uint32(currentFloor))
	writeFloat64(buf, 26, player.x)
	writeFloat64(buf, 34, player.y)
	writeFloat64(buf, 42, player.angle)
	writeFloat64(buf, 50, player.pitch)
	writeFloat64(buf, 58, player.camHeight)
	binary.LittleEndian.PutUint32(buf[66:], uint32(len(doors)))
	for i, d := range doors {
		off := 70 + i*9
		binary.LittleEndian.PutUint32(buf[off:], uint32(d.X))
		binary.LittleEndian.PutUint32(buf[off+4:], uint32(d.Y))
		if d.Open {
			buf[off+8] = 1
		}
	}
	copy(buf[hs:], grid)
	if err := os.WriteFile(p, buf, 0644); err != nil {
		return "保存失败: " + err.Error(), 1
	}
	return fmt.Sprintf("已保存 %s (%.2f MB, RAMAP %dx%d 楼层%d 种子%d 门%d)",
		p, float64(len(buf))/1024/1024, gameMap.width, gameMap.height, currentFloor, gameMap.seed, len(doors)), 0
}

func cmdLoad(args []string) (string, int) {
	p, err := savePath(joinArgs(args))
	if err != nil {
		return err.Error(), 1
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Sprintf("无法读取 %s", p), 1
	}
	if len(data) < 22 || string(data[:5]) != rmapMagic {
		return fmt.Sprintf("%s 不是有效存档（缺少 RAMAP 头）", filepath.Base(p)), 1
	}
	ver := data[5]
	m := &Map{}
	m.seed = int64(binary.LittleEndian.Uint64(data[6:14]))
	m.width = int(binary.LittleEndian.Uint32(data[14:18]))
	m.height = int(binary.LittleEndian.Uint32(data[18:22]))
	if m.width < 7 || m.height < 7 || m.width > 20000 || m.height > 20000 {
		return "存档尺寸异常，拒绝载入", 1
	}
	want := m.width * m.height
	var loadedDoors []Door
	var loadedFloor = 1
	switch ver {
	case rmapVersion:
		doorCount := int(binary.LittleEndian.Uint32(data[66:70]))
		hs := saveHeaderSize(doorCount)
		if hs+want != len(data) {
			return fmt.Sprintf("存档数据不完整（预期 %d 字节，实际 %d）", hs+want, len(data)), 1
		}
		loadedFloor = int(binary.LittleEndian.Uint32(data[22:26]))
		loadedDoors = make([]Door, 0, doorCount)
		for i := 0; i < doorCount; i++ {
			off := 70 + i*9
			d := Door{
				X:    int(binary.LittleEndian.Uint32(data[off:])),
				Y:    int(binary.LittleEndian.Uint32(data[off+4:])),
				Open: data[off+8] != 0,
			}
			loadedDoors = append(loadedDoors, d)
		}
		m.grid = append([]byte{}, data[hs:]...)
		player.x = readFloat64(data, 26)
		player.y = readFloat64(data, 34)
		player.angle = readFloat64(data, 42)
		player.pitch = readFloat64(data, 50)
		player.camHeight = readFloat64(data, 58)
	case 0x01:
		if 22+want != len(data) {
			return fmt.Sprintf("存档数据不完整（预期 %d 字节，实际 %d）", 22+want, len(data)), 1
		}
		m.grid = append([]byte{}, data[22:]...)
		player.x = float64(1) + 0.5
		player.y = float64(1) + 0.5
	default:
		return "存档版本不支持", 1
	}
	m.startX, m.startY = 1, 1
	gameMap = *m
	doors = loadedDoors
	currentFloor = loadedFloor
	openMode = false
	doorBlock = nil
	fmtStr := "已载入 %s (%dx%d 楼层%d, 种子 %d)"
	if m.width == 200 {
		fmtStr = "已载入 %s (平地 %dx%d)"
	}
	return fmt.Sprintf(fmtStr, filepath.Base(p), m.width, m.height, currentFloor, m.seed), 0
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}

func cmdFile(args []string) (string, int) {
	if len(args) < 1 {
		return "用法: file <路径>", 1
	}
	p := strings.Join(args, " ")
	f, err := os.Open(p)
	if err != nil {
		return "文件不存在: " + p, 1
	}
	defer f.Close()
	head := make([]byte, 22)
	n, _ := f.Read(head)
	if n >= 5 && string(head[:5]) == rmapMagic {
		if n >= 22 {
			w := binary.LittleEndian.Uint32(head[14:18])
			h := binary.LittleEndian.Uint32(head[18:22])
			return fmt.Sprintf("本游戏存档 (RAMAP)：%dx%d", w, h), 0
		}
		return "本游戏存档 (RAMAP)", 0
	}
	if n >= 4 && head[0] == 0x7f && string(head[1:4]) == "ELF" {
		return "ELF 可执行文件（不是游戏存档）", 0
	}
	return "未知文件（不是本游戏存档）", 0
}
