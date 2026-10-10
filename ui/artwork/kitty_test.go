package artwork

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKittyPlaceholders(t *testing.T) {
	lines := kittyPlaceholders(0x0A0B0C, 3, 2)
	if len(lines) != 2 {
		t.Fatalf("rows = %d, want 2", len(lines))
	}
	// Row 1, column 2: U+10EEEE, the diacritic for 1, the diacritic for 2.
	if want := "\U0010EEEE\u030D\u030E"; !strings.Contains(lines[1], want) {
		t.Fatalf("row 1 = %q, want the cell %q", lines[1], want)
	}
	if !strings.HasPrefix(lines[0], "\x1b[38;2;10;11;12m") {
		t.Fatalf("row 0 = %q, want the image ID in the foreground colour", lines[0])
	}
	if n := strings.Count(lines[0], "\U0010EEEE"); n != 3 {
		t.Fatalf("row 0 cells = %d, want 3", n)
	}
}

func TestKittyTransmitChunks(t *testing.T) {
	seq := kittyTransmit(7, 4, 2, make([]byte, 10000))
	if !strings.HasPrefix(seq, "\x1b_Ga=T,U=1,q=2,f=100,i=7,c=4,r=2,m=1;") {
		t.Fatalf("first chunk header = %q", seq[:60])
	}
	if strings.Count(seq, "\x1b_G") != 4 || !strings.Contains(seq, "\x1b_Gm=0;") {
		t.Fatalf("10000 bytes must go in 4 chunks ending with m=0: %d chunks", strings.Count(seq, "\x1b_G"))
	}
}

func TestKittyUpdateFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(path, testPNG(t, 200, 200), 0o644); err != nil {
		t.Fatal(err)
	}
	c := New("")
	if send := c.KittyUpdate(path, 20, 10); send != nil {
		t.Fatal("KittyUpdate with kitty off must do nothing")
	}
	c.EnableKitty()
	if send := c.KittyUpdate(path, 20, 10); send != nil {
		t.Fatal("KittyUpdate before the image loads must do nothing")
	}
	c.Claim(path)
	if err := c.Load(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	send := c.KittyUpdate(path, 20, 10)
	if send == nil {
		t.Fatal("KittyUpdate of a loaded image = nil, want a transmit")
	}
	if again := c.KittyUpdate(path, 20, 10); again != nil {
		t.Fatal("a second KittyUpdate while sending must do nothing")
	}
	if seq := send(); !strings.HasPrefix(seq, "\x1b_Ga=T,U=1") {
		t.Fatalf("transmit = %q", seq[:min(40, len(seq))])
	}
	if _, _, _, ok := c.KittyDraw(path, 20, 10); ok {
		t.Fatal("KittyDraw before KittyShown must fall back to half blocks")
	}
	c.KittyShown(path)
	lines, cols, rows, ok := c.KittyDraw(path, 20, 10)
	if !ok || cols != 20 || rows != 10 || len(lines) != 10 {
		t.Fatalf("KittyDraw = %d lines %dx%d %v, want 10 lines 20x10", len(lines), cols, rows, ok)
	}

	// A new size only moves the placement.
	place := c.KittyUpdate(path, 12, 6)
	if place == nil || !strings.Contains(place(), "a=p,U=1") {
		t.Fatal("a new size must send a new placement, not the image")
	}
}
