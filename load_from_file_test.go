package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

type sample struct {
	Name  string `json:"name" yaml:"name"`
	Count int    `json:"count" yaml:"count"`
}

func TestLoadFromFile(t *testing.T) {
	td := t.TempDir()
	write := func(name, contents string) string {
		p := filepath.Join(td, name)
		if err := os.WriteFile(p, []byte(contents), 0o600); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
		return p
	}

	yamlOKPath := write("good.yaml", "name: alice\ncount: 7\n")
	ymlOKPath := write("good.yml", "name: bob\ncount: 12\n")
	yamlBadPath := write("bad.yaml", "name: [unclosed\n")
	jsonOKPath := write("good.json", `{"name":"carol","count":3}`)
	jsonBadPath := write("bad.json", `{"name":"dave","count":,}`)
	txtPath := write("notes.txt", "just text")
	nonexistentYAML := filepath.Join(td, "missing.yaml")
	noExtPath := write("config", "name: x\n")

	tests := []struct {
		name        string
		path        string
		want        *sample
		errIs       error
		causeIs     error
		errContains string
	}{
		{name: "empty path", path: "", want: &sample{}},
		{name: "unsupported extension", path: txtPath, want: &sample{}, errIs: configerrors.ErrUnsupportedConfigFileType},
		{name: "no extension", path: noExtPath, want: &sample{}, errIs: configerrors.ErrUnsupportedConfigFileType},
		{name: "missing file", path: nonexistentYAML, want: &sample{}, errIs: configerrors.ErrReadFile, causeIs: os.ErrNotExist},
		{name: "yaml success", path: yamlOKPath, want: &sample{Name: "alice", Count: 7}},
		{name: "yml success", path: ymlOKPath, want: &sample{Name: "bob", Count: 12}},
		{name: "yaml parse error", path: yamlBadPath, errIs: configerrors.ErrParse},
		{name: "json success", path: jsonOKPath, want: &sample{Name: "carol", Count: 3}},
		{name: "json parse error", path: jsonBadPath, errIs: configerrors.ErrParse},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got sample
			err := loadFromFile(tt.path, &got)
			if tt.errIs != nil {
				if !errors.Is(err, tt.errIs) {
					t.Fatalf("expected errors.Is(err, %v), got %v", tt.errIs, err)
				}
				if tt.causeIs != nil && !errors.Is(err, tt.causeIs) {
					t.Fatalf("expected errors.Is(err, %v), got %v", tt.causeIs, err)
				}
			} else if tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %v", tt.errContains, err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != nil && err == nil && got != *tt.want {
				t.Fatalf("value mismatch: got=%+v want=%+v", got, *tt.want)
			}
		})
	}
}
