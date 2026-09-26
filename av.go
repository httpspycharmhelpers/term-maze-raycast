package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// ffmpeg 集成：mp4 用 ffprobe 探测、ffmpeg 抽帧成 ASCII 真彩帧、ffplay 放音轨。
// Termux 安装：pkg install ffmpeg

const maxVideoFrames = 6000

var videoRamp = []rune(" .·:;!%#@")

// findFF 返回三个可执行文件路径；缺失时给出安装提示
func findFF() (ffmpeg, ffprobe, ffplay string, err error) {
	ffmpeg, _ = exec.LookPath("ffmpeg")
	ffprobe, _ = exec.LookPath("ffprobe")
	ffplay, _ = exec.LookPath("ffplay")
	if ffmpeg == "" || ffprobe == "" {
		return "", "", "", fmt.Errorf("需要 ffmpeg + ffprobe（Termux: pkg install ffmpeg）")
	}
	return ffmpeg, ffprobe, ffplay, nil
}

type ffStreamInfo struct {
	Width, Height  int
	Duration       string
	AvgFrameRate   string
	NumFrames      string
	HasAudioStream bool
}

func (s *ffStreamInfo) duration() float64 {
	f, _ := strconv.ParseFloat(s.Duration, 64)
	return f
}

// fps 解析 "30/1" 或 "30000/1001"
func (s *ffStreamInfo) fps() float64 {
	a := strings.SplitN(s.AvgFrameRate, "/", 2)
	if len(a) == 2 {
		n, err1 := strconv.ParseFloat(a[0], 64)
		d, err2 := strconv.ParseFloat(a[1], 64)
		if err1 == nil && err2 == nil && d > 0 {
			return n / d
		}
	}
	f, _ := strconv.ParseFloat(s.AvgFrameRate, 64)
	return f
}

// probeVideo 探测视频信息并检查是否有音轨
func probeVideo(ffprobe, path string) (info ffStreamInfo, err error) {
	out, err := exec.Command(ffprobe, "-v", "error",
		"-show_entries", "stream=codec_type,width,height,duration,nb_frames,avg_frame_rate",
		"-of", "json", path).Output()
	if err != nil {
		return info, fmt.Errorf("ffprobe 失败: %v", err)
	}
	var resp struct {
		Streams []struct {
			CodecType     string `json:"codec_type"`
			Width, Height int
			Duration      string
			AvgFrameRate  string
			NbFrames      string `json:"nb_frames"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return info, fmt.Errorf("ffprobe 输出无法解析: %v", err)
	}
	for _, s := range resp.Streams {
		switch s.CodecType {
		case "video":
			info.Width = s.Width
			info.Height = s.Height
			info.Duration = s.Duration
			info.AvgFrameRate = s.AvgFrameRate
			info.NumFrames = s.NbFrames
		case "audio":
			info.HasAudioStream = true
		}
	}
	if info.Width <= 0 || info.Height <= 0 {
		return info, fmt.Errorf("没有找到视频流")
	}
	return info, nil
}

// mp4ToFrames 用 ffmpeg 把 mp4 抽成 10fps 的 ASCII 真彩帧（W 列、按 2:1 换算行数）
func mp4ToFrames(path string) (frames []WallImage, fps int, hasAudio bool, err error) {
	ffmpeg, ffprobe, _, err := findFF()
	if err != nil {
		return nil, 0, false, err
	}
	info, err := probeVideo(ffprobe, path)
	if err != nil {
		return nil, 0, false, err
	}
	hasAudio = info.HasAudioStream
	srcFPS := info.fps()
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
		{"ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", "-vn"},
		{"mpv", "--no-video", "--really-quiet"},
		{"termux-media-player", "play"},
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
	if c != nil && c.Process != nil {
		_ = c.Process.Kill()
	}
}

// mediaAudioPath 供 ffprobe/ffmpeg/ffplay 用统一解析后的绝对路径
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
