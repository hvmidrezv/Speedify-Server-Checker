package tui

import (
	"fmt"
	"path/filepath"
)

func (m Model) viewDone() string {
	bodyH := m.h - 3
	topH := 6
	tableH := bodyH - topH
	if tableH < 6 {
		tableH = 6
	}

	failed := map[string]int{}
	tested := len(m.results)
	for _, r := range m.results {
		if !r.Stable {
			failed[r.Category]++
		}
	}
	abs, err := filepath.Abs(m.cfg.OutDir)
	if err != nil {
		abs = m.cfg.OutDir
	}
	nFail := tested - len(m.rank)

	statusLine := m.status
	if statusLine == "" {
		statusLine = dimS("Select a server and press Enter to connect to it.")
	}
	top := panel("", []string{
		bold("Done") + fmt.Sprintf(" · tested %d · ", tested) + good(fmt.Sprintf("✔ stable %d", len(m.rank))) +
			" · " + bad(fmt.Sprintf("✘ failed %d", nFail)),
		dimS(fmt.Sprintf("Failures: connect timeout %d · dropped %d · traffic loss %d",
			failed["connect_timeout"], failed["dropped"], failed["loss"])),
		dimS("Saved in " + abs + ": stable_servers.txt · results.csv · results.json · speedify_check.log"),
		statusLine,
	}, m.w, topH, cAccent)

	var lines []string
	if len(m.rank) == 0 {
		lines = []string{
			"",
			warnS("No stable server found in this run."),
			dimS("Try: -connect-timeout 40s   -max-loss 1   -retries 1   or a shorter -hold"),
		}
	} else {
		lines = append(lines, dimS(fmt.Sprintf("  %-4s %-28s %9s %9s %9s %8s  %s",
			"#", "SERVER", "AVG ms", "JIT ms", "MAX ms", "CONN s", "EXIT IP")))
		rows := m.tableRows()
		end := m.rtop + rows
		if end > len(m.rank) {
			end = len(m.rank)
		}
		for i := m.rtop; i < end; i++ {
			r := m.rank[i]
			line := fmt.Sprintf("%-4d %-28s %9.1f %9.1f %9.1f %8.1f  %s",
				i+1, r.Server, r.AvgMs, r.JitterMs, r.MaxMs, r.ConnectTime, r.ExitIP)
			if i == m.rcur {
				lines = append(lines, accentBold("▸ "+line))
			} else if i == 0 {
				lines = append(lines, "  "+goodBold(line))
			} else {
				lines = append(lines, "  "+line)
			}
		}
	}
	table := panel("🏆 Stable servers, best first", lines, m.w, tableH, cGood)
	return top + "\n" + table
}
