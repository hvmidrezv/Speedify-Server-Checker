// Command speedify-check tests Speedify VPN servers one by one in a
// terminal UI and ranks the stable ones. It only parses flags, sets up
// the log file and the results journal, and wires the packages
// together; all logic lives in the internal packages.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hvmidrezv/Speedify-Server-Checker/internal/config"
	"github.com/hvmidrezv/Speedify-Server-Checker/internal/logx"
	"github.com/hvmidrezv/Speedify-Server-Checker/internal/model"
	"github.com/hvmidrezv/Speedify-Server-Checker/internal/report"
	"github.com/hvmidrezv/Speedify-Server-Checker/internal/tui"
)

func main() {
	cfg := config.Parse(os.Args[1:])
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	_ = os.MkdirAll(cfg.OutDir, 0o755)

	var lg *logx.Logger
	if l, err := logx.Open(filepath.Join(cfg.OutDir, "speedify_check.log")); err == nil {
		lg = l
		defer func() { _ = l.Close() }()
	}

	jsonl := filepath.Join(cfg.OutDir, "results.jsonl")
	var prev []model.Result
	if cfg.Resume {
		prev = report.LoadJSONL(jsonl)
	} else {
		_ = os.Remove(jsonl)
	}

	p := tea.NewProgram(tui.New(cfg, prev, lg), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
