// Speedify Server Checker - TUI edition
//
// Run:
//
//	go run .
//	go run . -hold 15s -retries 1 -max-loss 1
//
// Flow: notice -> country menu -> confirm popup -> live test screen -> ranked results
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"speedify-check/internal/config"
	"speedify-check/internal/logx"
	"speedify-check/internal/model"
	"speedify-check/internal/report"
	"speedify-check/internal/tui"
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
		defer l.Close()
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
