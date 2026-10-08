package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ======================================================================
// cli helpers
// ======================================================================

// ======================================================================
// styles
// ======================================================================

var (
	cAccent = lipgloss.Color("#8B7BFF")
	cGood   = lipgloss.Color("#3DDC84")
	cBad    = lipgloss.Color("#FF5C5C")
	cWarn   = lipgloss.Color("#FFB454")
	cDim    = lipgloss.Color("#7A7F8C")
)

func colorize(c lipgloss.Color, s string) string {
	return lipgloss.NewStyle().Foreground(c).Render(s)
}
func good(s string) string   { return colorize(cGood, s) }
func bad(s string) string    { return colorize(cBad, s) }
func warnS(s string) string  { return colorize(cWarn, s) }
func dimS(s string) string   { return colorize(cDim, s) }
func accent(s string) string { return colorize(cAccent, s) }
func bold(s string) string   { return lipgloss.NewStyle().Bold(true).Render(s) }
func accentBold(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(cAccent).Render(s)
}
func goodBold(s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(cGood).Render(s)
}

func latencyStyled(ms float64) string {
	s := fmt.Sprintf("%6.1f ms", ms)
	switch {
	case ms < 80:
		return good(s)
	case ms < 200:
		return warnS(s)
	default:
		return bad(s)
	}
}

func panel(title string, lines []string, outerW, outerH int, border lipgloss.Color) string {
	if outerW < 10 {
		outerW = 10
	}
	if outerH < 3 {
		outerH = 3
	}
	inner := outerW - 4
	maxLines := outerH - 2
	var out []string
	if title != "" {
		out = append(out, bold(title))
	}
	out = append(out, lines...)
	if len(out) > maxLines {
		out = out[:maxLines]
	}
	for i := range out {
		out[i] = lipgloss.NewStyle().MaxWidth(inner).Render(out[i])
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(outerW - 2).
		Height(outerH - 2).
		Render(strings.Join(out, "\n"))
}

func helpBar(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, accentBold(pairs[i])+" "+dimS(pairs[i+1]))
	}
	return " " + strings.Join(parts, dimS("  ·  "))
}

func progressBar(frac float64, w int) string {
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	if w < 5 {
		w = 5
	}
	f := int(frac * float64(w))
	return accent(strings.Repeat("█", f)) + dimS(strings.Repeat("░", w-f))
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	h := int(d.Hours())
	mi := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, mi)
	}
	if mi > 0 {
		return fmt.Sprintf("%dm %02ds", mi, s)
	}
	return fmt.Sprintf("%ds", s)
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
