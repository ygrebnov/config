package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ygrebnov/config/pkg/types"
)

type tinyCfg struct {
	Name string `yaml:"name" env:"NAME" default:"svc" validate:"min(1)"`
	Port int    `yaml:"port" env:"PORT" default:"8080" validate:"min(1),nonzero"`
}

type smallCfg struct {
	Name  string         `json:"name" yaml:"name" env:"NAME"`
	Count int            `json:"count" yaml:"count" env:"COUNT"`
	Dur   types.Duration `json:"dur" yaml:"dur" env:"DUR"`
}

type mediumCfg struct {
	Name      string        `yaml:"name" env:"NAME"`
	Port      int           `yaml:"port" env:"PORT"`
	Debug     bool          `yaml:"debug" env:"DEBUG"`
	Timeout   time.Duration `yaml:"timeout" env:"TIMEOUT"`
	RateLimit struct {
		MaxConn int           `yaml:"max_conn" env:"MAX_CONN"`
		Window  time.Duration `yaml:"window" env:"WINDOW"`
	} `yaml:"ratelimit" env:"RATELIMIT"`
	DB *struct {
		Host string  `yaml:"host" env:"HOST"`
		Port int     `yaml:"port" env:"PORT"`
		User *string `yaml:"user" env:"USER"`
		SSL  *bool   `yaml:"ssl" env:"SSL"`
	} `yaml:"db" env:"DB"`
	Tag     *string        `yaml:"tag" env:"TAG"`
	MaxJobs *uint          `yaml:"max_jobs" env:"MAX_JOBS"`
	Delay   *time.Duration `yaml:"delay" env:"DELAY"`
}

var sinkMediumCfg *mediumCfg
var sinkAny any

func setupBenchmarkEnv(b *testing.B) {
	b.Helper()
	b.Setenv("BM_NAME", "service")
	b.Setenv("BM_PORT", "8081")
	b.Setenv("BM_DEBUG", "true")
	b.Setenv("BM_TIMEOUT", "250ms")
	b.Setenv("BM_RATELIMIT_MAX_CONN", "512")
	b.Setenv("BM_RATELIMIT_WINDOW", "2s")
	b.Setenv("BM_DB_HOST", "127.0.0.1")
	b.Setenv("BM_DB_PORT", "5432")
	b.Setenv("BM_DB_SSL", "true")
	b.Setenv("BM_TAG", "v1")
	b.Setenv("BM_MAX_JOBS", "64")
	b.Setenv("BM_DELAY", "75ms")
}

func clearBenchmarkEnv(b *testing.B) {
	b.Helper()
	for _, key := range []string{
		"BM_NAME", "BM_PORT", "BM_DEBUG", "BM_TIMEOUT",
		"BM_RATELIMIT_MAX_CONN", "BM_RATELIMIT_WINDOW",
		"BM_DB_HOST", "BM_DB_PORT", "BM_DB_SSL",
		"BM_TAG", "BM_MAX_JOBS", "BM_DELAY", "BM_CONFIG_PATH",
	} {
		b.Setenv(key, "")
	}
}

func BenchmarkLoad_Medium_WithEnv(b *testing.B) {
	setupBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg mediumCfg
		if err := Load(context.Background(), &cfg, WithEnvPrefix("BM")); err != nil {
			b.Fatalf("Load error: %v", err)
		}
		sinkMediumCfg = &cfg
	}
}

func BenchmarkLoad_Medium_NoEnv(b *testing.B) {
	clearBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg mediumCfg
		if err := Load(context.Background(), &cfg, WithEnvPrefix("BM")); err != nil {
			b.Fatalf("Load error: %v", err)
		}
		sinkMediumCfg = &cfg
	}
}

func BenchmarkLoad_Model_WithEnv(b *testing.B) {
	b.Setenv("BMM_NAME", "fromenv")
	b.Setenv("BMM_PORT", "9090")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg tinyCfg
		if err := Load(context.Background(), &cfg,
			WithEnvPrefix("BMM"),
		); err != nil {
			b.Fatalf("Load error: %v", err)
		}
		sinkAny = cfg
	}
}

func BenchmarkLoad_Full(b *testing.B) {
	const prefix = "BM"
	path := filepath.Join(b.TempDir(), "config.yaml")
	content := "" +
		"name: fromfile\n" +
		"port: 8082\n" +
		"debug: false\n" +
		"timeout: 1s\n" +
		"ratelimit:\n" +
		"  max_conn: 100\n" +
		"  window: 4s\n" +
		"db:\n" +
		"  host: 127.0.0.1\n" +
		"  port: 5432\n" +
		"  user: fileuser\n" +
		"  ssl: false\n" +
		"tag: filetag\n" +
		"max_jobs: 8\n" +
		"delay: 500ms\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		b.Fatalf("write file: %v", err)
	}
	setupBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var cfg mediumCfg
		if err := Load(context.Background(), &cfg,
			WithPath(path),
			WithEnvPrefix(prefix),
		); err != nil {
			b.Fatalf("Load full error: %v", err)
		}
		sinkMediumCfg = &cfg
	}
}

func BenchmarkController_GetSetSave(b *testing.B) {
	path := filepath.Join(b.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("name: fromfile\ncount: 2\n"), 0o600); err != nil {
		b.Fatalf("write file: %v", err)
	}
	controller, err := NewController[smallCfg](WithPath(path))
	if err != nil {
		b.Fatalf("NewController: %v", err)
	}
	cfg := smallCfg{}
	if err := controller.Load(context.Background(), &cfg); err != nil {
		b.Fatalf("controller load: %v", err)
	}

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			value, ok := controller.Get("name")
			if !ok {
				b.Fatalf("option not found")
			}
			sinkAny = value
		}
	})

	b.Run("Set", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			controller.Set("count", i)
		}
	})

	b.Run("Save", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			controller.Set("count", i)
			if err := controller.Save(context.Background()); err != nil {
				b.Fatalf("Save error: %v", err)
			}
		}
	})
}
