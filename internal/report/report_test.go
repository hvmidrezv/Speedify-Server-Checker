package report

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"speedify-check/internal/model"
)

func TestJSONLRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.jsonl")
	in := []model.Result{
		{Server: "de-frankfurt-3", Stable: true, Score: 21.5, Attempts: 1},
		{Server: "us-dallas-1", Stable: false, Category: "loss", Score: 0, Attempts: 2},
	}
	for _, r := range in {
		AppendJSONL(path, r)
	}
	got := LoadJSONL(path)
	if !reflect.DeepEqual(got, in) {
		t.Errorf("round trip = %+v, want %+v", got, in)
	}

	// Appending after a reload must extend, not replace.
	AppendJSONL(path, model.Result{Server: "uk-london-7", Stable: true, Score: 30})
	if got := LoadJSONL(path); len(got) != 3 {
		t.Errorf("after third append got %d results, want 3", len(got))
	}
}

func TestLoadJSONLMissingFile(t *testing.T) {
	if got := LoadJSONL(filepath.Join(t.TempDir(), "nope.jsonl")); got != nil {
		t.Errorf("LoadJSONL(missing) = %+v, want nil", got)
	}
}

func TestLoadJSONLSkipsMalformedLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results.jsonl")
	good1, _ := json.Marshal(model.Result{Server: "de-a-1", Stable: true})
	good2, _ := json.Marshal(model.Result{Server: "us-b-2"})
	_ = os.WriteFile(path, []byte(string(good1)+"\nnot json\n"+string(good2)+"\n"), 0o644)
	got := LoadJSONL(path)
	if len(got) != 2 || got[0].Server != "de-a-1" || got[1].Server != "us-b-2" {
		t.Errorf("LoadJSONL = %+v, want the two valid results", got)
	}
}

func TestRankStableOrdersByScoreAscending(t *testing.T) {
	all := []model.Result{
		{Server: "slow", Stable: true, Score: 30},
		{Server: "broken", Stable: false, Category: "loss", Score: 1},
		{Server: "best", Stable: true, Score: 10},
		{Server: "mid", Stable: true, Score: 20},
	}
	got := RankStable(all)
	var names []string
	for _, r := range got {
		names = append(names, r.Server)
	}
	want := []string{"best", "mid", "slow"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("RankStable order = %v, want %v", names, want)
	}
}

func TestRankStableEmpty(t *testing.T) {
	if got := RankStable(nil); got != nil {
		t.Errorf("RankStable(nil) = %+v, want nil", got)
	}
}

func TestWriteReports(t *testing.T) {
	dir := t.TempDir()
	all := []model.Result{
		{Server: "de-a-1", Stable: true, Score: 20, AvgMs: 20, ExitIP: "1.1.1.1"},
		{Server: "us-b-2", Stable: false, Category: "loss", Reason: "traffic loss"},
		{Server: "uk-c-3", Stable: true, Score: 10, AvgMs: 10, ExitIP: "2.2.2.2"},
	}
	WriteReports(dir, all)

	// stable_servers.txt: header, then stable rows best-first.
	txt, err := os.ReadFile(filepath.Join(dir, "stable_servers.txt"))
	if err != nil {
		t.Fatalf("read stable_servers.txt: %v", err)
	}
	txtLines := strings.Split(strings.TrimRight(string(txt), "\n"), "\n")
	if len(txtLines) != 3 {
		t.Fatalf("stable_servers.txt has %d lines, want 3", len(txtLines))
	}
	if !strings.Contains(txtLines[0], "SERVER") || !strings.Contains(txtLines[1], "uk-c-3") || !strings.Contains(txtLines[2], "de-a-1") {
		t.Errorf("stable_servers.txt unexpected content:\n%s", txt)
	}

	// stable_names.txt: one stable name per line, ranked.
	names, err := os.ReadFile(filepath.Join(dir, "stable_names.txt"))
	if err != nil {
		t.Fatalf("read stable_names.txt: %v", err)
	}
	if got, want := string(names), "uk-c-3\nde-a-1\n"; got != want {
		t.Errorf("stable_names.txt = %q, want %q", got, want)
	}

	// results.json: all results, valid JSON.
	jb, err := os.ReadFile(filepath.Join(dir, "results.json"))
	if err != nil {
		t.Fatalf("read results.json: %v", err)
	}
	var back []model.Result
	if err := json.Unmarshal(jb, &back); err != nil {
		t.Fatalf("results.json invalid: %v", err)
	}
	if len(back) != 3 {
		t.Errorf("results.json has %d results, want 3", len(back))
	}

	// results.csv: exact header + one row per result.
	f, err := os.Open(filepath.Join(dir, "results.csv"))
	if err != nil {
		t.Fatalf("open results.csv: %v", err)
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		t.Fatalf("read results.csv: %v", err)
	}
	wantHeader := []string{"server", "stable", "category", "reason", "attempts", "connect_s", "exit_ip",
		"samples", "probe_loss", "min_ms", "avg_ms", "max_ms", "jitter_ms", "score"}
	if !reflect.DeepEqual(records[0], wantHeader) {
		t.Errorf("csv header = %v, want %v", records[0], wantHeader)
	}
	if len(records) != 4 {
		t.Errorf("csv has %d records, want 4 (header + 3 rows)", len(records))
	}
}
