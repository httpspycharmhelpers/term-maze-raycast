package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgs(t *testing.T) {
	cases := []struct {
		args []string
		size int
		save string
	}{
		{[]string{}, 1000, ""},
		{[]string{"500"}, 500, ""},
		{[]string{"-s", "world"}, 1000, "world"},
		{[]string{"--save", "a.rmap", "800"}, 800, "a.rmap"},
		{[]string{"--save=b.rmap"}, 1000, "b.rmap"},
		{[]string{"-s=c.rmap"}, 1000, "c.rmap"},
		{[]string{"-sd.rmap"}, 1000, "d.rmap"},
	}
	for _, c := range cases {
		size, save := parseArgs(c.args)
		if size != c.size || save != c.save {
			t.Fatalf("parseArgs(%v)=(%d,%q), want (%d,%q)", c.args, size, save, c.size, c.save)
		}
	}
}

func TestResolveStartupSavePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	want := filepath.Join(home, "w.rmap")
	if err := writeTestFile(want); err != nil {
		t.Fatal(err)
	}
	for _, arg := range []string{"w", "w.rmap", "~/w.rmap", want} {
		got, err := resolveStartupSavePath(arg)
		if err != nil {
			t.Fatalf("resolve(%q): %v", arg, err)
		}
		if filepath.Base(got) != "w.rmap" {
			t.Fatalf("resolve(%q)=%q", arg, got)
		}
	}
	if _, err := resolveStartupSavePath("missing"); err == nil {
		t.Fatal("expected error for missing save")
	}
}

func writeTestFile(p string) error {
	return os.WriteFile(p, []byte("x"), 0644)
}

func TestSaveLoadFromPathRoundtrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oldMap, oldPlayer, oldDoors, oldFloor, oldOpen := gameMap, player, doors, currentFloor, openMode
	t.Cleanup(func() {
		gameMap, player, doors, currentFloor, openMode = oldMap, oldPlayer, oldDoors, oldFloor, oldOpen
		doorBlock = nil
	})

	gameMap.regen(31)
	gameMap.seed = 4242
	player.x, player.y, player.angle = 3.5, 5.5, 1.25
	doors = []Door{{X: 3, Y: 5, Open: true}}
	currentFloor = 3
	if out, code := cmdSave([]string{"rt"}); code != 0 {
		t.Fatalf("save: %s", out)
	}

	gameMap = Map{}
	player = Player{}
	doors = nil
	out, code := loadSaveFromPath(filepath.Join(home, "rt.rmap"))
	if code != 0 {
		t.Fatalf("load: %s", out)
	}
	if gameMap.width != 31 || gameMap.seed != 4242 || currentFloor != 3 {
		t.Fatalf("loaded header mismatch: %dx%d seed=%d floor=%d", gameMap.width, gameMap.height, gameMap.seed, currentFloor)
	}
	if player.x != 3.5 || player.y != 5.5 || player.angle != 1.25 {
		t.Fatalf("player mismatch: %+v", player)
	}
	if len(doors) != 1 || !doors[0].Open || doors[0].X != 3 {
		t.Fatalf("doors mismatch: %+v", doors)
	}
}

func TestGrabPlaceCommands(t *testing.T) {
	oldMap, oldDoors, oldCell := gameMap, doors, grabbedCell
	t.Cleanup(func() { gameMap, doors, grabbedCell = oldMap, oldDoors, oldCell })

	gameMap.regen(31)
	doors = nil
	gameMap.grid[2*gameMap.width+2] = '#'
	gameMap.grid[4*gameMap.width+4] = ' '
	if out, code := cmdGrab([]string{"2", "2"}); code != 0 {
		t.Fatalf("grab: %s", out)
	}
	if grabbedCell != '#' {
		t.Fatalf("grabbed=%q, want #", grabbedCell)
	}
	if out, code := cmdPlace([]string{"4", "4"}); code != 0 {
		t.Fatalf("place: %s", out)
	}
	if gameMap.grid[4*gameMap.width+4] != '#' {
		t.Fatal("place did not write cell")
	}
	if _, code := cmdGrab([]string{"9999", "9999"}); code == 0 {
		t.Fatal("grab accepted out-of-range coords")
	}
}
