// Package model defines the shared domain types of speedify-check:
// servers, per-server test results, country groupings and the events
// emitted by the test engine.
//
// This package is a leaf: it must not import any other internal
// package.
package model

import (
	"fmt"
	"time"
)

// Server identifies a single Speedify server by country code, city and
// the server number within that city.
type Server struct {
	Country string `json:"country"`
	City    string `json:"city"`
	Num     int    `json:"num"`
}

// Name returns the canonical server name used in logs and reports,
// e.g. "de-frankfurt-3".
func (s Server) Name() string { return fmt.Sprintf("%s-%s-%d", s.Country, s.City, s.Num) }

// Result holds the outcome of testing one server: identity, how the
// attempt went (category/reason/attempts) and the probe statistics.
type Result struct {
	Server      string    `json:"server"`
	Country     string    `json:"country"`
	City        string    `json:"city"`
	Num         int       `json:"num"`
	Stable      bool      `json:"stable"`
	Category    string    `json:"category,omitempty"` // connect_timeout | dropped | loss | ok
	Reason      string    `json:"reason,omitempty"`
	Attempts    int       `json:"attempts"`
	ConnectTime float64   `json:"connect_time_s"`
	ExitIP      string    `json:"exit_ip,omitempty"`
	Samples     int       `json:"samples"`
	ProbeLoss   int       `json:"probe_loss"`
	MinMs       float64   `json:"min_ms"`
	AvgMs       float64   `json:"avg_ms"`
	MaxMs       float64   `json:"max_ms"`
	JitterMs    float64   `json:"jitter_ms"`
	Score       float64   `json:"score"` // lower is better
	TestedAt    time.Time `json:"tested_at"`
}

// Country groups servers that share a country code, together with the
// display name and flag emoji used by the selection menu.
type Country struct {
	Code    string
	Name    string
	Flag    string
	Servers []Server
}
