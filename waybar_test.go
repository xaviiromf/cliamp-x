package main

import (
	"strings"
	"testing"

	"github.com/xaviiromf/cliamp-x/ipc"
)

func TestWaybarLine(t *testing.T) {
	snap := &ipc.RuntimeSnapshot{State: "playing", Position: 30, Duration: 120,
		Track: &ipc.TrackInfo{Title: "Arsenal", Artist: "Slipknot", Album: "Arsenal", Path: "https://music.youtube.com/watch?v=x"}}
	out := waybarLine(snap, "{icon} {artist} - {title}", 45)
	if out.Text != "▶ Slipknot - Arsenal" || out.Class != "playing" || out.Percentage != 25 {
		t.Fatalf("out = %+v", out)
	}
	if !strings.Contains(out.Tooltip, "0:30 / 2:00") {
		t.Fatalf("tooltip = %q", out.Tooltip)
	}
	snap.State = "paused"
	snap.Track.Artist = ""
	if out := waybarLine(snap, "{icon} {artist} - {title}", 45); out.Text != "⏸ Arsenal" || out.Class != "paused" {
		t.Fatalf("paused without artist = %+v", out)
	}
	long := waybarLine(&ipc.RuntimeSnapshot{State: "playing", Track: &ipc.TrackInfo{Title: strings.Repeat("x", 80)}}, "{title}", 10)
	if len([]rune(long.Text)) != 10 || !strings.HasSuffix(long.Text, "…") {
		t.Fatalf("long text = %q", long.Text)
	}
	if out := waybarLine(&ipc.RuntimeSnapshot{}, "{title}", 10); out.Class != "stopped" || out.Text != "" {
		t.Fatalf("stopped = %+v", out)
	}
}
