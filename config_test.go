package config

import (
	"bytes"
	"io"
	"strings"
	"testing"

	modellib "github.com/ygrebnov/model"
)

// test type for T
type testCfg struct {
	Answer int
}

// Minimal IOStreams-like stub used only for testing.
// It must satisfy the IOStreams interface used by Provider.
type fakeStreams struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func (s fakeStreams) In() io.Reader     { return s.in }
func (s fakeStreams) Out() io.Writer    { return s.out }
func (s fakeStreams) ErrOut() io.Writer { return s.errOut }

func TestNew(t *testing.T) {
	// Common fixtures
	defaultFn := func() *testCfg { return &testCfg{Answer: 42} }
	// zeroFn := func() *testCfg { return &testCfg{} }
	dir := "myapp"
	pfx := "MYAPP"
	fs := fakeStreams{
		in:     strings.NewReader(""),
		out:    &bytes.Buffer{},
		errOut: &bytes.Buffer{},
	}

	type args struct {
		withDefault   bool
		withPersist   bool
		withEnvPrefix bool
		withStreams   bool
		withModel     bool
	}
	type want struct {
		persist       bool
		dirName       string
		envPrefix     string
		cfgIsNil      bool // New should NOT instantiate cfg; it’s created in Get()
		defaultIsZero bool // calling defaultFn() yields zero value struct
		defaultIs42   bool // calling defaultFn() yields Answer=42
		hasStreams    bool
		hasModelInit  bool
	}

	// Generate all 2^4 = 16 combinations of options
	tests := []struct {
		name string
		args args
		want want
	}{
		{
			name: "no options",
			args: args{},
			want: want{
				persist:       false,
				dirName:       "",
				envPrefix:     "",
				cfgIsNil:      true,
				defaultIsZero: true, // New supplies zero default if none provided
				defaultIs42:   false,
				hasStreams:    false,
			},
		},
		{
			name: "WithDefaultFn only",
			args: args{withDefault: true},
			want: want{
				cfgIsNil:      true,
				defaultIsZero: false,
				defaultIs42:   true,
			},
		},
		{
			name: "WithPersistence only",
			args: args{withPersist: true},
			want: want{
				persist:       true,
				dirName:       dir,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithEnvPrefix only",
			args: args{withEnvPrefix: true},
			want: want{
				envPrefix:     pfx,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithStreams only",
			args: args{withStreams: true},
			want: want{
				hasStreams:    true,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithPersistence + WithEnvPrefix",
			args: args{withPersist: true, withEnvPrefix: true},
			want: want{
				persist:       true,
				dirName:       dir,
				envPrefix:     pfx,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithPersistence + WithDefaultFn",
			args: args{withPersist: true, withDefault: true},
			want: want{
				persist:     true,
				dirName:     dir,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithEnvPrefix + WithDefaultFn",
			args: args{withEnvPrefix: true, withDefault: true},
			want: want{
				envPrefix:   pfx,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithStreams + WithDefaultFn",
			args: args{withStreams: true, withDefault: true},
			want: want{
				hasStreams:  true,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithStreams + WithPersistence",
			args: args{withStreams: true, withPersist: true},
			want: want{
				hasStreams:    true,
				persist:       true,
				dirName:       dir,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithStreams + WithEnvPrefix",
			args: args{withStreams: true, withEnvPrefix: true},
			want: want{
				hasStreams:    true,
				envPrefix:     pfx,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithPersistence + WithEnvPrefix + WithDefaultFn",
			args: args{withPersist: true, withEnvPrefix: true, withDefault: true},
			want: want{
				persist:     true,
				dirName:     dir,
				envPrefix:   pfx,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithPersistence + WithStreams + WithDefaultFn",
			args: args{withPersist: true, withStreams: true, withDefault: true},
			want: want{
				persist:     true,
				dirName:     dir,
				hasStreams:  true,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithEnvPrefix + WithStreams + WithDefaultFn",
			args: args{withEnvPrefix: true, withStreams: true, withDefault: true},
			want: want{
				envPrefix:   pfx,
				hasStreams:  true,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithPersistence + WithEnvPrefix + WithStreams",
			args: args{withPersist: true, withEnvPrefix: true, withStreams: true},
			want: want{
				persist:       true,
				dirName:       dir,
				envPrefix:     pfx,
				hasStreams:    true,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithPersistence + WithEnvPrefix + WithStreams + WithDefaultFn (all)",
			args: args{withPersist: true, withEnvPrefix: true, withStreams: true, withDefault: true},
			want: want{
				persist:     true,
				dirName:     dir,
				envPrefix:   pfx,
				hasStreams:  true,
				cfgIsNil:    true,
				defaultIs42: true,
			},
		},
		{
			name: "WithModel only",
			args: args{withModel: true},
			want: want{
				hasModelInit:  true,
				cfgIsNil:      true,
				defaultIsZero: true,
			},
		},
		{
			name: "WithModel + WithDefaultFn",
			args: args{withModel: true, withDefault: true},
			want: want{
				hasModelInit: true,
				cfgIsNil:     true,
				defaultIs42:  true,
			},
		},
		{
			name: "WithModel + WithPersistence + WithEnvPrefix + WithStreams + WithDefaultFn",
			args: args{withModel: true, withPersist: true, withEnvPrefix: true, withStreams: true, withDefault: true},
			want: want{
				persist:      true,
				dirName:      dir,
				envPrefix:    pfx,
				hasStreams:   true,
				hasModelInit: true,
				cfgIsNil:     true,
				defaultIs42:  true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt // capture
		t.Run(tt.name, func(t *testing.T) {
			mInit := func(*testCfg) (*modellib.Model[testCfg], error) { return nil, nil }
			var opts []Option[testCfg]

			if tt.args.withPersist {
				opts = append(opts, WithPersistence[testCfg](dir))
			}
			if tt.args.withEnvPrefix {
				opts = append(opts, WithEnvPrefix[testCfg](pfx))
			}
			if tt.args.withModel {
				opts = append(opts, WithModel[testCfg](mInit))
			}
			if tt.args.withStreams {
				opts = append(opts, WithStreams[testCfg](fs))
			}
			if tt.args.withDefault {
				opts = append(opts, WithDefaultFn[testCfg](defaultFn))
			}

			p := New[testCfg](opts...)

			// Assert persist + dirName
			if got := p.persist; got != tt.want.persist {
				t.Fatalf("persist: got %v, want %v", got, tt.want.persist)
			}
			if got := p.dirName; got != tt.want.dirName {
				t.Fatalf("dirName: got %q, want %q", got, tt.want.dirName)
			}

			// Assert envPrefix
			if got := p.envPrefix; got != tt.want.envPrefix {
				t.Fatalf("envPrefix: got %q, want %q", got, tt.want.envPrefix)
			}

			// cfg must be nil after New (Get() constructs it)
			if (p.cfg == nil) != tt.want.cfgIsNil {
				t.Fatalf("cfgIsNil: got %v, want %v", p.cfg == nil, tt.want.cfgIsNil)
			}

			// defaultFn must be non-nil always
			if p.defaultFn == nil {
				t.Fatalf("defaultFn must be set")
			}

			// Check behavior of defaultFn()
			df := p.defaultFn()
			switch {
			case tt.want.defaultIs42:
				if df == nil || df.Answer != 42 {
					t.Fatalf("defaultFn(): expected Answer=42, got %+v", df)
				}
			case tt.want.defaultIsZero:
				if df == nil || df.Answer != 0 {
					t.Fatalf("defaultFn(): expected zero-value struct, got %+v", df)
				}
			}

			// Streams presence check
			if tt.want.hasStreams {
				if p.streams == nil || p.streams.Out() == nil || p.streams.ErrOut() == nil {
					t.Fatalf("streams: expected non-nil streams with Out and ErrOut")
				}
			} else {
				if p.streams != nil {
					// We allow In to be nil, but the interface being non-nil when not set indicates option applied.
					t.Fatalf("streams: expected nil, got non-nil")
				}
			}

			// Model init presence check
			if tt.want.hasModelInit {
				if p.modelInit == nil {
					t.Fatalf("modelInit: expected non-nil")
				}
			} else {
				if p.modelInit != nil {
					t.Fatalf("modelInit: expected nil")
				}
			}
		})
	}
}

func TestNew_Panics(t *testing.T) {
	t.Run("WithPersistence empty dirName panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic, got none")
			}
		}()
		_ = New[testCfg](WithPersistence[testCfg](""))
	})

	t.Run("WithEnvPrefix empty panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic, got none")
			}
		}()
		_ = New[testCfg](WithEnvPrefix[testCfg](""))
	})

	t.Run("WithDefaultFn nil panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic, got none")
			}
		}()
		var nilFn func() *testCfg
		_ = New[testCfg](WithDefaultFn[testCfg](nilFn))
	})

	t.Run("No defaultFn provided => New injects zero default", func(t *testing.T) {
		p := New[testCfg]() // no options
		if p.defaultFn == nil {
			t.Fatalf("defaultFn must be auto-initialized")
		}
		if got := p.defaultFn(); got == nil || got.Answer != 0 {
			t.Fatalf("auto defaultFn should return zero-value; got %+v", got)
		}
	})

	t.Run("Custom defaultFn overrides auto", func(t *testing.T) {
		p := New[testCfg](WithDefaultFn[testCfg](func() *testCfg { return &testCfg{Answer: 7} }))
		if p.defaultFn == nil {
			t.Fatalf("defaultFn must be set")
		}
		if got := p.defaultFn(); got == nil || got.Answer != 7 {
			t.Fatalf("custom defaultFn not applied; got %+v", got)
		}
	})

	t.Run("WithModel nil panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic, got none")
			}
		}()
		_ = New[testCfg](WithModel[testCfg](nil))
	})
}
