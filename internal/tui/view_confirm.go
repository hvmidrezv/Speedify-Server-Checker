package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewConfirm() string {
	todo, skipped := m.buildTodo()
	sc, _ := m.selectedCounts()

	var lines []string
	if len(todo) == 0 {
		lines = []string{
			bold("Nothing to test"),
			"",
			"All selected servers were already tested (-resume).",
			"",
			accentBold("[N]") + " Go back",
		}
	} else {
		est := time.Duration(len(todo)) * (m.cfg.Hold + 8*time.Second)
		lines = []string{
			accentBold("Ready to start?"),
			"",
			fmt.Sprintf("Found %s servers in %s countries.", goodBold(fmt.Sprint(len(todo))), bold(fmt.Sprint(sc))),
		}
		if skipped > 0 {
			lines = append(lines, dimS(fmt.Sprintf("(%d already tested, skipped)", skipped)))
		}
		lines = append(lines,
			fmt.Sprintf("Estimated time: %s", bold("~"+fmtDur(est))),
			"",
			warnS("⚠ Your connection will drop and reconnect many times."),
			"",
			goodBold("[Y]")+" Yes, start      "+bad("[N]")+" No, go back",
		)
	}
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cWarn).
		Padding(1, 4).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.w, m.h-3, lipgloss.Center, lipgloss.Center, box)
}

// ---------- running ----------
