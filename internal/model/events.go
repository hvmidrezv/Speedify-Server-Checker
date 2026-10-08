package model

// Event is a progress notification emitted by the checker engine while
// it tests servers. The interface is sealed: only the types defined in
// this package implement it. The TUI adapts events to tea.Msg values.
type Event interface{ isEvent() }

// LogEvent carries a human-readable notice for the live log (Kind is
// free-form, e.g. "warn").
type LogEvent struct {
	Kind string
	Text string
}

func (LogEvent) isEvent() {}

// ServerStartEvent announces that testing of one server begins.
type ServerStartEvent struct {
	Idx   int // 1-based position in the run
	Total int // total servers in the run
	Name  string
}

func (ServerStartEvent) isEvent() {}

// PhaseEvent reports a phase change for the current server: Phase is
// "connecting" or "holding"; ConnectTime and IP are set for holding.
type PhaseEvent struct {
	Phase       string
	ConnectTime float64
	IP          string
}

func (PhaseEvent) isEvent() {}

// SampleEvent reports one sampling tick during the hold phase.
type SampleEvent struct {
	N       int // 1-based sample number
	StateOK bool
	ProbeOK bool
	State   string
	RTT     float64 // milliseconds, valid when ProbeOK
}

func (SampleEvent) isEvent() {}

// ServerDoneEvent reports the final result for one server.
type ServerDoneEvent struct {
	Result Result
}

func (ServerDoneEvent) isEvent() {}

// FinishedEvent is emitted once when the run ends (also on
// cancellation, with whatever results exist by then).
type FinishedEvent struct {
	All []Result
}

func (FinishedEvent) isEvent() {}
