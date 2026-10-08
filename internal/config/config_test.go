package config

import (
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	c := Defaults()
	if err := c.Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}
	if c.CLIPath != "speedify_cli" || c.Hold != 10*time.Second || c.Interval != time.Second ||
		c.ProbeAddr != "1.1.1.1:443" || c.MaxLoss != 0 || c.Retries != 0 || c.Resume ||
		c.OutDir != "." || c.Pause != time.Second || c.ConnectTimeout != 25*time.Second ||
		c.ProbeTimeout != 2*time.Second {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestParseOverrides(t *testing.T) {
	c := Parse([]string{
		"-cli", "/opt/speedify_cli",
		"-hold", "15s",
		"-retries", "1",
		"-max-loss", "2",
		"-resume",
		"-out", "/tmp/results",
	})
	if c.CLIPath != "/opt/speedify_cli" {
		t.Errorf("CLIPath = %q", c.CLIPath)
	}
	if c.Hold != 15*time.Second {
		t.Errorf("Hold = %v", c.Hold)
	}
	if c.Retries != 1 || c.MaxLoss != 2 {
		t.Errorf("Retries = %d, MaxLoss = %d", c.Retries, c.MaxLoss)
	}
	if !c.Resume {
		t.Error("Resume = false, want true")
	}
	if c.OutDir != "/tmp/results" {
		t.Errorf("OutDir = %q", c.OutDir)
	}
	// Untouched flags keep their defaults.
	if c.Interval != time.Second || c.Pause != time.Second {
		t.Errorf("defaults lost: %+v", c)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"defaults are valid", func(*Config) {}, false},
		{"zero hold", func(c *Config) { c.Hold = 0 }, true},
		{"negative hold", func(c *Config) { c.Hold = -time.Second }, true},
		{"zero interval", func(c *Config) { c.Interval = 0 }, true},
		{"zero connect timeout", func(c *Config) { c.ConnectTimeout = 0 }, true},
		{"zero probe timeout", func(c *Config) { c.ProbeTimeout = 0 }, true},
		{"negative retries", func(c *Config) { c.Retries = -1 }, true},
		{"negative max loss", func(c *Config) { c.MaxLoss = -1 }, true},
		{"negative pause", func(c *Config) { c.Pause = -time.Second }, true},
		{"empty out dir", func(c *Config) { c.OutDir = "" }, true},
		{"empty cli path", func(c *Config) { c.CLIPath = "" }, true},
		{"empty probe", func(c *Config) { c.ProbeAddr = "" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Defaults()
			tt.mutate(&c)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
