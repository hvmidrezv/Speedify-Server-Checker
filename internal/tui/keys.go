package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := k.String()

	// global quit / stop
	if key == "ctrl+c" || (key == "q" && m.stage != stConfirm) {
		if m.stage == stRun {
			if !m.stopping {
				m.stopping = true
				m.addLog(warnS("■"), warnS("Stopping… finishing current step (press q again to force quit)"))
				if m.cancel != nil {
					m.cancel()
				}
				return m, nil
			}
			m.client().ForceDisconnect()
			return m, tea.Quit
		}
		return m, tea.Quit
	}

	switch m.stage {

	case stNotice:
		switch key {
		case "enter":
			if len(m.servers) > 0 {
				m.stage = stMenu
				m.fixMenuView()
			}
		case "r":
			m.checking = true
			m.loadErr = ""
			return m, m.checkCmd()
		}

	case stMenu:
		switch key {
		case "up", "k":
			m.moveMenu(-1)
		case "down", "j":
			m.moveMenu(1)
		case "pgup":
			m.moveMenu(-m.menuRows())
		case "pgdown":
			m.moveMenu(m.menuRows())
		case "home", "g":
			m.cur = 0
			m.fixMenuView()
		case "end", "G":
			m.cur = len(m.countries)
			m.fixMenuView()
		case " ", "space", "x":
			m.toggle()
		case "a":
			for i := range m.countries {
				m.countries[i].Selected = true
			}
			m.menuMsg = ""
		case "n":
			for i := range m.countries {
				m.countries[i].Selected = false
			}
			m.menuMsg = ""
		case "esc":
			m.stage = stNotice
		case "enter":
			if _, n := m.selectedCounts(); n == 0 {
				m.menuMsg = warnS("⚠ Select at least one country first (Space)")
			} else {
				m.stage = stConfirm
			}
		}

	case stConfirm:
		switch key {
		case "y", "Y", "enter":
			cmd := m.startRun()
			return m, cmd
		case "n", "N", "esc", "q":
			m.stage = stMenu
		}

	case stRun:
		// only stop is handled above

	case stDone:
		switch key {
		case "up", "k":
			if m.rcur > 0 {
				m.rcur--
			}
			m.fixTableView()
		case "down", "j":
			if m.rcur < len(m.rank)-1 {
				m.rcur++
			}
			m.fixTableView()
		case "pgup":
			m.rcur -= m.tableRows()
			if m.rcur < 0 {
				m.rcur = 0
			}
			m.fixTableView()
		case "pgdown":
			m.rcur += m.tableRows()
			if m.rcur > len(m.rank)-1 {
				m.rcur = len(m.rank) - 1
			}
			if m.rcur < 0 {
				m.rcur = 0
			}
			m.fixTableView()
		case "enter":
			if len(m.rank) > 0 {
				r := m.rank[m.rcur]
				m.status = warnS("… connecting to " + r.Server)
				return m, m.connectCmd(r)
			}
		case "d":
			m.status = dimS("… disconnecting")
			return m, m.disconnectCmd()
		case "r":
			m.stage = stMenu
			m.status = ""
			m.fixMenuView()
		}
	}
	return m, nil
}
