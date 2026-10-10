package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/xaviiromf/cliamp-x/ipc"
)

// cliamp-x waybar prints the playing song as a Waybar custom module: one JSON
// line each time the playback changes, and an empty module while cliamp-x
// does not run. Waybar's on-click and on-scroll settings call the other
// subcommands (toggle, next, prev, volume).

// waybarOutput is the JSON a Waybar custom module with return-type json
// reads.
type waybarOutput struct {
	Text       string `json:"text"`
	Tooltip    string `json:"tooltip"`
	Class      string `json:"class"`
	Alt        string `json:"alt"`
	Percentage int    `json:"percentage"`
}

// waybarRetry is how long the module waits before it looks for cliamp-x
// again.
const waybarRetry = 2 * time.Second

func waybarCommand() *cli.Command {
	return &cli.Command{
		Name:  "waybar",
		Usage: "print the playing song for a Waybar custom module (JSON lines)",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "{icon} {artist} - {title}", Usage: "text template: {icon} {artist} {title} {album}"},
			&cli.IntFlag{Name: "max-length", Value: 45, Usage: "longest text in characters, 0 for no limit"},
		},
		Action: func(ctx context.Context, c *cli.Command) error {
			return runWaybar(ctx, os.Stdout, c.String("format"), int(c.Int("max-length")))
		},
	}
}

func runWaybar(ctx context.Context, w io.Writer, format string, maxLen int) error {
	enc := json.NewEncoder(w)
	emit := func(out waybarOutput) error { return enc.Encode(out) }
	for {
		stream, err := ipc.SubscribeV2(ipc.DefaultSocketPath(), json.RawMessage(`"waybar"`), []string{"runtime.playback"})
		if err != nil {
			if err := emit(waybarOutput{Class: "stopped", Alt: "stopped"}); err != nil {
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(waybarRetry):
			}
			continue
		}
		for {
			ev, err := stream.Next()
			if err != nil {
				break
			}
			var snap ipc.RuntimeSnapshot
			if json.Unmarshal(ev.Data, &snap) != nil {
				continue
			}
			if err := emit(waybarLine(&snap, format, maxLen)); err != nil {
				stream.Close()
				return err
			}
		}
		stream.Close()
		if ctx.Err() != nil {
			return nil
		}
	}
}

// waybarLine builds the module output for a runtime snapshot.
func waybarLine(snap *ipc.RuntimeSnapshot, format string, maxLen int) waybarOutput {
	state := snap.State
	if state == "" || snap.Track == nil || (snap.Track.Title == "" && snap.Track.Path == "") {
		return waybarOutput{Class: "stopped", Alt: "stopped"}
	}
	icon := map[string]string{"playing": "▶", "paused": "⏸"}[state]
	if icon == "" {
		icon = "■"
	}
	t := snap.Track
	title := t.Title
	if title == "" {
		title = t.Path
	}
	text := strings.NewReplacer("{icon}", icon, "{artist}", t.Artist, "{title}", title, "{album}", t.Album).Replace(format)
	if t.Artist == "" {
		text = strings.ReplaceAll(text, " - ", "")
	}
	text = strings.TrimSpace(text)
	if r := []rune(text); maxLen > 0 && len(r) > maxLen {
		text = string(r[:maxLen-1]) + "…"
	}
	tooltip := title
	if t.Artist != "" {
		tooltip = t.Artist + " — " + title
	}
	if t.Album != "" {
		tooltip += "\n" + t.Album
	}
	pct := 0
	if snap.Duration > 0 {
		pct = int(100 * snap.Position / snap.Duration)
		tooltip += fmt.Sprintf("\n%s / %s", clock(snap.Position), clock(snap.Duration))
	}
	return waybarOutput{Text: text, Tooltip: tooltip, Class: state, Alt: state, Percentage: max(0, min(100, pct))}
}

func clock(secs float64) string {
	s := int(secs)
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}
