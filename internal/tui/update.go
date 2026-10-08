package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"speedify-check/internal/model"
	"speedify-check/internal/report"
)

// ======================================================================
// update
// ======================================================================

// Update processes one message: key presses, terminal resize, CLI
// check results, connect outcomes and checker engine events.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		m.fixMenuView()
		m.fixTableView()
		return m, nil

	case tickMsg:
		m.spin++
		return m, tickCmd()

	case checkMsg:
		m.checking = false
		m.cliFound = msg.CLIFound
		m.loadErr = msg.Err
		if msg.Servers != nil {
			m.servers = msg.Servers
			m.countries = buildCountries(msg.Servers)
		}
		return m, nil

	case tea.KeyMsg:
		return m.onKey(msg)

	// ----- events from the checker engine -----
	case eventMsg:
		return m.onEvent(msg.ev)

	case connectedMsg:
		if msg.Err != nil {
			m.status = bad("✘ Could not connect to " + msg.Name)
		} else {
			m.status = good("✔ Connected to " + msg.Name)
		}
		return m, nil

	case disconnectedMsg:
		if msg.Err != nil {
			m.status = bad("✘ Disconnect failed")
		} else {
			m.status = dimS("Disconnected")
		}
		return m, nil
	}
	return m, nil
}

// onEvent adapts one checker engine event to UI state and keeps the
// engine event loop running by re-arming the wait command.
func (m Model) onEvent(ev model.Event) (tea.Model, tea.Cmd) {
	switch ev := ev.(type) {
	case model.LogEvent:
		m.addLog(warnS("↻"), warnS(ev.Text))
		return m, waitEvent(m.events)

	case model.ServerStartEvent:
		m.curName = ev.Name
		m.curPhase = "connecting"
		m.curStart = time.Now()
		m.samples = nil
		m.curConnT, m.curIP = 0, ""
		m.addLog(dimS("▶"), dimS(fmt.Sprintf("%s  (%d/%d)", ev.Name, ev.Idx, ev.Total)))
		return m, waitEvent(m.events)

	case model.PhaseEvent:
		m.curPhase = ev.Phase
		if ev.Phase == "connecting" {
			m.curStart = time.Now()
			m.samples = nil
		} else {
			m.curConnT = ev.ConnectTime
			m.curIP = ev.IP
		}
		return m, waitEvent(m.events)

	case model.SampleEvent:
		m.samples = append(m.samples, ev)
		return m, waitEvent(m.events)

	case model.ServerDoneEvent:
		r := ev.Result
		m.results = append(m.results, r)
		m.doneN++
		if r.Stable {
			m.runStable++
			m.addLog(good("✔"), fmt.Sprintf("%s  %s",
				goodBold(fmt.Sprintf("%-26s", r.Server)),
				dimS(fmt.Sprintf("avg %.1fms · jitter %.1f · loss %d/%d · conn %.1fs",
					r.AvgMs, r.JitterMs, r.ProbeLoss, r.Samples, r.ConnectTime))))
		} else {
			m.runFailed++
			m.addLog(bad("✘"), fmt.Sprintf("%s  %s",
				bad(fmt.Sprintf("%-26s", r.Server)), dimS(r.Reason)))
		}
		return m, waitEvent(m.events)

	case model.FinishedEvent:
		m.results = ev.All
		m.rank = report.RankStable(m.results)
		m.rcur, m.rtop = 0, 0
		m.stage = stDone
		m.stopping = false
		m.status = ""
		return m, nil
	}
	return m, nil
}

// View renders the current screen.
func (m Model) View() string {
	title := accentBold("⚡ Speedify Server Checker")
	var body, help string

	switch m.stage {
	case stNotice:
		title += dimS("  ·  welcome")
		body = m.viewNotice()
		help = helpBar("Enter", "continue", "r", "re-check", "q", "quit")
	case stMenu:
		title += dimS("  ·  choose countries")
		body = m.viewMenu()
		help = helpBar("↑/↓", "move", "Space", "toggle", "a", "all", "n", "none", "Enter", "continue", "q", "quit")
	case stConfirm:
		title += dimS("  ·  confirm")
		body = m.viewConfirm()
		help = helpBar("Y/Enter", "start", "N/Esc", "go back")
	case stRun:
		title += dimS("  ·  testing")
		body = m.viewRun()
		if m.stopping {
			help = helpBar("q", "force quit")
		} else {
			help = helpBar("q", "stop & show results")
		}
	case stDone:
		title += dimS("  ·  results")
		body = m.viewDone()
		help = helpBar("↑/↓", "move", "Enter", "connect to selected", "d", "disconnect", "r", "new run", "q", "quit")
	}
	return title + "\n" + body + "\n" + help
}

// ---------- notice ----------
