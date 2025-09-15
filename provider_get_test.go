package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	modellib "github.com/ygrebnov/model"
)

// ---- Test scaffolding ----

type testCfg2 struct {
	Name  string        `json:"name" yaml:"name" env:"NAME"`
	Count int           `json:"count" yaml:"count" env:"COUNT"`
	Dur   time.Duration `json:"dur"  yaml:"dur"  env:"DUR"`
}

// mCfg exercises defaults+validation through github.com/ygrebnov/model
// and is used in the model-integrated test cases below.
type mCfg struct {
	Name string `yaml:"name" env:"NAME" default:"svc" validate:"nonempty"`
	Port int    `yaml:"port" env:"PORT" default:"8080" validate:"positive,nonzero"`
}

// helpers
func defFn() *testCfg2 { return &testCfg2{Name: "default", Count: 1} }

func writeFile(t *testing.T, p, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// ---- Tests ----

func TestProvider_Get_TableDriven(t *testing.T) {
	td := t.TempDir()
	// Make UserConfigDir predictable where needed
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	// (Clear HOME/USERPROFILE so XDG path is used on all platforms)
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	var outBuf, errBuf bytes.Buffer
	streams := fakeStreams{out: &outBuf, errOut: &errBuf}

	type want struct {
		errContains   string
		fileCreated   bool
		pathHasSuffix string
		outContains   string // message printed to Out
		errContainsLn string // message printed to ErrOut
		name          string // resulting cfg.Name
		count         int    // resulting cfg.Count
	}

	tests := []struct {
		name  string
		setup func(t *testing.T) (opts []Option[testCfg2])
		want  want
	}{
		{
			name: "with model: factory non-zero default wins over model default",
			setup: func(t *testing.T) []Option[testCfg2] {
				// no file, no path override; isolate env
				t.Setenv("MYAPP_CONFIG_PATH", "")
				return nil
			},
			want: want{},
		},
		{
			name: "with model: model fills zero value when factory leaves zero",
			setup: func(t *testing.T) []Option[testCfg2] {
				// no file, no path override; isolate env
				t.Setenv("MYAPP_CONFIG_PATH", "")
				return nil
			},
			want: want{},
		},
		{
			name: "env override path missing + persistent => create file, print created",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				envPath := filepath.Join(td, "env-override", "config.yaml")
				t.Setenv("MYAPP_CONFIG_PATH", envPath)
				return []Option[testCfg2]{
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithPersistence[testCfg2]("whatever"), // persist TRUE (dirName ignored when env override)
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				fileCreated:   true,
				pathHasSuffix: filepath.Join("env-override", "config.yaml"),
				outContains:   "created new config",
				name:          "default",
				count:         1,
			},
		},
		{
			name: "env override path present (yaml) + persistent => load file, print loaded",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				envPath := filepath.Join(td, "present", "config.yaml")
				writeFile(t, envPath, "name: fromfile\ncount: 7\ndur: 2s\n")
				t.Setenv("MYAPP_CONFIG_PATH", envPath)
				return []Option[testCfg2]{
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithPersistence[testCfg2]("anything"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				fileCreated:   false,
				pathHasSuffix: filepath.Join("present", "config.yaml"),
				outContains:   "loaded from",
				name:          "fromfile",
				count:         7,
			},
		},
		{
			name: "env override path present (bad yaml) => parse error returned",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				envPath := filepath.Join(td, "bad", "config.yaml")
				writeFile(t, envPath, "name: [unclosed\n")
				t.Setenv("MYAPP_CONFIG_PATH", envPath)
				return []Option[testCfg2]{
					WithEnvPrefix[testCfg2]("MYAPP"),
					// persist value does not matter here; parse error path triggers before create
					WithPersistence[testCfg2]("irrelevant"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				errContains:   "parse config file",
				pathHasSuffix: filepath.Join("bad", "config.yaml"),
			},
		},
		{
			name: "persistent via UserConfigDir (no env override) => load existing",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				// Pre-create ~/.config/myapp/config.yml under XDG_CONFIG_HOME set above
				p := filepath.Join(td, "xdg", "myapp", "config.yml")
				writeFile(t, p, "name: usercfg\ncount: 5\n")
				// No MYAPP_CONFIG_PATH
				t.Setenv("MYAPP_CONFIG_PATH", "")
				return []Option[testCfg2]{
					WithPersistence[testCfg2]("myapp"),
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				fileCreated:   false,
				pathHasSuffix: filepath.Join("xdg", "myapp", "config.yml"),
				outContains:   "loaded from",
				name:          "usercfg",
				count:         5,
			},
		},
		{
			name: "persistent via UserConfigDir (no file) => create new file",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				// Ensure no file exists at the resolved path
				t.Setenv("MYAPP_CONFIG_PATH", "")
				return []Option[testCfg2]{
					WithPersistence[testCfg2]("newapp"),
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				fileCreated:   true,
				pathHasSuffix: filepath.Join("xdg", "newapp", "config.yml"),
				outContains:   "created new config",
				name:          "default",
				count:         1,
			},
		},
		{
			name: "non-persistent with dirName set + UserConfigDir error => no error, warning to ErrOut",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				// Simulate error: clear XDG and HOME/USERPROFILE; then manually set dirName (non-persistent)
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
				t.Setenv("MYAPP_CONFIG_PATH", "")
				// Build without WithPersistence, but we need dirName non-empty to hit warning branch.
				p := New[testCfg2](
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				)
				// set dirName directly; tests are in same package
				p.dirName = "np-app"
				// run Get() and then restore env for subsequent cases
				return []Option[testCfg2]{
					// we'll ignore returned opts; we already created provider above
					// but to unify flow, we'll return nil and execute inline in test body
					// (special-cased below)
				}
			},
			want: want{
				fileCreated:   false,
				pathHasSuffix: "", // none
				errContainsLn: "warning: cannot determine user config dir",
				name:          "default",
				count:         1,
			},
		},
		{
			name: "env overrides applied after file load",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				envPath := filepath.Join(td, "over", "config.yaml")
				writeFile(t, envPath, "name: fromfile\ncount: 2\ndur: 1s\n")
				t.Setenv("MYAPP_CONFIG_PATH", envPath)
				// override values via env
				t.Setenv("MYAPP_NAME", "fromenv")
				t.Setenv("MYAPP_COUNT", "9")
				t.Setenv("MYAPP_DUR", "3s")
				return []Option[testCfg2]{
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithPersistence[testCfg2]("overapp"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				fileCreated:   false,
				pathHasSuffix: filepath.Join("over", "config.yaml"),
				outContains:   "loaded from",
				name:          "fromenv",
				count:         9,
			},
		},
		{
			name: "persistent: UserConfigDir error => returns error",
			setup: func(t *testing.T) []Option[testCfg2] {
				outBuf.Reset()
				errBuf.Reset()
				// Force error by clearing envs used by os.UserConfigDir
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
				t.Setenv("MYAPP_CONFIG_PATH", "")
				return []Option[testCfg2]{
					WithPersistence[testCfg2]("needcfg"),
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				}
			},
			want: want{
				errContains: "cannot determine user config dir",
			},
		},
		{
			name: "with model: defaults→file→env → validate ok",
			setup: func(t *testing.T) []Option[testCfg2] {
				// Prepare env and file; actual provider will be built inline with mCfg generic
				// in the test body switch-case.
				// Create a file with only name so default port stays until env override.
				envPath := filepath.Join(td, "model_ok", "config.yaml")
				if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				if err := os.WriteFile(envPath, []byte("name: fromfile\n"), 0o600); err != nil {
					t.Fatalf("seed write: %v", err)
				}
				t.Setenv("MYAPP_CONFIG_PATH", envPath)
				t.Setenv("MYAPP_PORT", "9090")
				return nil
			},
			want: want{},
		},
		{
			name: "with model: validation error",
			setup: func(t *testing.T) []Option[testCfg2] {
				// Set envs that violate validation rules; provider will be built inline with mCfg
				t.Setenv("MYAPP_NAME", "")
				t.Setenv("MYAPP_PORT", "0")
				// Clear any path override
				t.Setenv("MYAPP_CONFIG_PATH", "")
				return nil
			},
			want: want{errContains: "nonempty"}, // substring check in inline branch will be stricter
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			switch tt.name {
			case "with model: defaults→file→env → validate ok":
				if tt.setup != nil {
					tt.setup(t)
				}
				// Build and run provider using mCfg generic with model integration.
				p := New[mCfg](
					WithEnvPrefix[mCfg]("MYAPP"),
					WithDefaultFn[mCfg](func() *mCfg { return &mCfg{} }),
					WithModel[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) {
						return modellib.New(
							c,
							modellib.WithRules[mCfg, string](modellib.BuiltinStringRules()),
							modellib.WithRules[mCfg, int](modellib.BuiltinIntRules()),
						)
					}),
				)
				cfg, path, created, err := p.Get()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if created {
					t.Fatalf("expected fileCreated=false for existing env-path file")
				}
				if !strings.HasSuffix(path, filepath.Join("model_ok", "config.yaml")) {
					t.Fatalf("unexpected path: %s", path)
				}
				if cfg.Name != "fromfile" {
					t.Fatalf("Name: got %q, want %q", cfg.Name, "fromfile")
				}
				if cfg.Port != 9090 {
					t.Fatalf("Port: got %d, want %d", cfg.Port, 9090)
				}
				return

			case "with model: validation error":
				if tt.setup != nil {
					tt.setup(t)
				}
				p := New[mCfg](
					WithEnvPrefix[mCfg]("MYAPP"),
					WithDefaultFn[mCfg](func() *mCfg { return &mCfg{} }),
					WithModel[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) {
						return modellib.New(
							c,
							modellib.WithRules[mCfg, string](modellib.BuiltinStringRules()),
							modellib.WithRules[mCfg, int](modellib.BuiltinIntRules()),
						)
					}),
				)
				_, _, _, err := p.Get()
				if err == nil {
					t.Fatalf("expected validation error, got nil")
				}
				var ve *modellib.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected *model.ValidationError, got %T: %v", err, err)
				}
				msg := ve.Error()
				if !strings.Contains(msg, "nonempty") || !strings.Contains(msg, "nonzero") {
					t.Fatalf("validation error does not mention expected rules: %q", msg)
				}
				return

			case "non-persistent with dirName set + UserConfigDir error => no error, warning to ErrOut":
				// Special inline flow per setup
				outBuf.Reset()
				errBuf.Reset()
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
				t.Setenv("MYAPP_CONFIG_PATH", "")

				p := New[testCfg2](
					WithEnvPrefix[testCfg2]("MYAPP"),
					WithDefaultFn[testCfg2](defFn),
					WithStreams[testCfg2](streams),
				)
				p.dirName = "np-app" // non-persistent but has dirName

				cfg, path, created, err := p.Get()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if created != tt.want.fileCreated {
					t.Fatalf("fileCreated = %v, want %v", created, tt.want.fileCreated)
				}
				if tt.want.pathHasSuffix != "" && !strings.HasSuffix(path, tt.want.pathHasSuffix) {
					t.Fatalf("path %q does not end with %q", path, tt.want.pathHasSuffix)
				}
				if got := errBuf.String(); !strings.Contains(got, tt.want.errContainsLn) {
					t.Fatalf("expected ErrOut to contain %q, got %q", tt.want.errContainsLn, got)
				}
				if cfg.Name != tt.want.name || cfg.Count != tt.want.count {
					t.Fatalf("cfg mismatch: got %+v, want Name=%q Count=%d", cfg, tt.want.name, tt.want.count)
				}
				return
			case "with model: factory non-zero default wins over model default":
				if tt.setup != nil {
					tt.setup(t)
				}
				p := New[mCfg](
					WithEnvPrefix[mCfg]("MYAPP"),
					// Factory sets a NON-ZERO value for Name; model default should NOT overwrite it.
					WithDefaultFn[mCfg](func() *mCfg { return &mCfg{Name: "factory", Port: 0} }),
					WithModel[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) {
						return modellib.New(
							c,
							modellib.WithRules[mCfg, string](modellib.BuiltinStringRules()),
							modellib.WithRules[mCfg, int](modellib.BuiltinIntRules()),
						)
					}),
				)
				cfg, path, created, err := p.Get()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				// Non-persistent, no env/file; path may be empty; created must be false.
				if created {
					t.Fatalf("did not expect file creation in non-persistent case")
				}
				if cfg.Name != "factory" {
					t.Fatalf("Name: got %q, want %q (factory non-zero must not be overwritten by model default)", cfg.Name, "factory")
				}
				// Port default via model should fill zero (8080) since factory left it zero and no env/file.
				if cfg.Port != 8080 {
					t.Fatalf("Port: got %d, want %d (model default should fill zero)", cfg.Port, 8080)
				}
				_ = path
				return

			case "with model: model fills zero value when factory leaves zero":
				if tt.setup != nil {
					tt.setup(t)
				}
				p := New[mCfg](
					WithEnvPrefix[mCfg]("MYAPP"),
					// Factory leaves Name zero; model default should set it to "svc".
					WithDefaultFn[mCfg](func() *mCfg { return &mCfg{} }),
					WithModel[mCfg](func(c *mCfg) (*modellib.Model[mCfg], error) {
						return modellib.New(
							c,
							modellib.WithRules[mCfg, string](modellib.BuiltinStringRules()),
							modellib.WithRules[mCfg, int](modellib.BuiltinIntRules()),
						)
					}),
				)
				cfg, path, created, err := p.Get()
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if created {
					t.Fatalf("did not expect file creation in non-persistent case")
				}
				if cfg.Name != "svc" {
					t.Fatalf("Name: got %q, want %q (model default should fill zero)", cfg.Name, "svc")
				}
				if cfg.Port != 8080 {
					t.Fatalf("Port: got %d, want %d (model default should fill zero)", cfg.Port, 8080)
				}
				_ = path
				return
			default:
				opts := tt.setup(t)
				p := New[testCfg2](opts...)
				cfg, path, created, err := p.Get()

				if tt.want.errContains != "" {
					if err == nil || !strings.Contains(err.Error(), tt.want.errContains) {
						t.Fatalf("Get() error = %v, want contains %q", err, tt.want.errContains)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if created != tt.want.fileCreated {
					t.Fatalf("fileCreated = %v, want %v", created, tt.want.fileCreated)
				}
				if tt.want.pathHasSuffix != "" && !strings.HasSuffix(path, tt.want.pathHasSuffix) {
					t.Fatalf("path %q does not end with %q", path, tt.want.pathHasSuffix)
				}
				if tt.want.outContains != "" {
					if got := outBuf.String(); !strings.Contains(got, tt.want.outContains) {
						t.Fatalf("expected Out to contain %q, got %q", tt.want.outContains, got)
					}
				}
				if cfg.Name != tt.want.name || cfg.Count != tt.want.count {
					t.Fatalf("cfg mismatch: got %+v, want Name=%q Count=%d", cfg, tt.want.name, tt.want.count)
				}
			}
		})
	}
}

func TestProvider_Get_Once(t *testing.T) {
	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	var outBuf, errBuf bytes.Buffer
	streams := fakeStreams{out: &outBuf, errOut: &errBuf}

	// Use env override path that does not yet exist to force a create on first call.
	envPath := filepath.Join(td, "once", "config.yaml")
	t.Setenv("MYAPP_CONFIG_PATH", envPath)

	p := New[testCfg2](
		WithEnvPrefix[testCfg2]("MYAPP"),
		WithPersistence[testCfg2]("irrelevant"),
		WithDefaultFn[testCfg2](defFn),
		WithStreams[testCfg2](streams),
	)

	// First call should create the file and print "created new config ..."
	cfg1, path1, created1, err1 := p.Get()
	if err1 != nil {
		t.Fatalf("first Get error: %v", err1)
	}
	if !created1 {
		t.Fatalf("first Get expected fileCreated=true")
	}
	if !strings.HasSuffix(path1, filepath.Join("once", "config.yaml")) {
		t.Fatalf("first path unexpected: %s", path1)
	}
	if !strings.Contains(outBuf.String(), "created new config") {
		t.Fatalf("expected created message once, got: %q", outBuf.String())
	}

	// Second call should return cached values, not create again, and not append more messages.
	outBuf.Reset() // clear buffer to ensure no new messages appear
	cfg2, path2, created2, err2 := p.Get()
	if err2 != nil {
		t.Fatalf("second Get error: %v", err2)
	}
	if created2 != true { // still returns same cached fileCreated flag
		t.Fatalf("second Get expected fileCreated=true (cached), got %v", created2)
	}
	if path1 != path2 {
		t.Fatalf("path changed between calls: %q vs %q", path1, path2)
	}
	if cfg1 != cfg2 {
		t.Fatalf("cfg pointer changed between calls; want same cached instance")
	}
	if outBuf.Len() != 0 {
		t.Fatalf("expected no additional messages on second Get; got %q", outBuf.String())
	}
}

func TestProvider_Get_Concurrent_Create(t *testing.T) {
	// Stress concurrent Get() calls when the file does not exist yet.
	// Expect: exactly one init path runs, file is created once, all callers see
	// the same cfg pointer, path, and fileCreated=true.

	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	var outBuf, errBuf bytes.Buffer
	streams := fakeStreams{out: &outBuf, errOut: &errBuf}

	// Use env override path that does not yet exist to force a create on first init.
	envPath := filepath.Join(td, "conc", "config.yaml")
	t.Setenv("MYAPP_CONFIG_PATH", envPath)

	p := New[testCfg2](
		WithEnvPrefix[testCfg2]("MYAPP"),
		WithPersistence[testCfg2]("irrelevant"),
		WithDefaultFn[testCfg2](defFn),
		WithStreams[testCfg2](streams),
	)

	n := 32
	type res struct {
		cfg  *testCfg2
		path string
		cr   bool
		err  error
	}
	ch := make(chan res, n)

	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			cfg, path, created, err := p.Get()
			ch <- res{cfg, path, created, err}
		}()
	}
	close(start)
	wg.Wait()
	close(ch)

	var first res
	firstSet := false
	for r := range ch {
		if r.err != nil {
			t.Fatalf("unexpected error: %v", r.err)
		}
		if !firstSet {
			first = r
			firstSet = true
			continue
		}
		if r.cfg != first.cfg {
			t.Fatalf("cfg pointer mismatch: %p vs %p", r.cfg, first.cfg)
		}
		if r.path != first.path {
			t.Fatalf("path mismatch: %q vs %q", r.path, first.path)
		}
		if r.cr != first.cr {
			t.Fatalf("fileCreated mismatch: %v vs %v", r.cr, first.cr)
		}
	}

	if !strings.HasSuffix(first.path, filepath.Join("conc", "config.yaml")) {
		t.Fatalf("unexpected path: %s", first.path)
	}
	if !first.cr {
		t.Fatalf("expected fileCreated=true")
	}

	// Only one init should have printed the created message.
	if got := outBuf.String(); strings.Count(got, "created new config") != 1 {
		t.Fatalf("expected exactly one 'created new config' message, got: %q", got)
	}
}

func TestProvider_Get_Concurrent_LoadExisting(t *testing.T) {
	// Stress concurrent Get() calls when the file already exists.
	// Expect: no creation, all callers see same cfg pointer, path, fileCreated=false.

	td := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(td, "xdg"))
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")

	var outBuf, errBuf bytes.Buffer
	streams := fakeStreams{out: &outBuf, errOut: &errBuf}

	// Pre-create the env override path
	envPath := filepath.Join(td, "conc2", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(envPath, []byte("name: pre\ncount: 4\n"), 0o600); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	t.Setenv("MYAPP_CONFIG_PATH", envPath)

	p := New[testCfg2](
		WithEnvPrefix[testCfg2]("MYAPP"),
		WithPersistence[testCfg2]("irrelevant"),
		WithDefaultFn[testCfg2](defFn),
		WithStreams[testCfg2](streams),
	)

	n := 32
	type res struct {
		cfg  *testCfg2
		path string
		cr   bool
		err  error
	}
	ch := make(chan res, n)

	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			<-start
			cfg, path, created, err := p.Get()
			ch <- res{cfg, path, created, err}
		}()
	}
	close(start)
	wg.Wait()
	close(ch)

	var first res
	firstSet := false
	for r := range ch {
		if r.err != nil {
			t.Fatalf("unexpected error: %v", r.err)
		}
		if !firstSet {
			first = r
			firstSet = true
			continue
		}
		if r.cfg != first.cfg {
			t.Fatalf("cfg pointer mismatch: %p vs %p", r.cfg, first.cfg)
		}
		if r.path != first.path {
			t.Fatalf("path mismatch: %q vs %q", r.path, first.path)
		}
		if r.cr != first.cr {
			t.Fatalf("fileCreated mismatch: %v vs %v", r.cr, first.cr)
		}
	}

	if !strings.HasSuffix(first.path, filepath.Join("conc2", "config.yaml")) {
		t.Fatalf("unexpected path: %s", first.path)
	}
	if first.cr {
		t.Fatalf("expected fileCreated=false")
	}

	// Only one init should have printed the loaded message.
	if got := outBuf.String(); strings.Count(got, "loaded from") != 1 {
		t.Fatalf("expected exactly one 'loaded from' message, got: %q", got)
	}
}
