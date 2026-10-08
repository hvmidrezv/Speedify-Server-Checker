package tui

import (
	"fmt"
)

func checkbox(on bool) string {
	if on {
		return good("[✔]")
	}
	return dimS("[ ]")
}

func (m Model) menuRow(i int) string {
	cursor := "  "
	if i == m.cur {
		cursor = accentBold("▸ ")
	}
	var box, label string
	var count int
	if i == 0 {
		box = checkbox(m.allSelected())
		label = "🌍  " + fmt.Sprintf("%-22s", "All countries")
		count = m.totalServers()
	} else {
		c := m.countries[i-1]
		box = checkbox(c.Selected)
		label = c.Flag + "  " + fmt.Sprintf("%-22s", c.Name)
		count = len(c.Servers)
	}
	if i == m.cur {
		label = accentBold(label)
	}
	return cursor + box + " " + label + " " + dimS(fmt.Sprintf("%4d servers", count))
}

func (m Model) viewMenu() string {
	rows := m.menuRows()
	total := len(m.countries) + 1
	end := m.top + rows
	if end > total {
		end = total
	}
	var lines []string
	lines = append(lines, dimS("      COUNTRY                         SERVERS"))
	for i := m.top; i < end; i++ {
		lines = append(lines, m.menuRow(i))
	}
	lines = append(lines, "")
	sc, ss := m.selectedCounts()
	summary := fmt.Sprintf("Selected: %s countries · %s servers   %s",
		bold(fmt.Sprint(sc)), bold(fmt.Sprint(ss)), dimS(fmt.Sprintf("(%d/%d)", m.cur+1, total)))
	if m.menuMsg != "" {
		summary += "   " + m.menuMsg
	}
	lines = append(lines, summary)
	return panel("Choose which countries to test", lines, m.w, m.h-3, cAccent)
}

// ---------- confirm popup ----------
