// Speedify Server Checker - TUI edition
//
// Setup (once, in an empty folder containing only this file):
//
//	go mod init speedify-check
//	go mod tidy
//
// Run:
//
//	go run .
//	go run . -hold 15s -retries 1 -max-loss 1
//
// Flow: notice -> country menu -> confirm popup -> live test screen -> ranked results
package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"speedify-check/internal/checker"
	"speedify-check/internal/config"
	"speedify-check/internal/geo"
	"speedify-check/internal/logx"
	// Aliased because this file declares its own Bubble Tea "model"
	// struct; the alias disappears when the UI moves to internal/tui.
	domain "speedify-check/internal/model"
	"speedify-check/internal/probe"
	"speedify-check/internal/report"
	"speedify-check/internal/speedify"
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

// ======================================================================
// data
// ======================================================================

// Server and Result are aliases for the domain types that now live in
// the model package. The aliases keep this file compiling unchanged
// until the UI is moved into internal/tui.
type (
	Server = domain.Server
	Result = domain.Result
)

// country is one row of the country selection menu: a model.Country
// grouping plus its selection state.
type country struct {
	domain.Country
	Selected bool
}

func buildCountries(sv []Server) []country {
	m := map[string]*country{}
	for _, s := range sv {
		code := strings.ToLower(s.Country)
		c, ok := m[code]
		if !ok {
			c = &country{Country: domain.Country{Code: code, Name: geo.CountryName(code), Flag: geo.FlagEmoji(code)}}
			m[code] = c
		}
		c.Servers = append(c.Servers, s)
	}
	out := make([]country, 0, len(m))
	for _, c := range m {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ======================================================================
// file log (full detail, always written)
// ======================================================================

// appLog is configured by main; before that (and if the file cannot be
// opened) it is nil and logging is a no-op.
var appLog *logx.Logger

// ======================================================================
// speedify cli helpers
// ======================================================================

// client builds the speedify CLI client from the current config.
func (m model) client() *speedify.Client {
	return speedify.NewClient(speedify.NewExecRunner(m.cfg.CLIPath))
}

// ======================================================================
// messages
// ======================================================================

type tickMsg time.Time
type checkMsg struct {
	CLIFound bool
	Err      string
	Servers  []Server
}

// eventMsg wraps one event from the checker engine so Bubble Tea can
// deliver it through the update loop.
type eventMsg struct{ ev domain.Event }

type connectedMsg struct {
	Name string
	Err  error
}
type disconnectedMsg struct{ Err error }

// ======================================================================
// model
// ======================================================================

type stage int

const (
	stNotice stage = iota
	stMenu
	stConfirm
	stRun
	stDone
)

type logEntry struct {
	T    time.Time
	Icon string
	Text string
}

type model struct {
	cfg config.Config

	w, h  int
	stage stage
	spin  int

	// notice / checks
	checking bool
	cliFound bool
	loadErr  string
	servers  []Server

	// menu
	countries []country
	cur, top  int
	menuMsg   string

	// run
	prev         []Result
	results      []Result
	cancel       context.CancelFunc
	events       chan domain.Event
	stopping     bool
	startAt      time.Time
	totalN       int
	doneN        int
	skipped      int
	runStable    int
	runFailed    int
	curName      string
	curPhase     string
	curStart     time.Time
	curConnT     float64
	curIP        string
	samples      []domain.SampleEvent
	totalSamples int
	logs         []logEntry

	// done
	rank       []Result
	rcur, rtop int
	status     string
}

// newModel builds the initial UI state for the given configuration
// and the results of a previous run (empty when not resuming).
func newModel(cfg config.Config, prev []Result) model {
	return model{
		cfg: cfg,
		w:   100, h: 32,
		stage:    stNotice,
		checking: true,
		prev:     prev,
		results:  append([]Result{}, prev...),
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// checkCmd looks for the CLI and loads the server list.
func (m model) checkCmd() tea.Cmd {
	return func() tea.Msg {
		if err := speedify.Available(m.cfg.CLIPath); err != nil {
			return checkMsg{CLIFound: false, Err: err.Error()}
		}
		sv, err := m.client().Servers(context.Background())
		if err != nil {
			return checkMsg{CLIFound: true, Err: "could not read the server list: " + err.Error()}
		}
		if len(sv) == 0 {
			return checkMsg{CLIFound: true, Err: "server list is empty"}
		}
		return checkMsg{CLIFound: true, Servers: sv}
	}
}

// waitEvent blocks until the next engine event arrives and wraps it
// as a Bubble Tea message.
func waitEvent(ch <-chan domain.Event) tea.Cmd {
	return func() tea.Msg { return eventMsg{<-ch} }
}

// connectCmd connects interactively (results screen, Enter).
func (m model) connectCmd(r Result) tea.Cmd {
	return func() tea.Msg {
		err := m.client().Connect(context.Background(), Server{Country: r.Country, City: r.City, Num: r.Num})
		return connectedMsg{Name: r.Server, Err: err}
	}
}

// disconnectCmd is a manual disconnect (results screen, d).
func (m model) disconnectCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.client().Disconnect(context.Background())
		return disconnectedMsg{Err: err}
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.checkCmd(), tickCmd())
}

// ---------- helpers on the model ----------

func (m model) totalServers() int {
	n := 0
	for _, c := range m.countries {
		n += len(c.Servers)
	}
	return n
}

func (m model) allSelected() bool {
	if len(m.countries) == 0 {
		return false
	}
	for _, c := range m.countries {
		if !c.Selected {
			return false
		}
	}
	return true
}

func (m model) selectedCounts() (countries, servers int) {
	for _, c := range m.countries {
		if c.Selected {
			countries++
			servers += len(c.Servers)
		}
	}
	return
}

// servers to test = selected countries minus (when -resume) already-tested ones
func (m model) buildTodo() (todo []Server, skipped int) {
	done := map[string]bool{}
	for _, r := range m.prev {
		done[r.Server] = true
	}
	for _, c := range m.countries {
		if !c.Selected {
			continue
		}
		for _, s := range c.Servers {
			if done[s.Name()] {
				skipped++
				continue
			}
			todo = append(todo, s)
		}
	}
	return
}

func (m model) menuRows() int {
	r := m.h - 9
	if r < 3 {
		r = 3
	}
	return r
}

func (m model) tableRows() int {
	r := m.h - 13
	if r < 3 {
		r = 3
	}
	return r
}

func (m *model) fixMenuView() {
	rows := m.menuRows()
	if m.cur < m.top {
		m.top = m.cur
	}
	if m.cur >= m.top+rows {
		m.top = m.cur - rows + 1
	}
	if m.top < 0 {
		m.top = 0
	}
}

func (m *model) fixTableView() {
	rows := m.tableRows()
	if m.rcur < m.rtop {
		m.rtop = m.rcur
	}
	if m.rcur >= m.rtop+rows {
		m.rtop = m.rcur - rows + 1
	}
	if m.rtop < 0 {
		m.rtop = 0
	}
}

func (m *model) moveMenu(d int) {
	m.cur += d
	if m.cur < 0 {
		m.cur = 0
	}
	if m.cur > len(m.countries) {
		m.cur = len(m.countries)
	}
	m.menuMsg = ""
	m.fixMenuView()
}

func (m *model) toggle() {
	m.menuMsg = ""
	if m.cur == 0 {
		v := !m.allSelected()
		for i := range m.countries {
			m.countries[i].Selected = v
		}
		return
	}
	i := m.cur - 1
	m.countries[i].Selected = !m.countries[i].Selected
}

func (m *model) addLog(icon, text string) {
	m.logs = append(m.logs, logEntry{T: time.Now(), Icon: icon, Text: text})
	if len(m.logs) > 500 {
		m.logs = m.logs[len(m.logs)-500:]
	}
}

func (m *model) startRun() tea.Cmd {
	todo, skipped := m.buildTodo()
	if len(todo) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan domain.Event, 256)

	m.cancel = cancel
	m.events = ch
	m.stopping = false
	m.stage = stRun
	m.startAt = time.Now()
	m.totalN = len(todo)
	m.doneN = 0
	m.skipped = skipped
	m.runStable, m.runFailed = 0, 0
	m.samples = nil
	m.curName, m.curPhase = "", ""
	m.logs = nil
	m.totalSamples = int(math.Ceil(float64(m.cfg.Hold) / float64(m.cfg.Interval)))
	m.results = append([]Result{}, m.prev...)
	m.addLog(accent("●"), fmt.Sprintf("Run started · %d servers", len(todo)))

	eng := checker.New(m.engineConfig(), m.client(),
		probe.NewTCP(m.cfg.ProbeAddr, m.cfg.ProbeTimeout), fileRecorder{dir: m.cfg.OutDir}, appLog)
	go eng.Run(ctx, todo, m.prev, func(ev domain.Event) { ch <- ev })
	return waitEvent(ch)
}

// engineConfig maps the configuration onto the checker engine knobs,
// keeping the hard-coded timings of the original program (500ms state
// polling, 2s retry delay).
func (m model) engineConfig() checker.Config {
	return checker.Config{
		ConnectTimeout: m.cfg.ConnectTimeout,
		Hold:           m.cfg.Hold,
		Interval:       m.cfg.Interval,
		MaxLoss:        m.cfg.MaxLoss,
		Retries:        m.cfg.Retries,
		Pause:          m.cfg.Pause,
		PollInterval:   checker.DefaultPollInterval,
		RetryDelay:     checker.DefaultRetryDelay,
		ProbeAddr:      m.cfg.ProbeAddr,
	}
}

// fileRecorder persists results through the report package: one jsonl
// line per finished server, full reports when the run ends.
type fileRecorder struct{ dir string }

func (rec fileRecorder) Record(r Result) {
	report.AppendJSONL(filepath.Join(rec.dir, "results.jsonl"), r)
}

func (rec fileRecorder) Finalize(all []Result) {
	report.WriteReports(rec.dir, all)
}

// ======================================================================
// update
// ======================================================================

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
func (m model) onEvent(ev domain.Event) (tea.Model, tea.Cmd) {
	switch ev := ev.(type) {
	case domain.LogEvent:
		m.addLog(warnS("↻"), warnS(ev.Text))
		return m, waitEvent(m.events)

	case domain.ServerStartEvent:
		m.curName = ev.Name
		m.curPhase = "connecting"
		m.curStart = time.Now()
		m.samples = nil
		m.curConnT, m.curIP = 0, ""
		m.addLog(dimS("▶"), dimS(fmt.Sprintf("%s  (%d/%d)", ev.Name, ev.Idx, ev.Total)))
		return m, waitEvent(m.events)

	case domain.PhaseEvent:
		m.curPhase = ev.Phase
		if ev.Phase == "connecting" {
			m.curStart = time.Now()
			m.samples = nil
		} else {
			m.curConnT = ev.ConnectTime
			m.curIP = ev.IP
		}
		return m, waitEvent(m.events)

	case domain.SampleEvent:
		m.samples = append(m.samples, ev)
		return m, waitEvent(m.events)

	case domain.ServerDoneEvent:
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

	case domain.FinishedEvent:
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

func (m model) onKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
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

// ======================================================================
// view
// ======================================================================

func (m model) View() string {
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

func (m model) viewNotice() string {
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

func checkbox(on bool) string {
	if on {
		return good("[✔]")
	}
	return dimS("[ ]")
}

func (m model) menuRow(i int) string {
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

func (m model) viewMenu() string {
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

func (m model) viewConfirm() string {
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

func (m model) viewRun() string {
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

func (m model) viewDone() string {
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

// ======================================================================
// main
// ======================================================================

func main() {
	cfg := config.Parse(os.Args[1:])
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	_ = os.MkdirAll(cfg.OutDir, 0o755)

	if lg, err := logx.Open(filepath.Join(cfg.OutDir, "speedify_check.log")); err == nil {
		appLog = lg
		defer lg.Close()
	}

	jsonl := filepath.Join(cfg.OutDir, "results.jsonl")
	var prev []Result
	if cfg.Resume {
		prev = report.LoadJSONL(jsonl)
	} else {
		_ = os.Remove(jsonl)
	}

	p := tea.NewProgram(newModel(cfg, prev), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
