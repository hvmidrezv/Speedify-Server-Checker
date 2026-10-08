package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/hvmidrezv/Speedify-Server-Checker/internal/config"
)

// step feeds one message to the model and returns the updated model.
func step(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	nm, _ := m.Update(msg)
	return nm.(Model)
}

func key(runes ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: runes}
}

func keyOf(t tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: t}
}

// loadedModel returns a model that passed the CLI check with two
// countries selected-state untouched.
func loadedModel(t *testing.T) Model {
	t.Helper()
	m := New(config.Defaults(), nil, nil)
	m = step(t, m, checkMsg{CLIFound: true, Servers: []Server{
		{Country: "de", City: "frankfurt", Num: 1},
		{Country: "de", City: "frankfurt", Num: 2},
		{Country: "us", City: "dallas", Num: 1},
	}})
	if m.stage != stNotice {
		t.Fatalf("after checkMsg stage = %v, want stNotice", m.stage)
	}
	return m
}

func TestEnterMovesNoticeToMenu(t *testing.T) {
	m := loadedModel(t)
	m = step(t, m, keyOf(tea.KeyEnter))
	if m.stage != stMenu {
		t.Errorf("stage = %v, want stMenu", m.stage)
	}
	if len(m.countries) != 2 {
		t.Errorf("countries = %d, want 2", len(m.countries))
	}
}

func TestMissingCLIFailsNotice(t *testing.T) {
	m := New(config.Defaults(), nil, nil)
	m = step(t, m, checkMsg{CLIFound: false, Err: "exec: \"speedify_cli\": executable file not found"})
	if m.stage != stNotice {
		t.Errorf("stage = %v, want stNotice on failed check", m.stage)
	}
	if !strings.Contains(m.loadErr, "speedify_cli") {
		t.Errorf("loadErr = %q, want the CLI error", m.loadErr)
	}
}

func TestSpaceTogglesCountry(t *testing.T) {
	m := loadedModel(t)
	m = step(t, m, keyOf(tea.KeyEnter))
	if countries, servers := m.selectedCounts(); countries != 0 || servers != 0 {
		t.Fatalf("selection before toggle = %d/%d, want 0/0", countries, servers)
	}
	// Row 0 is the pseudo-row "All countries"; move to the first real row.
	m = step(t, m, key('j'))
	m = step(t, m, keyOf(tea.KeySpace))
	countries, servers := m.selectedCounts()
	if countries != 1 || servers != 2 {
		t.Errorf("selection after toggle = %d/%d, want 1/2", countries, servers)
	}
	// Toggle the same country off again.
	m = step(t, m, keyOf(tea.KeySpace))
	if countries, servers := m.selectedCounts(); countries != 0 || servers != 0 {
		t.Errorf("selection after second toggle = %d/%d, want 0/0", countries, servers)
	}
}

func TestSelectAllThenConfirmRoundTrip(t *testing.T) {
	m := loadedModel(t)
	m = step(t, m, keyOf(tea.KeyEnter))
	m = step(t, m, key('a'))
	if !m.allSelected() {
		t.Fatal("a did not select all countries")
	}
	if _, servers := m.selectedCounts(); servers != 3 {
		t.Errorf("selected servers = %d, want 3", servers)
	}
	m = step(t, m, keyOf(tea.KeyEnter))
	if m.stage != stConfirm {
		t.Fatalf("stage = %v, want stConfirm", m.stage)
	}
	// The confirm popup goes back with n.
	m = step(t, m, key('n'))
	if m.stage != stMenu {
		t.Errorf("stage after n = %v, want stMenu", m.stage)
	}
}

func TestEnterWithoutSelectionWarns(t *testing.T) {
	m := loadedModel(t)
	m = step(t, m, keyOf(tea.KeyEnter))
	if m.stage != stMenu {
		t.Fatalf("stage = %v, want stMenu", m.stage)
	}
	m = step(t, m, keyOf(tea.KeyEnter)) // nothing selected
	if m.stage != stMenu {
		t.Errorf("stage = %v, want to stay in stMenu", m.stage)
	}
	if m.menuMsg == "" {
		t.Error("menuMsg is empty, want the nothing-selected warning")
	}
	if !strings.Contains(m.menuMsg, "Select at least one") {
		t.Errorf("menuMsg = %q, want the nothing-selected warning", m.menuMsg)
	}
}

func TestCtrlCQuits(t *testing.T) {
	m := loadedModel(t)
	_, cmd := m.Update(keyOf(tea.KeyCtrlC))
	if cmd == nil {
		t.Fatal("ctrl+c returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("ctrl+c command produced %T, want tea.QuitMsg", cmd())
	}
}

func TestEscapeLeavesMenu(t *testing.T) {
	m := loadedModel(t)
	m = step(t, m, keyOf(tea.KeyEnter))
	m = step(t, m, keyOf(tea.KeyEsc))
	if m.stage != stNotice {
		t.Errorf("stage after esc = %v, want stNotice", m.stage)
	}
}
