package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	modellib "github.com/ygrebnov/model"

	"github.com/ygrebnov/config/streams"
)

// A medium-size configuration object with nested structs and pointers
// intended to exercise env walking and zero/override logic.
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
		SSL  *bool   `yaml:"ssl"  env:"SSL"`
	} `yaml:"db" env:"DB"`
	// Pointers to basic types to exercise pointer handling in env overrides.
	Tag     *string        `yaml:"tag" env:"TAG"`
	MaxJobs *uint          `yaml:"max_jobs" env:"MAX_JOBS"`
	Delay   *time.Duration `yaml:"delay" env:"DELAY"`
}

// Model-backed configuration to exercise SetDefaults and Validate paths.
type modelCfg struct {
	Name string `yaml:"name" env:"NAME" default:"svc" validate:"nonempty"`
	Port int    `yaml:"port" env:"PORT" default:"8080" validate:"positive,nonzero"`
}

// sink prevents compiler from optimizing away results in benchmarks.
var sinkCfg *mediumCfg
var sinkModelCfg *modelCfg

func setupBenchmarkEnv(b *testing.B) {
	b.Helper()
	// Ensure we do not attempt to read from any file.
	b.Setenv("BM_CONFIG_PATH", "")
	// Set a handful of env vars so env logic has useful work to do.
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
	// Unset variables used by setupBenchmarkEnv to measure no-env path.
	keys := []string{
		"BM_NAME", "BM_PORT", "BM_DEBUG", "BM_TIMEOUT",
		"BM_RATELIMIT_MAX_CONN", "BM_RATELIMIT_WINDOW",
		"BM_DB_HOST", "BM_DB_PORT", "BM_DB_SSL",
		"BM_TAG", "BM_MAX_JOBS", "BM_DELAY",
	}
	for _, k := range keys {
		b.Setenv(k, "")
	}
	b.Setenv("BM_CONFIG_PATH", "")
}

// Prepares a persistent config path under a temp XDG_CONFIG_HOME and writes a file.
// Returns the dirName used. Benchmarks should pass WithPersistence(dirName).
func setupPersistentFile(b *testing.B, dirName string, yamlContent string) string {
	b.Helper()
	td := b.TempDir()
	b.Setenv("XDG_CONFIG_HOME", td)
	path := filepath.Join(td, dirName, "config.yml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		b.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(yamlContent), 0o600); err != nil {
		b.Fatalf("write: %v", err)
	}
	// Ensure there is no direct override path via env
	b.Setenv("BM_CONFIG_PATH", "")
	return dirName
}

// ---------------- Existing benchmarks ----------------

// Benchmark legacy Get path (env overrides present).
func BenchmarkProvider_Get_Legacy_Medium(b *testing.B) {
	setupBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[mediumCfg](
			WithEnvPrefix[mediumCfg]("BM"),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (legacy) error: %v", err)
		}
		sinkCfg = cfg
	}
}

// Benchmark pipeline mode using the Provider's internally constructed default stages (env present).
func BenchmarkProvider_Get_Pipeline_Medium(b *testing.B) {
	setupBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[mediumCfg](
			WithEnvPrefix[mediumCfg]("BM"),
			WithPipelineMode[mediumCfg](),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (pipeline) error: %v", err)
		}
		sinkCfg = cfg
	}
}

// ---------------- Additional variants ----------------

// No-env variants (same struct, but no env variables set)
func BenchmarkProvider_Get_Legacy_Medium_NoEnv(b *testing.B) {
	clearBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[mediumCfg](
			WithEnvPrefix[mediumCfg]("BM"),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (legacy no-env) error: %v", err)
		}
		sinkCfg = cfg
	}
}

func BenchmarkProvider_Get_Pipeline_Medium_NoEnv(b *testing.B) {
	clearBenchmarkEnv(b)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[mediumCfg](
			WithEnvPrefix[mediumCfg]("BM"),
			WithPipelineMode[mediumCfg](),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (pipeline no-env) error: %v", err)
		}
		sinkCfg = cfg
	}
}

// Persistent mode with an existing file (avoid creating per-iteration).
func BenchmarkProvider_Get_Legacy_Persistent_FileExists(b *testing.B) {
	// Seed file with some content; env overrides not set here.
	dirName := setupPersistentFile(b, "bmapp", "name: fromfile\nport: 8082\n")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[mediumCfg](
			WithEnvPrefix[mediumCfg]("BM"),
			WithPersistence[mediumCfg](dirName),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (legacy persistent file) error: %v", err)
		}
		sinkCfg = cfg
	}
}

func BenchmarkProvider_Get_Pipeline_Persistent_FileExists(b *testing.B) {
	dirName := setupPersistentFile(b, "bmapp", "name: fromfile\nport: 8082\n")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[mediumCfg](
			WithEnvPrefix[mediumCfg]("BM"),
			WithPersistence[mediumCfg](dirName),
			WithPipelineMode[mediumCfg](),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (pipeline persistent file) error: %v", err)
		}
		sinkCfg = cfg
	}
}

// Model-backed defaults + validation (no env)
func BenchmarkProvider_Get_Legacy_Model_NoEnv(b *testing.B) {
	// Separate prefix to avoid bleeding any BM_* envs
	b.Setenv("BMM_CONFIG_PATH", "")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[modelCfg](
			WithEnvPrefix[modelCfg]("BMM"),
			WithModel[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (legacy model no-env) error: %v", err)
		}
		sinkModelCfg = cfg
	}
}

func BenchmarkProvider_Get_Pipeline_Model_NoEnv(b *testing.B) {
	b.Setenv("BMM_CONFIG_PATH", "")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[modelCfg](
			WithEnvPrefix[modelCfg]("BMM"),
			WithModel[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }),
			WithPipelineMode[modelCfg](),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (pipeline model no-env) error: %v", err)
		}
		sinkModelCfg = cfg
	}
}

// Model-backed with env overrides set
func BenchmarkProvider_Get_Legacy_Model_WithEnv(b *testing.B) {
	b.Setenv("BMM_CONFIG_PATH", "")
	b.Setenv("BMM_NAME", "fromenv")
	b.Setenv("BMM_PORT", "9090")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[modelCfg](
			WithEnvPrefix[modelCfg]("BMM"),
			WithModel[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (legacy model with env) error: %v", err)
		}
		sinkModelCfg = cfg
	}
}

func BenchmarkProvider_Get_Pipeline_Model_WithEnv(b *testing.B) {
	b.Setenv("BMM_CONFIG_PATH", "")
	b.Setenv("BMM_NAME", "fromenv")
	b.Setenv("BMM_PORT", "9090")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p := New[modelCfg](
			WithEnvPrefix[modelCfg]("BMM"),
			WithModel[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }),
			WithPipelineMode[modelCfg](),
		)
		cfg, _, _, err := p.Get()
		if err != nil {
			b.Fatalf("Get (pipeline model with env) error: %v", err)
		}
		sinkModelCfg = cfg
	}
}

// ---------------- Builder variants ----------------

func BenchmarkBuilder_Medium_WithEnv(b *testing.B) {
	setupBenchmarkEnv(b)
	b.ReportAllocs()
	builder := NewBuilder[mediumCfg](
		WithBuilderEnv[mediumCfg]("BM", SetOverride),
	)
	for i := 0; i < b.N; i++ {
		var cfg mediumCfg
		if err := builder.Build(context.Background(), &cfg); err != nil {
			b.Fatalf("Builder env error: %v", err)
		}
		sinkCfg = &cfg
	}
}

func BenchmarkBuilder_Medium_NoEnv(b *testing.B) {
	clearBenchmarkEnv(b)
	b.ReportAllocs()
	builder := NewBuilder[mediumCfg](
		WithBuilderEnv[mediumCfg]("BM", SetOverride),
	)
	for i := 0; i < b.N; i++ {
		var cfg mediumCfg
		if err := builder.Build(context.Background(), &cfg); err != nil {
			b.Fatalf("Builder no-env error: %v", err)
		}
		sinkCfg = &cfg
	}
}

func BenchmarkBuilder_Persistent_FileExists(b *testing.B) {
	dirName := setupPersistentFile(b, "bmapp", "name: fromfile\nport: 8082\n")
	b.ReportAllocs()
	builder := NewBuilder[mediumCfg](
		WithBuilderFileOps[mediumCfg](
			func() string { return filepath.Join(os.Getenv("XDG_CONFIG_HOME"), dirName, "config.yml") },
			func() bool { return true },
			streams.Discard(),
			nil,
			nil,
		),
	)
	for i := 0; i < b.N; i++ {
		var cfg mediumCfg
		if err := builder.Build(context.Background(), &cfg); err != nil {
			b.Fatalf("Builder persistent file error: %v", err)
		}
		sinkCfg = &cfg
	}
}

func BenchmarkBuilder_Model_NoEnv(b *testing.B) {
	b.Setenv("BMM_CONFIG_PATH", "")
	b.ReportAllocs()
	builder := NewBuilder[modelCfg](
		WithBuilderModelDefaults[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }),
		WithBuilderModelValidateInit[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }, func(err error, _ ValidationStrategy) error { return err }, ValidateAllErrors),
	)
	for i := 0; i < b.N; i++ {
		var cfg modelCfg
		if err := builder.Build(context.Background(), &cfg); err != nil {
			b.Fatalf("Builder model no-env error: %v", err)
		}
		sinkModelCfg = &cfg
	}
}

func BenchmarkBuilder_Model_WithEnv(b *testing.B) {
	b.Setenv("BMM_CONFIG_PATH", "")
	b.Setenv("BMM_NAME", "fromenv")
	b.Setenv("BMM_PORT", "9090")
	b.ReportAllocs()
	builder := NewBuilder[modelCfg](
		WithBuilderModelDefaults[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }),
		WithBuilderEnv[modelCfg]("BMM", SetOverride),
		WithBuilderModelValidateInit[modelCfg](func(c *modelCfg) (*modellib.Model[modelCfg], error) { return modellib.New(c) }, func(err error, _ ValidationStrategy) error { return err }, ValidateAllErrors),
	)
	for i := 0; i < b.N; i++ {
		var cfg modelCfg
		if err := builder.Build(context.Background(), &cfg); err != nil {
			b.Fatalf("Builder model with env error: %v", err)
		}
		sinkModelCfg = &cfg
	}
}
