package main

import "testing"

func resetMedia() {
	wallImages = nil
	videos = nil
}

func TestImgIDsMatchRemove(t *testing.T) {
	resetMedia()
	t.Cleanup(resetMedia)
	wallImages = append(wallImages, makeWallImage([]string{"a"}, 1, 1))
	wallImages = append(wallImages, makeWallImage([]string{"b"}, 2, 2))
	out, code := cmdImg([]string{"remove", "1"})
	if code != 0 {
		t.Fatalf("remove code=%d out=%q", code, out)
	}
	if len(wallImages) != 1 || wallImages[0].X != 2 {
		t.Fatalf("remove wrong element: %+v", wallImages)
	}
}

func TestImgRemoveNoArgDoesNotPanic(t *testing.T) {
	resetMedia()
	t.Cleanup(resetMedia)
	out, code := cmdImg([]string{"remove"})
	if code == 0 || out == "" {
		t.Fatalf("expected usage error, got code=%d out=%q", code, out)
	}
}

func TestImgRemoveAll(t *testing.T) {
	resetMedia()
	t.Cleanup(resetMedia)
	wallImages = append(wallImages, makeWallImage([]string{"a"}, 1, 1))
	out, code := cmdImg([]string{"remove", "all"})
	if code != 0 || len(wallImages) != 0 {
		t.Fatalf("remove all failed: code=%d out=%q n=%d", code, out, len(wallImages))
	}
}

func TestVideoStopAll(t *testing.T) {
	resetMedia()
	t.Cleanup(resetMedia)
	videos = []Video{{X: 1, Y: 1, Playing: true}, {X: 2, Y: 2, Playing: true}}
	out, code := cmdVideo([]string{"stop"})
	if code != 0 {
		t.Fatalf("stop code=%d out=%q", code, out)
	}
	for i, v := range videos {
		if v.Playing {
			t.Fatalf("video %d still playing", i)
		}
	}
}

func TestVideoRemoveNoArgDoesNotPanic(t *testing.T) {
	resetMedia()
	t.Cleanup(resetMedia)
	out, code := cmdVideo([]string{"remove"})
	if code == 0 || out == "" {
		t.Fatalf("expected usage error, got code=%d out=%q", code, out)
	}
}

func TestVideoRemoveAll(t *testing.T) {
	resetMedia()
	t.Cleanup(resetMedia)
	videos = []Video{{X: 1, Y: 1}, {X: 2, Y: 2}}
	out, code := cmdVideo([]string{"remove", "all"})
	if code != 0 || len(videos) != 0 {
		t.Fatalf("remove all failed: code=%d out=%q n=%d", code, out, len(videos))
	}
}
