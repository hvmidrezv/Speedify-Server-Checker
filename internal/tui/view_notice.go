package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewNotice() string {
	boxW := imin(76, m.w-4)
	if boxW < 40 {
		boxW = 40
	}

	var checks []string
	switch {
	case m.checking:
		checks = append(checks, accent(spinFrames[m.spin%len(spinFrames)])+" Checking Speedify…")
	case !m.cliFound:
		checks = append(checks,
			bad("✘ speedify_cli was not found"),
			dimS("  Install Speedify, or run with: -cli /path/to/speedify_cli"))
	case m.loadErr != "":
		checks = append(checks,
			good("✔ speedify_cli found"),
			bad("✘ "+m.loadErr),
			dimS("  Make sure you are logged in to Speedify, then press r"))
	default:
		checks = append(checks,
			good("✔ speedify_cli found"),
			good(fmt.Sprintf("✔ Logged in · %d servers in %d countries", len(m.servers), len(m.countries))))
	}

	lines := []string{
		warnS("⚠  READ THIS BEFORE YOU START"),
		"",
		"Speedify must be " + bold("INSTALLED") + " on this computer.",
		"You must be " + bold("LOGGED IN") + " to your Speedify account.",
		"Without both, no server can be tested.",
		"",
		dimS("Heads-up: your connection will drop and reconnect many times"),
		dimS("during the test. Pause downloads and calls first."),
		"",
		bold("Checks"),
	}
	lines = append(lines, checks...)
	if !m.checking && len(m.servers) > 0 {
		lines = append(lines, "", accentBold("Press Enter to continue ▸"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cWarn).
		Padding(1, 3).
		Width(boxW).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(m.w, m.h-3, lipgloss.Center, lipgloss.Center, box)
}

// ---------- menu ----------
