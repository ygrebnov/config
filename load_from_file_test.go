package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sample config struct for (de)serialization
type sample struct {
	Name  string `json:"name" yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

func TestLoadFromFile(t *testing.T) {
	td := t.TempDir()

	write := func(t *testing.T, name, contents string) string {
		t.Helper()
		p := filepath.Join(td, name)
		if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		return p
	}

	// Prepare files for scenarios
	yamlOKPath := write(t, "good.yaml", "name: alice\ncount: 7\n")
	ymlOKPath := write(t, "good.yml", "name: bob\ncount: 12\n")
	yamlBadPath := write(t, "bad.yaml", "name: [unclosed\n") // invalid YAML
	jsonOKPath := write(t, "good.json", `{"name":"carol","count":3}`)
	jsonBadPath := write(t, "bad.json", `{"name":"dave","count":,}`) // invalid JSON
	txtPath := write(t, "notes.txt", "just text")                    // unsupported ext

	nonexistentYAML := filepath.Join(td, "missing.yaml") // doesn't exist
	noExtPath := write(t, "config", "name: x\n")         // no extension -> unsupported

	tests := []struct {
		name        string
		path        string
		want        *sample
		wantErr     bool
		errIs       error // use errors.Is
		errContains string
	}{
		{
			name: "empty path => no-op",
			path: "",
			want: &sample{}, // unchanged
		},
		{
			name:  "unsupported extension .txt",
			path:  txtPath,
			want:  &sample{},
			errIs: ErrUnsupportedConfigFileType,
		},
		{
			name:  "no extension => unsupported",
			path:  noExtPath,
			want:  &sample{},
			errIs: ErrUnsupportedConfigFileType,
		},
		{
			name:        "read error (nonexistent file) wraps os.ErrNotExist",
			path:        nonexistentYAML,
			want:        &sample{},
			wantErr:     true,
			errContains: "read ", // function prefixes with "read <path>:"
		},
		{
			name: "YAML success (.yaml)",
			path: yamlOKPath,
			want: &sample{Name: "alice", Count: 7},
		},
		{
			name: "YAML success (.yml)",
			path: ymlOKPath,
			want: &sample{Name: "bob", Count: 12},
		},
		{
			name:    "YAML parse error",
			path:    yamlBadPath,
			wantErr: true,
			errIs:   ErrParse,
		},
		{
			name: "JSON success",
			path: jsonOKPath,
			want: &sample{Name: "carol", Count: 3},
		},
		{
			name:    "JSON parse error",
			path:    jsonBadPath,
			wantErr: true,
			errIs:   ErrParse,
		},
	}

	for _, tt := range tests {
		tt := tt // capture
		t.Run(tt.name, func(t *testing.T) {
			var got sample
			err := loadFromFile(tt.path, &got)

			// Error assertions
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("expected errors.Is(err, %v) to be true, got err=%v", tt.errIs, err)
				}
			} else if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			// Additional contains check (for read wrapper prefix)
			if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
				t.Fatalf("error %v does not contain %q", err, tt.errContains)
			}

			// Value assertions (when we expect success or no-op)
			if tt.want != nil && err == nil {
				if got != *tt.want {
					t.Fatalf("value mismatch: got=%+v want=%+v", got, *tt.want)
				}
			}
		})
	}
}
