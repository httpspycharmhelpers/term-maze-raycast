package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ffmpeg 集成：mp4 用 ffmpeg 自身探测元数据并抽帧，音频交给系统播放器。
// Termux 安装：pkg install ffmpeg

const maxVideoFrames = 6000

var videoRamp = []rune(" .·:;!%#@")

// findFF 返回 ffmpeg 路径（ffplay 可选，仅用于音频播放）
func findFF() (ffmpeg, ffplay string, err error) {
	ffmpeg, _ = exec.LookPath("ffmpeg")
	ffplay, _ = exec.LookPath("ffplay")
	if ffmpeg == "" {
		return "", "", fmt.Errorf("需要 ffmpeg（Termux: pkg install ffmpeg）")
	}
	return ffmpeg, ffplay, nil
}

type ffStreamInfo struct {
	Width, Height  int
	FPS            float64
	HasAudioStream bool
}

// probeVideo 用 ffmpeg -i 的 stderr 解析分辨率、帧率与音轨（不再依赖 ffprobe）
func probeVideo(ffmpeg, path string) (info ffStreamInfo, err error) {
	out, _ := exec.Command(ffmpeg, "-hide_banner", "-i", path).CombinedOutput()
	text := string(out)
	reStream := regexp.MustCompile(`(?m)^\s*Stream #\d+:\d+.*?(Video|Audio):.*$`)
	reSize := regexp.MustCompile(`,\s*(\d{2,5})x(\d{2,5})(?:[\s,\[]|$)`)
	reFPS := regexp.MustCompile(`([\d.]+)\s*fps`)
	for _, line := range strings.Split(text, "\n") {
		if !reStream.MatchString(line) {
			continue
		}
		if strings.Contains(line, "Audio:") {
			info.HasAudioStream = true
			continue
		}
		if strings.Contains(line, "Video:") && info.Width == 0 {
			if m := reSize.FindStringSubmatch(line); len(m) == 3 {
				info.Width, _ = strconv.Atoi(m[1])
				info.Height, _ = strconv.Atoi(m[2])
			}
			if m := reFPS.FindStringSubmatch(line); len(m) == 2 {
				info.FPS, _ = strconv.ParseFloat(m[1], 64)
			}
		}
	}
	if info.Width <= 0 || info.Height <= 0 {
		return info, fmt.Errorf("没有找到视频流（ffmpeg 是否已安装？）")
	}
	return info, nil
}

// mp4ToFrames 用 ffmpeg 把 mp4 抽成 10fps 的 ASCII 真彩帧（W 列、按 2:1 换算行数）
func mp4ToFrames(path string) (frames []WallImage, fps int, hasAudio bool, err error) {
	ffmpeg, _, err := findFF()
	if err != nil {
		return nil, 0, false, err
	}
	info, err := probeVideo(ffmpeg, path)
	if err != nil {
		return nil, 0, false, err
	}
	hasAudio = info.HasAudioStream
	srcFPS := info.FPS
	fps = 10
	if srcFPS > 0 && srcFPS < float64(fps) {
		fps = int(srcFPS)
	}
	if fps < 1 {
		fps = 5
	}
	W := 48
	if info.Width < W {
		W = info.Width
	}
	H := int(float64(W) * float64(info.Height) / float64(info.Width) * 0.6)
	if H < 8 {
		H = 8
	}
	if H > 36 {
		H = 36
	}

	tmp, err := os.CreateTemp("", "mazevideo*.raw")
	if err != nil {
		return nil, 0, false, fmt.Errorf("无法创建临时文件: %v", err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)

	cmd := exec.Command(ffmpeg, "-v", "error", "-y", "-i", path, "-an",
		"-vf", fmt.Sprintf("fps=%d,scale=%d:%d:flags=bicubic", fps, W, H),
		"-pix_fmt", "rgb24", "-f", "rawvideo", tmpName)
	if e := cmd.Run(); e != nil {
		return nil, 0, false, fmt.Errorf("ffmpeg 抽帧失败: %v", e)
	}

	raw, err := os.ReadFile(tmpName)
	if err != nil {
		return nil, 0, false, err
	}
	cell := W * H * 3
	if cell == 0 || len(raw) < cell {
		return nil, 0, false, fmt.Errorf("没有抽到有效帧")
	}
	n := len(raw) / cell
	if n > maxVideoFrames {
		n = maxVideoFrames
	}
	frames = make([]WallImage, 0, n)
	for f := 0; f < n; f++ {
		base := f * cell
		lines := make([]string, 0, H)
		colors := make([][]uint32, 0, H)
		for r := 0; r < H; r++ {
			line := make([]rune, W)
			colRow := make([]uint32, W)
			for c := 0; c < W; c++ {
				i := base + (r*W+c)*3
				R := raw[i]
				G := raw[i+1]
				B := raw[i+2]
				lum := (299*int(R) + 587*int(G) + 114*int(B)) / 1000
				idx := lum * (len(videoRamp) - 1) / 255
				if idx >= len(videoRamp) {
					idx = len(videoRamp) - 1
				}
				line[c] = videoRamp[idx]
				if R == 0 && G == 0 && B == 0 {
					// 纯黑保留 0 → 用墙体默认色，少刷一条色码
					colRow[c] = 0
				} else {
					colRow[c] = uint32(R)<<16 | uint32(G)<<8 | uint32(B)
				}
			}
			lines = append(lines, string(line))
			colors = append(colors, colRow)
		}
		fr := makeWallImage(lines, 0, 0)
		fr.Colors = colors
		frames = append(frames, fr)
	}
	return frames, fps, hasAudio, nil
}

// startAudio 后台播放音轨；按可用工具依次尝试，返回子进程与所用播放器名
func startAudio(path string) (*exec.Cmd, string) {
	players := [][]string{
		{"termux-media-player", "play"},
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "-vn"},
	}
	for _, argv := range players {
		if _, err := exec.LookPath(argv[0]); err != nil {
			continue
		}
		args := append(append([]string{}, argv[1:]...), path)
		c := exec.Command(argv[0], args...)
		if err := c.Start(); err != nil {
			continue
		}
		return c, argv[0]
	}
	return nil, ""
}

func stopAudio(c *exec.Cmd) {
	if c == nil || c.Process == nil {
		return
	}
	_ = c.Process.Kill()
	if len(c.Args) > 0 && strings.Contains(filepath.Base(c.Args[0]), "termux-media-player") {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		go func() {
			defer cancel()
			_ = exec.CommandContext(ctx, "termux-media-player", "stop").Run()
		}()
	}
}

// avPath 供 ffmpeg/ffplay 用统一解析后的绝对路径
func avPath(p string) string {
	if full, err := sanitizeArtPath(p); err == nil {
		return full
	}
	home := os.Getenv("HOME")
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(home, p)
}
