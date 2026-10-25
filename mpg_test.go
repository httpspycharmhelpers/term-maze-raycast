package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func makeTestVideo(t *testing.T, ext, codec, muxer string) string {
	t.Helper()
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	p := filepath.Join(t.TempDir(), "test"+ext)
	cmd := exec.Command(ff, "-v", "error", "-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=160x120:rate=25",
		"-c:v", codec, "-f", muxer, p)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot produce %s: %v %s", ext, err, out)
	}
	return p
}

func TestProbeVideoWithoutFFprobe(t *testing.T) {
	p := makeTestVideo(t, ".mpg", "mpeg1video", "mpeg")
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	info, err := probeVideo(ff, p)
	if err != nil {
		t.Fatalf("probeVideo: %v", err)
	}
	if info.Width != 160 || info.Height != 120 {
		t.Fatalf("probe=%dx%d, want 160x120", info.Width, info.Height)
	}
}
