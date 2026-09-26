package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ---- RAMAP 存档格式 ----
// 魔数 "RAMAP"(5) + 版本(1) + 种子 int64(8) + 宽 int32(4) + 高 int32(4) + grid w*h 字节

const rmapMagic = "RAMAP"
const rmapVersion = 0x01

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
	buf := make([]byte, 5+1+8+4+4+len(grid))
	copy(buf, rmapMagic)
	buf[5] = rmapVersion
	binary.LittleEndian.PutUint64(buf[6:], uint64(gameMap.seed))
	binary.LittleEndian.PutUint32(buf[14:], uint32(gameMap.width))
	binary.LittleEndian.PutUint32(buf[18:], uint32(gameMap.height))
	copy(buf[22:], grid)
	if err := os.WriteFile(p, buf, 0644); err != nil {
		return "保存失败: " + err.Error(), 1
	}
	return fmt.Sprintf("已保存 %s (%.2f MB, RAMAP %dx%d, 种子 %d)",
		p, float64(len(buf))/1024/1024, gameMap.width, gameMap.height, gameMap.seed), 0
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
	m := &Map{}
	m.seed = int64(binary.LittleEndian.Uint64(data[6:14]))
	m.width = int(binary.LittleEndian.Uint32(data[14:18]))
	m.height = int(binary.LittleEndian.Uint32(data[18:22]))
	if m.width < 7 || m.height < 7 || m.width > 20000 || m.height > 20000 {
		return "存档尺寸异常，拒绝载入", 1
	}
	if len(data) != 22+m.width*m.height {
		return fmt.Sprintf("存档数据不完整（预期 %d 字节，实际 %d）", 22+m.width*m.height, len(data)), 1
	}
	m.grid = append([]byte{}, data[22:]...)
	m.startX, m.startY = 1, 1
	gameMap = *m
	openMode = false
	player.x = float64(1) + 0.5
	player.y = float64(1) + 0.5
	fmtStr := "已载入 %s (%dx%d, 种子 %d)"
	if m.width == 200 {
		fmtStr = "已载入 %s (平地 %dx%d)"
	}
	return fmt.Sprintf(fmtStr, filepath.Base(p), m.width, m.height, m.seed), 0
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
	head := make([]byte, 8)
	n, _ := f.Read(head)
	if n >= 5 && string(head[:5]) == rmapMagic {
		if len(head) >= 22 {
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
