package speedify

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"speedify-check/internal/model"
)

func TestParseServers(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    []model.Server
		wantErr bool
	}{
		{
			name: "all three groups",
			out:  `{"public":[{"country":"de","city":"frankfurt","num":3}],"private":[{"country":"us","city":"dallas","num":1}],"team":[{"country":"uk","city":"london","num":7}]}`,
			want: []model.Server{
				{Country: "de", City: "frankfurt", Num: 3},
				{Country: "us", City: "dallas", Num: 1},
				{Country: "uk", City: "london", Num: 7},
			},
		},
		{
			name: "only public",
			out:  `{"public":[{"country":"de","city":"frankfurt","num":3}]}`,
			want: []model.Server{{Country: "de", City: "frankfurt", Num: 3}},
		},
		{
			name: "empty object yields empty list",
			out:  `{}`,
			want: []model.Server{},
		},
		{
			name:    "malformed json",
			out:     `{"public": [`,
			wantErr: true,
		},
		{
			name:    "non-array field",
			out:     `{"public": 5}`,
			wantErr: true,
		},
		{
			name:    "top-level array is not supported",
			out:     `[{"country":"de","city":"frankfurt","num":3}]`,
			wantErr: true,
		},
		{
			name:    "plain text",
			out:     `not json at all`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseServers(tt.out)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseServers(%q) error = nil, want error", tt.out)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseServers(%q) error = %v", tt.out, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseServers(%q) = %+v, want %+v", tt.out, got, tt.want)
			}
		})
	}
}

func TestParseState(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"json string", `"CONNECTED"`, "CONNECTED"},
		{"json string lower-case is upper-cased", `"connected"`, "CONNECTED"},
		{"object with state", `{"state":"CONNECTED"}`, "CONNECTED"},
		{"object with lower-case state", `{"state":"disconnected"}`, "DISCONNECTED"},
		{"plain text", `CONNECTED`, "CONNECTED"},
		{"plain text with newline", "CONNECTED\r\n", "CONNECTED"},
		{"plain text with spaces", `  connected  `, "CONNECTED"},
		{"quoted plain text", `"CONNECTED"`, "CONNECTED"},
		{"object without state key falls back to raw text", `{"foo":"bar"}`, `{"FOO":"BAR"}`},
		{"empty input", ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseState(tt.out); got != tt.want {
				t.Errorf("parseState(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

func TestParseCurrentServer(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want string
	}{
		{"object with ip", `{"ip":"1.2.3.4"}`, "1.2.3.4"},
		{"object without ip", `{"server":"de-frankfurt-3"}`, ""},
		{"malformed json", `{`, ""},
		{"plain text", `1.2.3.4`, ""},
		{"empty input", ``, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseCurrentServer(tt.out); got != tt.want {
				t.Errorf("parseCurrentServer(%q) = %q, want %q", tt.out, got, tt.want)
			}
		})
	}
}

// fakeRunner records calls and returns a canned response.
type fakeRunner struct {
	out   string
	err   error
	calls [][]string
	ctxs  []context.Context
}

func (f *fakeRunner) Run(ctx context.Context, args ...string) (string, error) {
	f.calls = append(f.calls, append([]string{}, args...))
	f.ctxs = append(f.ctxs, ctx)
	return f.out, f.err
}

func TestClientServers(t *testing.T) {
	fr := &fakeRunner{out: `{"public":[{"country":"de","city":"frankfurt","num":3}]}`}
	c := NewClient(fr)
	got, err := c.Servers(context.Background())
	if err != nil {
		t.Fatalf("Servers: %v", err)
	}
	want := []model.Server{{Country: "de", City: "frankfurt", Num: 3}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Servers = %+v, want %+v", got, want)
	}
	if len(fr.calls) != 1 || !reflect.DeepEqual(fr.calls[0], []string{"show", "servers"}) {
		t.Errorf("unexpected calls: %+v", fr.calls)
	}
}

func TestClientServersError(t *testing.T) {
	fr := &fakeRunner{err: errors.New("boom")}
	if _, err := NewClient(fr).Servers(context.Background()); err == nil {
		t.Fatal("expected error from failing runner")
	}
}

func TestClientConnectArgsAndDeadline(t *testing.T) {
	fr := &fakeRunner{}
	c := NewClient(fr)
	err := c.Connect(context.Background(), model.Server{Country: "de", City: "frankfurt", Num: 3})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !reflect.DeepEqual(fr.calls[0], []string{"connect", "de", "frankfurt", "3"}) {
		t.Errorf("unexpected args: %v", fr.calls[0])
	}
	deadline, ok := fr.ctxs[0].Deadline()
	if !ok {
		t.Fatal("Connect did not set a deadline on the context")
	}
	if d := time.Until(deadline); d <= 55*time.Second || d > connectTimeout {
		t.Errorf("unexpected deadline lead time: %v", d)
	}
}

func TestClientStateAndCurrentServer(t *testing.T) {
	fr := &fakeRunner{out: `"connected"`}
	c := NewClient(fr)
	if got := c.State(context.Background()); got != "CONNECTED" {
		t.Errorf("State = %q, want CONNECTED", got)
	}
	fr.out = `{"ip":"9.9.9.9"}`
	if got := c.CurrentServer(context.Background()); got != "9.9.9.9" {
		t.Errorf("CurrentServer = %q, want 9.9.9.9", got)
	}
	if !reflect.DeepEqual(fr.calls[0], []string{"state"}) ||
		!reflect.DeepEqual(fr.calls[1], []string{"show", "currentserver"}) {
		t.Errorf("unexpected calls: %+v", fr.calls)
	}
}

func TestClientStateFailureDegradesToEmpty(t *testing.T) {
	fr := &fakeRunner{out: "", err: errors.New("cli missing")}
	if got := NewClient(fr).State(context.Background()); got != "" {
		t.Errorf("State = %q, want empty on CLI failure", got)
	}
}

func TestClientDisconnect(t *testing.T) {
	fr := &fakeRunner{}
	if err := NewClient(fr).Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
	if !reflect.DeepEqual(fr.calls[0], []string{"disconnect"}) {
		t.Errorf("unexpected args: %v", fr.calls[0])
	}
}
