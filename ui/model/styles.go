package model

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/xaviiromf/cliamp-x/ui"
)

// Model-specific lipgloss styles. They have no initializers: rebuildModelStyles
// builds them from init for the default palette and again on every theme
// change. A style omitted there renders unstyled under every theme.
var (
	titleStyle               lipgloss.Style
	trackStyle               lipgloss.Style
	timeStyle                lipgloss.Style
	statusStyle              lipgloss.Style
	feedbackActivityStyle    lipgloss.Style
	feedbackSuccessStyle     lipgloss.Style
	feedbackWarningStyle     lipgloss.Style
	dimStyle                 lipgloss.Style
	labelStyle               lipgloss.Style
	eqActiveStyle            lipgloss.Style
	eqInactiveStyle          lipgloss.Style
	playlistActiveStyle      lipgloss.Style
	playlistItemStyle        lipgloss.Style
	playlistSelectedStyle    lipgloss.Style
	playlistUnavailableStyle lipgloss.Style
	helpStyle                lipgloss.Style
	helpKeyStyle             lipgloss.Style
	errorStyle               lipgloss.Style

	seekFillStyle lipgloss.Style
	seekDimStyle  lipgloss.Style
	seekSkipStyle lipgloss.Style
	volBarStyle   lipgloss.Style
	activeToggle  lipgloss.Style
	// favMarkerStyle paints the favorite heart in the theme's red so it
	// reads as a deliberate accent instead of inheriting the dim/unavailable
	// look. The glyph carries U+FE0E (text presentation) so terminals render
	// it as a compact font glyph rather than a large color emoji.
	favMarkerStyle lipgloss.Style
	// favRemovedStyle mutes the same filled heart for unfavorite feedback:
	// identical attractive glyph, faded to signal the removed state instead
	// of switching to a thin outline glyph.
	favRemovedStyle lipgloss.Style
)

func init() { rebuildModelStyles() }

// rebuildModelStyles reconstructs all model-specific lipgloss styles from current color variables.
func rebuildModelStyles() {
	titleStyle = lipgloss.NewStyle().Foreground(ui.ColorTitle).Bold(true)
	trackStyle = lipgloss.NewStyle().Foreground(ui.ColorAccent)
	timeStyle = lipgloss.NewStyle().Foreground(ui.ColorText)
	statusStyle = lipgloss.NewStyle().Foreground(ui.ColorPlaying).Bold(true)
	feedbackActivityStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	feedbackSuccessStyle = lipgloss.NewStyle().Foreground(ui.ColorPlaying).Bold(true)
	feedbackWarningStyle = lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
	dimStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	labelStyle = lipgloss.NewStyle().Foreground(ui.ColorText).Bold(true)
	eqActiveStyle = lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	eqInactiveStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	playlistActiveStyle = lipgloss.NewStyle().Foreground(ui.ColorPlaying).Bold(true)
	playlistItemStyle = lipgloss.NewStyle().Foreground(ui.ColorText)
	playlistSelectedStyle = lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	playlistUnavailableStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	helpStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	helpKeyStyle = lipgloss.NewStyle().Foreground(ui.ColorKeyFG).Background(ui.ColorKeyBG).Bold(true)
	errorStyle = lipgloss.NewStyle().Foreground(ui.ColorError)

	seekFillStyle = lipgloss.NewStyle().Foreground(ui.ColorSeekBar)
	seekDimStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
	seekSkipStyle = lipgloss.NewStyle().Foreground(ui.ColorWarning)
	volBarStyle = lipgloss.NewStyle().Foreground(ui.ColorVolume)
	activeToggle = lipgloss.NewStyle().Foreground(ui.ColorAccent).Bold(true)
	favMarkerStyle = lipgloss.NewStyle().Foreground(ui.ColorError)
	favRemovedStyle = lipgloss.NewStyle().Foreground(ui.ColorDim)
}

// paint draws s in style st exactly as st.Render(s) does, without the layout
// work Render runs for widths, padding, borders and margins. The list rows
// and hints paint many short pieces each frame, and their styles only set
// colors and bold. Any other style or a multi-line text takes Render.
func paint(st lipgloss.Style, s string) string {
	if s == "" || strings.ContainsAny(s, "\n\t\r") || !colorOnlyStyle(st) {
		return st.Render(s)
	}
	var te ansi.Style
	if st.GetBold() {
		te = te.Bold()
	}
	if st.GetItalic() {
		te = te.Italic(true)
	}
	if st.GetFaint() {
		te = te.Faint()
	}
	if fg := st.GetForeground(); fg != (lipgloss.NoColor{}) {
		te = te.ForegroundColor(fg)
	}
	if bg := st.GetBackground(); bg != (lipgloss.NoColor{}) {
		te = te.BackgroundColor(bg)
	}
	return te.Styled(s)
}

// colorOnlyStyle reports whether st sets nothing that changes the layout or
// the spaces of a one-line text: only colors, bold, italic and faint.
func colorOnlyStyle(st lipgloss.Style) bool {
	link, _ := st.GetHyperlink()
	pt, pr, pb, pl := st.GetPadding()
	mt, mr, mb, ml := st.GetMargin()
	border, bt, br, bb, bl := st.GetBorder()
	return st.GetWidth() == 0 && st.GetHeight() == 0 && st.GetMaxWidth() == 0 && st.GetMaxHeight() == 0 &&
		pt|pr|pb|pl|mt|mr|mb|ml == 0 && border == (lipgloss.Border{}) && !bt && !br && !bb && !bl &&
		!st.GetUnderline() && !st.GetStrikethrough() && !st.GetReverse() && !st.GetBlink() &&
		!st.GetUnderlineSpaces() && !st.GetStrikethroughSpaces() && st.GetTransform() == nil && link == "" &&
		st.Value() == ""
}

// textWidth is lipgloss.Width for the lines the frame draws every tick. A
// line of ASCII and the narrowRune characters, plus SGR and OSC sequences, is
// counted one cell per character without the grapheme segmentation that
// dominates frame time. Any other character takes lipgloss.Width, so wide,
// combining and joined characters stay exact.
func textWidth(s string) int {
	w := 0
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == 0x1b:
			n := escapeLen(s[i:])
			if n == 0 {
				return lipgloss.Width(s)
			}
			i += n
		case c >= 0x20 && c < 0x7f:
			w++
			i++
		case c < 0x80:
			return lipgloss.Width(s)
		default:
			r, size := utf8.DecodeRuneInString(s[i:])
			if !narrowRune(r) {
				return lipgloss.Width(s)
			}
			w++
			i += size
		}
	}
	return w
}

// escapeLen returns the length of the CSI or OSC sequence at the start of s,
// or 0 for anything else.
func escapeLen(s string) int {
	if len(s) < 2 {
		return 0
	}
	switch s[1] {
	case '[':
		for i := 2; i < len(s); i++ {
			if c := s[i]; c >= 0x40 && c <= 0x7e {
				return i + 1
			} else if c < 0x20 || c > 0x7e {
				return 0
			}
		}
	case ']':
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
	}
	return 0
}

// narrowRune reports whether r is one cell wide on its own and starts no
// grapheme cluster with what follows it, unless what follows is outside this
// set too (which sends the line to lipgloss.Width). It covers Latin letters
// with accents, general punctuation, arrows, box drawing, blocks, geometric
// shapes, braille and the music, heart and star signs the interface draws.
// TestNarrowRunesAreNarrow checks every one against lipgloss.
func narrowRune(r rune) bool {
	switch {
	case r >= 0xa0 && r <= 0x24f:
		return r != 0xad // the soft hyphen has no width
	case r == 0x25fd || r == 0x25fe: // emoji squares, two cells wide
		return false
	case r >= 0x2010 && r <= 0x2027, r >= 0x2190 && r <= 0x21ff, r >= 0x2500 && r <= 0x25ff, r >= 0x2800 && r <= 0x28ff:
		return true
	}
	switch r {
	case '♥', '♡', '♪', '♫', '★', '☆', '✓', '✗', '❚':
		return true
	}
	return false
}
