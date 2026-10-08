// Package config defines the runtime configuration of speedify-check:
// the command-line flags, their defaults and validation.
package config

import (
	"flag"
	"fmt"
	"os"
	"time"
)

// Config holds every command-line option. Field names, flags and
// defaults match the original single-file program.
type Config struct {
	CLIPath        string
	ConnectTimeout time.Duration
	Hold           time.Duration
	Interval       time.Duration
	ProbeAddr      string
	ProbeTimeout   time.Duration
	MaxLoss        int
	Retries        int
	Resume         bool
	OutDir         string
	Pause          time.Duration
}

// Defaults returns the configuration with the documented defaults.
func Defaults() Config {
	return Config{
		CLIPath:        "speedify_cli",
		ConnectTimeout: 25 * time.Second,
		Hold:           10 * time.Second,
		Interval:       time.Second,
		ProbeAddr:      "1.1.1.1:443",
		ProbeTimeout:   2 * time.Second,
		MaxLoss:        0,
		Retries:        0,
		Resume:         false,
		OutDir:         ".",
		Pause:          time.Second,
	}
}

// BindFlags registers the command-line flags on fs, using the receiver
// as destination and the original help strings.
func (c *Config) BindFlags(fs *flag.FlagSet) {
	fs.StringVar(&c.CLIPath, "cli", c.CLIPath, "path to speedify_cli")
	fs.DurationVar(&c.ConnectTimeout, "connect-timeout", c.ConnectTimeout, "max time to wait for CONNECTED")
	fs.DurationVar(&c.Hold, "hold", c.Hold, "how long the connection must stay stable")
	fs.DurationVar(&c.Interval, "interval", c.Interval, "sampling interval during hold")
	fs.StringVar(&c.ProbeAddr, "probe", c.ProbeAddr, "host:port for the TCP probe through the tunnel")
	fs.DurationVar(&c.ProbeTimeout, "probe-timeout", c.ProbeTimeout, "TCP probe timeout")
	fs.IntVar(&c.MaxLoss, "max-loss", c.MaxLoss, "max failed probes allowed for a server to count as stable")
	fs.IntVar(&c.Retries, "retries", c.Retries, "extra attempts for a failing server")
	fs.BoolVar(&c.Resume, "resume", c.Resume, "skip servers already tested in a previous run (results.jsonl)")
	fs.StringVar(&c.OutDir, "out", c.OutDir, "output directory")
	fs.DurationVar(&c.Pause, "pause", c.Pause, "pause between servers")
}

// Parse parses args (without the program name) into a Config. Unknown
// flags or bad values terminate the program, like the original
// flag.Parse did.
func Parse(args []string) Config {
	c := Defaults()
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	c.BindFlags(fs)
	// ExitOnError means Parse never returns an error here.
	_ = fs.Parse(args)
	return c
}

// Validate reports configurations that cannot work, such as
// non-positive durations (an interval of 0 would hang the sampler) or
// negative counts. Output file names and flag names are unchanged.
func (c Config) Validate() error {
	switch {
	case c.CLIPath == "":
		return fmt.Errorf("-cli must not be empty")
	case c.ConnectTimeout <= 0:
		return fmt.Errorf("-connect-timeout must be > 0, got %s", c.ConnectTimeout)
	case c.Hold <= 0:
		return fmt.Errorf("-hold must be > 0, got %s", c.Hold)
	case c.Interval <= 0:
		return fmt.Errorf("-interval must be > 0, got %s", c.Interval)
	case c.ProbeAddr == "":
		return fmt.Errorf("-probe must not be empty")
	case c.ProbeTimeout <= 0:
		return fmt.Errorf("-probe-timeout must be > 0, got %s", c.ProbeTimeout)
	case c.MaxLoss < 0:
		return fmt.Errorf("-max-loss must be >= 0, got %d", c.MaxLoss)
	case c.Retries < 0:
		return fmt.Errorf("-retries must be >= 0, got %d", c.Retries)
	case c.OutDir == "":
		return fmt.Errorf("-out must not be empty")
	case c.Pause < 0:
		return fmt.Errorf("-pause must be >= 0, got %s", c.Pause)
	}
	return nil
}
