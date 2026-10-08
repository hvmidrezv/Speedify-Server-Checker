package probe

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestStats(t *testing.T) {
	tests := []struct {
		name             string
		samples          []float64
		wantMin, wantAvg float64
		wantMax, wantJit float64
	}{
		{"empty", nil, 0, 0, 0, 0},
		{"single sample", []float64{5}, 5, 5, 5, 0},
		{"ascending", []float64{10, 20, 30}, 10, 20, 30, 10},
		{"descending", []float64{30, 10}, 10, 20, 30, 20},
		{"irregular", []float64{4, 1, 7}, 1, 4, 7, 4.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, avg, max, jit := Stats(tt.samples)
			if min != tt.wantMin || avg != tt.wantAvg || max != tt.wantMax || jit != tt.wantJit {
				t.Errorf("Stats(%v) = (%v, %v, %v, %v), want (%v, %v, %v, %v)",
					tt.samples, min, avg, max, jit,
					tt.wantMin, tt.wantAvg, tt.wantMax, tt.wantJit)
			}
		})
	}
}

func TestTCPProbeSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()

	p := NewTCP(ln.Addr().String(), time.Second)
	d, ok := p.Probe(context.Background())
	if !ok {
		t.Fatal("expected probe against local listener to succeed")
	}
	if d < 0 {
		t.Errorf("negative rtt: %v", d)
	}
}

func TestTCPProbeFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing listens here anymore

	p := NewTCP(addr, 200*time.Millisecond)
	if _, ok := p.Probe(context.Background()); ok {
		t.Fatal("expected probe against closed port to fail")
	}
}
