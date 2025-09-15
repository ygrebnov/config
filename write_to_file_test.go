package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Simple serializable type for success cases
type sampleCfg struct {
	Name  string `json:"name" yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

// Types that will fail marshaling
type yamlBad struct {
	F func() // YAML marshaller should error on functions
}

type jsonBad struct {
	F func() // JSON marshaller errors on functions (unsupported type)
}

func TestWriteToFile(t *testing.T) {
	td := t.TempDir()

	tests := []struct {
		name          string
		path          func() string // build per-test path
		cfg           any
		wantErrIs     error                        // errors.Is(err, wantErrIs) if set
		wantErrSubstr string                       // substring in error, if set
		verify        func(t *testing.T, p string) // extra verification on success/after call
	}{
		{
			name: "success: yaml extension",
			path: func() string { return filepath.Join(td, "ok.yaml") },
			cfg:  &sampleCfg{Name: "alice", Count: 7},
			verify: func(t *testing.T, p string) {
				// file should exist and contain 'name: "alice"' or 'name: alice'
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatalf("read back: %v", err)
				}
				s := string(b)
				if !strings.Contains(s, "name:") || !strings.Contains(s, "alice") {
					t.Fatalf("yaml content not as expected: %q", s)
				}
			},
		},
		{
			name: "success: json extension",
			path: func() string { return filepath.Join(td, "ok.json") },
			cfg:  &sampleCfg{Name: "bob", Count: 12},
			verify: func(t *testing.T, p string) {
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatalf("read back: %v", err)
				}
				if got := string(b); !strings.Contains(got, `"name": "bob"`) || !strings.Contains(got, `"count": 12`) {
					t.Fatalf("json content not as expected: %q", got)
				}
			},
		},
		{
			name: "success: no extension -> yaml by default",
			path: func() string { return filepath.Join(td, "config") }, // no extension
			cfg:  &sampleCfg{Name: "carol", Count: 3},
			verify: func(t *testing.T, p string) {
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatalf("read back: %v", err)
				}
				s := string(b)
				if !strings.Contains(s, "name:") || !strings.Contains(s, "carol") {
					t.Fatalf("yaml content not as expected: %q", s)
				}
			},
		},
		{
			name:      "unsupported extension .txt",
			path:      func() string { return filepath.Join(td, "notes.txt") },
			cfg:       &sampleCfg{},
			wantErrIs: ErrUnsupportedConfigFileType,
		},
		{
			name:      "marshal error: yaml",
			path:      func() string { return filepath.Join(td, "bad.yaml") },
			cfg:       &yamlBad{F: func() {}},
			wantErrIs: ErrFormat,
		},
		{
			name:      "marshal error: json",
			path:      func() string { return filepath.Join(td, "bad.json") },
			cfg:       &jsonBad{F: func() {}},
			wantErrIs: ErrFormat,
		},
		{
			name: "create temp file error: parent dir does not exist",
			path: func() string {
				// point to a file under a non-existent subdirectory
				return filepath.Join(td, "no_such_dir", "file.yaml")
			},
			cfg:           &sampleCfg{},
			wantErrSubstr: "create temp file",
		},
		{
			name: "rename error: destination is a directory",
			path: func() string {
				// Make a directory and use it as the "file" path (no extension)
				dir := filepath.Join(td, "destdir")
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatalf("mkdir: %v", err)
				}
				return dir // no filename; rename to a directory should fail
			},
			cfg:           &sampleCfg{Name: "x"},
			wantErrSubstr: "rename temp file",
			verify: func(t *testing.T, p string) {
				// Ensure it is still a directory and not replaced
				info, err := os.Stat(p)
				if err != nil {
					t.Fatalf("stat: %v", err)
				}
				if !info.IsDir() {
					t.Fatalf("expected a directory to remain at %s", p)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture
		t.Run(tt.name, func(t *testing.T) {
			p := tt.path()
			err := writeToFile(p, tt.cfg)

			// Error expectations
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("errors.Is(err, %v) = false; err = %v", tt.wantErrIs, err)
				}
			} else if tt.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrSubstr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Optional verification
			if tt.verify != nil {
				tt.verify(t, p)
			}
		})
	}
}
