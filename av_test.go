package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func ffmpegOrSkip(t *testing.T) {
	t.Helper()
	for _, b := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(b); err != nil {
			t.Skipf("未安装 %s，跳过 mp4 集成测试", b)
		}
	}
}

func TestMp4Ffmpeg(t *testing.T) {
	ffmpegOrSkip(t)
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)
	defer func() { videos = nil; wallImages = nil }()

	mp4 := filepath.Join(dir, "sample.mp4")
	// ffmpeg 现造一个 1 秒视频：短视频源 + 正弦音，带 H.264(软件可用编码)+AAC
	cmd := exec.Command("ffmpeg", "-v", "error",
		"-f", "lavfi", "-i", "testsrc=size=64x48:rate=12:duration=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-preset", "ultrafast", "-tune", "zerolatency", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-y", mp4)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg 生成样本失败（%v）：%s", err, out)
	}

	// 直接探测
	ffprobe, _ := exec.LookPath("ffprobe")
	info, err := probeVideo(ffprobe, mp4)
	if err != nil {
		t.Fatalf("probeVideo 失败: %v", err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Fatalf("探测不到尺寸: %+v", info)
	}
	if !info.HasAudioStream {
		t.Fatalf("样本应带音轨")
	}

	frames, fps, hasAudio, err := mp4ToFrames(mp4)
	if err != nil {
		t.Fatalf("mp4ToFrames 失败: %v", err)
	}
	if fps < 1 || fps > 12 {
		t.Fatalf("fps 应在 1..12，实际 %d", fps)
	}
	if len(frames) < 5 {
		t.Fatalf("1s/12fps 至少应抽 5 帧，实际 %d", len(frames))
	}
	if !hasAudio {
		t.Fatalf("应探测到音轨")
	}
	for i, fr := range frames {
		if fr.H == 0 || fr.W == 0 {
			t.Fatalf("帧 %d 尺寸非法", i)
		}
		if fr.Colors == nil || len(fr.Colors) != fr.H {
			t.Fatalf("帧 %d 缺彩色数据", i)
		}
		for r := 0; r < fr.H; r++ {
			if len(fr.Colors[r]) != fr.W {
				t.Fatalf("帧 %d 行 %d 颜色宽度不对", i, r)
			}
		}
	}

	// 通过命令链路放置并播放（音轨由 startAudio 择机播放，无播放器也应不报错）
	msg, code := cmdVideo([]string{"play", "sample.mp4", "2", "2"})
	if code != 0 {
		t.Fatalf("video play mp4 失败: %s", msg)
	}
	if len(videos) != 1 || videos[0].FPS == 0 || len(videos[0].Frames) < 5 {
		t.Fatalf("mp4 播放注册异常: %+v", videos)
	}
	_, code = cmdVideo([]string{"stop", "1"})
	if code != 0 {
		t.Fatalf("stop 失败")
	}
}
