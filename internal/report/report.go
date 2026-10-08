// Package report writes test results to the on-disk artifacts the tool
// produces: the append-only results.jsonl journal plus the
// stable_servers.txt, stable_names.txt, results.csv and results.json
// summary files.
//
// All writers are best-effort: they never return errors that could
// abort a run, matching the original behavior.
package report

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"speedify-check/internal/model"
)

// AppendJSONL appends r as one JSON line to the journal at path,
// creating the file when needed.
func AppendJSONL(path string, r model.Result) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	b, _ := json.Marshal(r)
	_, _ = f.Write(append(b, '\n'))
}

// LoadJSONL reads results previously written by AppendJSONL. A missing
// file yields nil; malformed lines are skipped.
func LoadJSONL(path string) []model.Result {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	var out []model.Result
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var r model.Result
		if json.Unmarshal(sc.Bytes(), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

// RankStable returns the stable subset of all results sorted by score,
// best (lowest score) first.
func RankStable(all []model.Result) []model.Result {
	var st []model.Result
	for _, r := range all {
		if r.Stable {
			st = append(st, r)
		}
	}
	sort.Slice(st, func(i, j int) bool { return st[i].Score < st[j].Score })
	return st
}

// WriteReports writes the summary files for all results into dir:
// stable_servers.txt (ranked table), stable_names.txt (one name per
// line), results.json (everything) and results.csv (everything).
func WriteReports(dir string, all []model.Result) {
	stable := RankStable(all)

	var sb, names strings.Builder
	_, _ = fmt.Fprintf(&sb, "%-4s %-30s %8s %8s %8s %8s  %s\n", "#", "SERVER", "AVG(ms)", "JIT(ms)", "MAX(ms)", "CONN(s)", "EXIT IP")
	for i, r := range stable {
		_, _ = fmt.Fprintf(&sb, "%-4d %-30s %8.1f %8.1f %8.1f %8.1f  %s\n",
			i+1, r.Server, r.AvgMs, r.JitterMs, r.MaxMs, r.ConnectTime, r.ExitIP)
		_, _ = fmt.Fprintln(&names, r.Server)
	}
	_ = os.WriteFile(filepath.Join(dir, "stable_servers.txt"), []byte(sb.String()), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "stable_names.txt"), []byte(names.String()), 0o644)

	j, _ := json.MarshalIndent(all, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, "results.json"), j, 0o644)

	if f, err := os.Create(filepath.Join(dir, "results.csv")); err == nil {
		w := csv.NewWriter(f)
		_ = w.Write([]string{"server", "stable", "category", "reason", "attempts", "connect_s", "exit_ip",
			"samples", "probe_loss", "min_ms", "avg_ms", "max_ms", "jitter_ms", "score"})
		for _, r := range all {
			_ = w.Write([]string{r.Server, fmt.Sprint(r.Stable), r.Category, r.Reason, fmt.Sprint(r.Attempts),
				fmt.Sprintf("%.1f", r.ConnectTime), r.ExitIP, fmt.Sprint(r.Samples), fmt.Sprint(r.ProbeLoss),
				fmt.Sprintf("%.1f", r.MinMs), fmt.Sprintf("%.1f", r.AvgMs), fmt.Sprintf("%.1f", r.MaxMs),
				fmt.Sprintf("%.1f", r.JitterMs), fmt.Sprintf("%.1f", r.Score)})
		}
		w.Flush()
		_ = f.Close()
	}
}
