package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) viewRun() string {
	bodyH := m.h - 3
	statusH := 6
	panelH := bodyH - statusH
	if panelH < 5 {
		panelH = 5
	}

	spin := spinFrames[m.spin%len(spinFrames)]
	elapsed := time.Since(m.startAt)

	// line 1: what we are doing
	var l1 string
	if m.stopping {
		l1 = warnS(spin + "  Stopping…")
	} else {
		cur := imin(m.doneN+1, m.totalN)
		l1 = accent(spin) + "  " + bold("Testing") + fmt.Sprintf(" [%d/%d]  ", cur, m.totalN) + accentBold(m.curName)
	}

	// line 2: progress
	frac := 0.0
	if m.totalN > 0 {
		frac = float64(m.doneN) / float64(m.totalN)
	}
	eta := "…"
	if m.doneN > 0 {
		per := elapsed / time.Duration(m.doneN)
		eta = fmtDur(per * time.Duration(m.totalN-m.doneN))
	}
	barW := m.w - 52
	if barW > 50 {
		barW = 50
	}
	l2 := progressBar(frac, barW) + fmt.Sprintf("  %3.0f%%  ", frac*100) +
		dimS("elapsed "+fmtDur(elapsed)+" · ETA "+eta)

	// line 3: live phase
	var l3 string
	switch m.curPhase {
	case "connecting":
		l3 = dimS("Connecting… ") + fmt.Sprintf("%.1fs", time.Since(m.curStart).Seconds())
	case "holding":
		var dots strings.Builder
		for i := 0; i < m.totalSamples; i++ {
			if i < len(m.samples) {
				s := m.samples[i]
				switch {
				case !s.StateOK:
					dots.WriteString(bad("✘"))
				case !s.ProbeOK:
					dots.WriteString(warnS("✘"))
				default:
					dots.WriteString(good("●"))
				}
			} else {
				dots.WriteString(dimS("·"))
			}
		}
		last := ""
		if n := len(m.samples); n > 0 && m.samples[n-1].ProbeOK {
			last = "  " + latencyStyled(m.samples[n-1].RTT)
		}
		l3 = dimS("Holding ") + dots.String() + last +
			dimS(fmt.Sprintf("   connected in %.1fs · %s", m.curConnT, m.curIP))
	default:
		l3 = dimS("Starting…")
	}

	// line 4: counters
	l4 := good(fmt.Sprintf("✔ stable %d", m.runStable)) + "   " +
		bad(fmt.Sprintf("✘ failed %d", m.runFailed))
	if m.skipped > 0 {
		l4 += "   " + dimS(fmt.Sprintf("⏭ skipped %d", m.skipped))
	}

	status := panel("", []string{l1, l2, l3, l4}, m.w, statusH, cAccent)

	// log + stable panels
	wL := m.w * 58 / 100
	wR := m.w - wL
	capacity := panelH - 3
	if capacity < 1 {
		capacity = 1
	}

	var logLines []string
	start := 0
	if len(m.logs) > capacity {
		start = len(m.logs) - capacity
	}
	for _, e := range m.logs[start:] {
		logLines = append(logLines, dimS(e.T.Format("15:04:05"))+"  "+e.Icon+"  "+e.Text)
	}
	left := panel("Live log", logLines, wL, panelH, cDim)

	var stLines []string
	var stableAll []Result
	for _, r := range m.results {
		if r.Stable {
			stableAll = append(stableAll, r)
		}
	}
	for i := len(stableAll) - 1; i >= 0 && len(stLines) < capacity; i-- {
		r := stableAll[i]
		stLines = append(stLines, good("✔")+" "+bold(fmt.Sprintf("%-24s", r.Server))+" "+latencyStyled(r.AvgMs))
	}
	right := panel(fmt.Sprintf("✔ Stable servers (%d)", len(stableAll)), stLines, wR, panelH, cGood)

	return status + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

// ---------- results ----------
