package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type testCfg struct {
	Answer int
}

type testCfg2 struct {
	Name  string        `json:"name" yaml:"name" env:"NAME"`
	Count int           `json:"count" yaml:"count" env:"COUNT"`
	Dur   time.Duration `json:"dur" yaml:"dur" env:"DUR"`
}

type mCfg struct {
	Name string `yaml:"name" env:"NAME" default:"svc" validate:"min(1)"`
	Port int    `yaml:"port" env:"PORT" default:"8080" validate:"min(1),nonzero"`
}

type fakeStreams struct {
	in     io.Reader
	out    io.Writer
	Err    io.Writer
}

func (s fakeStreams) In() io.Reader     { return s.in }
func (s fakeStreams) Out() io.Writer    { return s.out }
func (s fakeStreams) ErrOut() io.Writer { return s.Err }

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}


