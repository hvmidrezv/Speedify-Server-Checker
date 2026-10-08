// Package probe provides the TCP connectivity probe used during the
// hold phase and pure statistics over the collected RTT samples.
//
// This package contains no scheduling or CLI knowledge; the test
// engine decides when to probe.
package probe

import (
	"context"
	"math"
	"net"
	"time"
)

// TCP probes one fixed host:port by opening a TCP connection and
// measuring how long the handshake takes.
type TCP struct {
	Addr    string
	Timeout time.Duration
}

// NewTCP returns a prober that dials addr with the given timeout.
func NewTCP(addr string, timeout time.Duration) *TCP {
	return &TCP{Addr: addr, Timeout: timeout}
}

// Probe dials the target once and reports the duration of the TCP
// handshake. ok is false when the dial failed, timed out or the
// context was cancelled.
func (p *TCP) Probe(ctx context.Context) (time.Duration, bool) {
	start := time.Now()
	d := net.Dialer{Timeout: p.Timeout}
	c, err := d.DialContext(ctx, "tcp", p.Addr)
	if err != nil {
		return 0, false
	}
	d2 := time.Since(start)
	_ = c.Close()
	return d2, true
}

// Stats summarizes RTT samples given in milliseconds. Jitter is the
// mean absolute difference between consecutive samples. For an empty
// input every value is zero; a single sample yields min == avg == max
// with zero jitter.
func Stats(samples []float64) (min, avg, max, jitter float64) {
	if len(samples) == 0 {
		return
	}
	min = math.MaxFloat64
	sum := 0.0
	for i, x := range samples {
		sum += x
		if x < min {
			min = x
		}
		if x > max {
			max = x
		}
		if i > 0 {
			jitter += math.Abs(x - samples[i-1])
		}
	}
	avg = sum / float64(len(samples))
	if len(samples) > 1 {
		jitter /= float64(len(samples) - 1)
	}
	return
}
