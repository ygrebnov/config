package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

type sampleCfg struct {
	Name  string `json:"name" yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

type yamlBad struct{ F func() }
type jsonBad struct{ F func() }

func TestWriteToFile(t *testing.T) {
	td := t.TempDir()
	tests := []struct {
		name          string
		path          func() string
		cfg           any
		wantErrIs     error
		wantCauseIs   error
		wantErrSubstr string
		verify        func(t *testing.T, path string)
	}{
		{
			name: "success yaml",
			path: func() string { return filepath.Join(td, "ok.yaml") },
			cfg:  &sampleCfg{Name: "alice", Count: 7},
			verify: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read back: %v", err)
				}
				if s := string(data); !strings.Contains(s, "name:") || !strings.Contains(s, "alice") {
					t.Fatalf("unexpected yaml content: %q", s)
				}
			},
		},
		{
			name: "success json",
			path: func() string { return filepath.Join(td, "ok.json") },
			cfg:  &sampleCfg{Name: "bob", Count: 12},
			verify: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read back: %v", err)
				}
				if s := string(data); !strings.Contains(s, `"name": "bob"`) || !strings.Contains(s, `"count": 12`) {
					t.Fatalf("unexpected json content: %q", s)
				}
			},
		},
		{
			name:      "unsupported extension",
			path:      func() string { return filepath.Join(td, "notes.txt") },
			cfg:       &sampleCfg{},
			wantErrIs: configerrors.ErrUnsupportedConfigFileType,
		},
		{
			name:      "marshal error yaml",
			path:      func() string { return filepath.Join(td, "bad.yaml") },
			cfg:       &yamlBad{F: func() {}},
			wantErrIs: configerrors.ErrFormat,
		},
		{
			name:      "marshal error json",
			path:      func() string { return filepath.Join(td, "bad.json") },
			cfg:       &jsonBad{F: func() {}},
			wantErrIs: configerrors.ErrFormat,
		},
		{
			name:        "missing parent directory",
			path:        func() string { return filepath.Join(td, "missing", "file.yaml") },
			cfg:         &sampleCfg{},
			wantErrIs:   configerrors.ErrCreateTempFile,
			wantCauseIs: os.ErrNotExist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path()
			err := writeToFile(path, tt.cfg)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("expected errors.Is(err, %v), got %v", tt.wantErrIs, err)
				}
				if tt.wantCauseIs != nil && !errors.Is(err, tt.wantCauseIs) {
					t.Fatalf("expected errors.Is(err, %v), got %v", tt.wantCauseIs, err)
				}
			} else if tt.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tt.wantErrSubstr, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.verify != nil {
				tt.verify(t, path)
			}
		})
	}
}
