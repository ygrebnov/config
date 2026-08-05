package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ygrebnov/config/pkg/types"
)

type cfg struct {
	Name      string          `yaml:"name"`
	Port      int             `yaml:"port"`
	Debug     bool            `yaml:"debug"`
	Timeout   types.Duration  `yaml:"timeout"`
	RateLimit rateLimitCfg    `yaml:"rate_limit"`
	DB        *dbConfig       `yaml:"db"`
	Tag       *string         `yaml:"tag"`
	MaxJobs   *uint           `yaml:"max_jobs"`
	Delay     *types.Duration `yaml:"delay"`
}

type rateLimitCfg struct {
	MaxConn int            `yaml:"max_conn"`
	Window  types.Duration `yaml:"window"`
}

type dbConfig struct {
	Host string  `yaml:"host"`
	Port int     `yaml:"port"`
	User *string `yaml:"user"`
	SSL  *bool   `yaml:"ssl"`
}

func checkCfg(t *testing.T, actual, expected cfg) {
	if actual.Name != expected.Name {
		t.Fatalf("incorrect Name, got: %s, want: %s", actual.Name, expected.Name)
	}
	if actual.Port != expected.Port {
		t.Fatalf("incorrect Port, got: %d, want: %d", actual.Port, expected.Port)
	}
	if actual.Debug != expected.Debug {
		t.Fatalf("incorrect Debug, got: %t, want: %t", actual.Debug, expected.Debug)
	}
	if actual.Timeout != expected.Timeout {
		t.Fatalf("incorrect Timeout, got: %d, want: %d", actual.Timeout, expected.Timeout)
	}
	if actual.RateLimit.MaxConn != expected.RateLimit.MaxConn {
		t.Fatalf("incorrect RateLimit.MaxConn, got: %d, want: %d", actual.RateLimit.MaxConn, expected.RateLimit.MaxConn)
	}
	if actual.RateLimit.Window != expected.RateLimit.Window {
		t.Fatalf("incorrect RateLimit.Window, got: %d, want: %d", actual.RateLimit.Window, expected.RateLimit.Window)
	}
	if expected.DB == nil && actual.DB != nil {
		t.Fatalf("expected DB to be nil, got: %+v", actual.DB)
	}
	if expected.DB != nil && actual.DB == nil {
		t.Fatalf("expected DB to be non-nil, got nil")
	}
	if expected.DB != nil && actual.DB != nil {
		if actual.DB.Host != expected.DB.Host {
			t.Fatalf("incorrect DB.Host, got: %s, want: %s", actual.DB.Host, expected.DB.Host)
		}
		if actual.DB.Port != expected.DB.Port {
			t.Fatalf("incorrect DB.Port, got: %d, want: %d", actual.DB.Port, expected.DB.Port)
		}
		if (actual.DB.User == nil) != (expected.DB.User == nil) || (actual.DB.User != nil && *actual.DB.User != *expected.DB.User) {
			t.Fatalf("incorrect DB.User, got: %v, want: %v", actual.DB.User, expected.DB.User)
		}
		if (actual.DB.SSL == nil) != (expected.DB.SSL == nil) || (actual.DB.SSL != nil && *actual.DB.SSL != *expected.DB.SSL) {
			t.Fatalf("incorrect DB.SSL, got: %v, want: %v", actual.DB.SSL, expected.DB.SSL)
		}
	}
	if actual.Tag == nil && expected.Tag != nil {
		t.Fatalf("expected Tag to be non-nil, got nil")
	}
	if actual.Tag != nil && expected.Tag == nil {
		t.Fatalf("expected Tag to be nil, got: %v", *actual.Tag)
	}
	if actual.Tag != nil && expected.Tag != nil && *actual.Tag != *expected.Tag {
		t.Fatalf("incorrect Tag, got: %v, want: %v", *actual.Tag, *expected.Tag)
	}
	if actual.MaxJobs == nil && expected.MaxJobs != nil {
		t.Fatalf("expected MaxJobs to be non-nil, got nil")
	}
	if actual.MaxJobs != nil && expected.MaxJobs == nil {
		t.Fatalf("expected MaxJobs to be nil, got: %v", *actual.MaxJobs)
	}
	if actual.MaxJobs != nil && expected.MaxJobs != nil && *actual.MaxJobs != *expected.MaxJobs {
		t.Fatalf("incorrect MaxJobs, got: %v, want: %v", *actual.MaxJobs, *expected.MaxJobs)
	}
	if actual.Delay == nil && expected.Delay != nil {
		t.Fatalf("expected Delay to be non-nil, got nil")
	}
	if actual.Delay != nil && expected.Delay == nil {
		t.Fatalf("expected Delay to be nil, got: %v", *actual.Delay)
	}
	if actual.Delay != nil && expected.Delay != nil && *actual.Delay != *expected.Delay {
		t.Fatalf("incorrect Delay, got: %v, want: %v", *actual.Delay, *expected.Delay)
	}
}

func writeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	return string(data)
}

type smallCfg struct {
	Name  string         `json:"name" yaml:"name" env:"NAME"`
	Count int            `json:"count" yaml:"count" env:"COUNT"`
	Dur   types.Duration `json:"dur" yaml:"dur" env:"DUR"`
}

type tinyCfg struct {
	Name string `yaml:"name" env:"NAME" default:"svc" validate:"min(1)"`
	Port int    `yaml:"port" env:"PORT" default:"8080" validate:"min(1),nonzero"`
}
