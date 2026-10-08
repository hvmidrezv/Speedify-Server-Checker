package tui

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"speedify-check/internal/checker"
	"speedify-check/internal/config"
	"speedify-check/internal/geo"
	"speedify-check/internal/logx"
	"speedify-check/internal/model"
	"speedify-check/internal/probe"
	"speedify-check/internal/report"
	"speedify-check/internal/speedify"
)

// ======================================================================
// data
// ======================================================================

// Server and Result are aliases for the domain types shared with the
// other packages; they keep this package's code terse.
type (
	Server = model.Server
	Result = model.Result
)

// country is one row of the country selection menu: a model.Country
// grouping plus its selection state.
type country struct {
	model.Country
	Selected bool
}

func buildCountries(sv []Server) []country {
	m := map[string]*country{}
	for _, s := range sv {
		code := strings.ToLower(s.Country)
		c, ok := m[code]
		if !ok {
			c = &country{Country: model.Country{Code: code, Name: geo.CountryName(code), Flag: geo.FlagEmoji(code)}}
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
// speedify cli helpers
// ======================================================================

// client builds the speedify CLI client from the current config.
func (m Model) client() *speedify.Client {
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
type eventMsg struct{ ev model.Event }

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

// Model is the Bubble Tea UI state machine for the whole application.
type Model struct {
	cfg config.Config
	log *logx.Logger

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
	events       chan model.Event
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
	samples      []model.SampleEvent
	totalSamples int
	logs         []logEntry

	// done
	rank       []Result
	rcur, rtop int
	status     string
}

// New builds the initial UI state for the given configuration, the
// results of a previous run (nil when not resuming) and the optional
// diagnostics logger.
func New(cfg config.Config, prev []Result, log *logx.Logger) Model {
	return Model{
		cfg: cfg,
		log: log,
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
func (m Model) checkCmd() tea.Cmd {
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
func waitEvent(ch <-chan model.Event) tea.Cmd {
	return func() tea.Msg { return eventMsg{<-ch} }
}

// connectCmd connects interactively (results screen, Enter).
func (m Model) connectCmd(r Result) tea.Cmd {
	return func() tea.Msg {
		err := m.client().Connect(context.Background(), Server{Country: r.Country, City: r.City, Num: r.Num})
		return connectedMsg{Name: r.Server, Err: err}
	}
}

// disconnectCmd is a manual disconnect (results screen, d).
func (m Model) disconnectCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.client().Disconnect(context.Background())
		return disconnectedMsg{Err: err}
	}
}

// Init starts the CLI check and the spinner tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.checkCmd(), tickCmd())
}

// ---------- helpers on the model ----------

func (m Model) totalServers() int {
	n := 0
	for _, c := range m.countries {
		n += len(c.Servers)
	}
	return n
}

func (m Model) allSelected() bool {
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

func (m Model) selectedCounts() (countries, servers int) {
	for _, c := range m.countries {
		if c.Selected {
			countries++
			servers += len(c.Servers)
		}
	}
	return
}

// servers to test = selected countries minus (when -resume) already-tested ones
func (m Model) buildTodo() (todo []Server, skipped int) {
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

func (m Model) menuRows() int {
	r := m.h - 9
	if r < 3 {
		r = 3
	}
	return r
}

func (m Model) tableRows() int {
	r := m.h - 13
	if r < 3 {
		r = 3
	}
	return r
}

func (m *Model) fixMenuView() {
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

func (m *Model) fixTableView() {
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

func (m *Model) moveMenu(d int) {
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

func (m *Model) toggle() {
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

func (m *Model) addLog(icon, text string) {
	m.logs = append(m.logs, logEntry{T: time.Now(), Icon: icon, Text: text})
	if len(m.logs) > 500 {
		m.logs = m.logs[len(m.logs)-500:]
	}
}

func (m *Model) startRun() tea.Cmd {
	todo, skipped := m.buildTodo()
	if len(todo) == 0 {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan model.Event, 256)

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
		probe.NewTCP(m.cfg.ProbeAddr, m.cfg.ProbeTimeout), fileRecorder{dir: m.cfg.OutDir}, m.log)
	go eng.Run(ctx, todo, m.prev, func(ev model.Event) { ch <- ev })
	return waitEvent(ch)
}

// engineConfig maps the configuration onto the checker engine knobs,
// keeping the hard-coded timings of the original program (500ms state
// polling, 2s retry delay).
func (m Model) engineConfig() checker.Config {
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
