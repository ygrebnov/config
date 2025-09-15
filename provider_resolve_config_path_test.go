package config

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

// ---- Test helpers ----

//type testCfg struct{}

//type fakeStreams struct {
//	in     io.Reader
//	out    io.Writer
//	errOut io.Writer
//}
//
//func (s fakeStreams) In() io.Reader     { return s.in }
//func (s fakeStreams) Out() io.Writer    { return s.out }
//func (s fakeStreams) ErrOut() io.Writer { return s.errOut }

// newProvider is a tiny helper to build a Provider with common defaults for tests.
func newProvider(opts ...Option[testCfg]) *Provider[testCfg] {
	return New[testCfg](opts...) // New already injects defaultFn if nil
}

// ---- Tests ----

func TestProvider_resolveConfigPath(t *testing.T) {
	const (
		dirName = "myapp"
		prefix  = "MYAPP"
	)

	// We'll reuse these buffers in scenarios that validate streams output.
	var outBuf, errBuf bytes.Buffer
	fs := fakeStreams{out: &outBuf, errOut: &errBuf}

	type want struct {
		errContains    string // substring of error (if non-empty)
		configPath     string // exact path expected (if non-empty)
		errOutContains string // substring expected in ErrOut (if non-empty)
	}

	tests := []struct {
		name string
		// per-case setup (env, buffers reset, etc.)
		setup func(t *testing.T)
		// provider options to pass to New
		opts    []Option[testCfg]
		dirName string
		want    want
	}{
		{
			name: "env override takes precedence (prefix set, env var set)",
			setup: func(t *testing.T) {
				t.Setenv(prefix+"_CONFIG_PATH", "/tmp/override/config.yml")
			},
			opts: []Option[testCfg]{
				WithEnvPrefix[testCfg](prefix),
				WithPersistence[testCfg](dirName), // even with persistence, env wins
			},
			want: want{
				configPath: "/tmp/override/config.yml",
			},
		},
		{
			name: "non-persistent, no dirName => no path, no error",
			setup: func(t *testing.T) {
				t.Setenv(prefix+"_CONFIG_PATH", "") // ensure empty
			},
			opts: []Option[testCfg]{ /* no WithPersistence, no dirName */ },
			want: want{
				configPath: "", // remains empty
			},
		},
		{
			name: "env prefix set but no env var; persistent with dirName => uses UserConfigDir",
			setup: func(t *testing.T) {
				t.Setenv(prefix+"_CONFIG_PATH", "")
			},
			opts: []Option[testCfg]{
				WithEnvPrefix[testCfg](prefix),
				WithPersistence[testCfg](dirName),
			},
			want: want{
				// configPath asserted below with dynamic UserConfigDir
			},
		},
		{
			name: "persistent: UserConfigDir error => returns error",
			setup: func(t *testing.T) {
				// Make UserConfigDir likely fail by clearing HOME and XDG_CONFIG_HOME.
				// On most systems this causes os.UserConfigDir() to error.
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv(prefix+"_CONFIG_PATH", "")
				// reset buffers
				outBuf.Reset()
				errBuf.Reset()
			},
			opts: []Option[testCfg]{
				WithPersistence[testCfg](dirName),
				WithEnvPrefix[testCfg](prefix), // prefix irrelevant here (no override)
			},
			want: want{
				errContains: "cannot determine user config dir",
			},
		},
		{
			name: "non-persistent: UserConfigDir error => no error, warning to ErrOut if streams present",
			setup: func(t *testing.T) {
				t.Setenv("HOME", "")
				t.Setenv("USERPROFILE", "")
				t.Setenv("XDG_CONFIG_HOME", "")
				t.Setenv(prefix+"_CONFIG_PATH", "")
				outBuf.Reset()
				errBuf.Reset()
			},
			// No WithPersistence here!
			opts: []Option[testCfg]{
				WithEnvPrefix[testCfg](prefix),
				WithStreams[testCfg](fs),
			},
			dirName: "np-app",
			want: want{
				configPath:     "",
				errOutContains: "warning: cannot determine user config dir",
			},
		},
		{
			name: "persistent: normal path (no env override) => join(UserConfigDir, dirName, config.yml)",
			setup: func(t *testing.T) {
				t.Setenv(prefix+"_CONFIG_PATH", "")
				outBuf.Reset()
				errBuf.Reset()
			},
			opts: []Option[testCfg]{
				WithPersistence[testCfg](dirName),
			},
			want: want{
				// configPath asserted dynamically
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup(t)
			}

			// Build provider with options
			p := newProvider(tt.opts...)

			// To make resolveConfigPath try UserConfigDir.
			// This will trigger the warning path instead of returning an error.
			if tt.dirName != "" {
				p.dirName = tt.dirName
			}

			// Call resolveConfigPath
			err := p.resolveConfigPath()

			// Assertions on error
			if tt.want.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want.errContains) {
					t.Fatalf("resolveConfigPath() error = %v, want contains %q", err, tt.want.errContains)
				}
			} else if err != nil {
				t.Fatalf("resolveConfigPath() unexpected error: %v", err)
			}

			// Assertions on computed path
			if tt.want.configPath != "" {
				if p.configPath != tt.want.configPath {
					t.Fatalf("configPath = %q, want %q", p.configPath, tt.want.configPath)
				}
			} else {
				// For cases where we expect a default (UserConfigDir + dirName)
				if contains := strings.Contains(tt.name, "UserConfigDir"); contains && tt.want.errContains == "" && p.persist {
					// Only when we expect a real path and no error in persistent mode
					if p.configPath == "" && !strings.Contains(tt.name, "error") {
						t.Fatalf("configPath is empty; expected a joined path")
					}
					if p.configPath != "" && !strings.HasSuffix(p.configPath, filepath.Join(dirName, configFileName)) {
						t.Fatalf("configPath %q does not end with %q", p.configPath, filepath.Join(dirName, configFileName))
					}
				}
			}

			// Assertions on warning output
			if tt.want.errOutContains != "" {
				got := errBuf.String()
				if !strings.Contains(got, tt.want.errOutContains) {
					t.Fatalf("ErrOut does not contain %q; got: %q", tt.want.errOutContains, got)
				}
			} else {
				// In other cases, ensure we didn't accidentally write a warning.
				if s := errBuf.String(); s != "" && !strings.Contains(tt.name, "UserConfigDir error") {
					t.Fatalf("unexpected warning in ErrOut: %q", s)
				}
			}
		})
	}
}
