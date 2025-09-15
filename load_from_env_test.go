package config

import (
	"os"
	"testing"
	"time"
)

// Build a config type that exercises all branches.
type envInner struct {
	Str  string        `env:"STR"`
	Skip string        `env:"-"`   // must be ignored even if env var exists
	Dur  time.Duration `env:"DUR"` // duration special-case
	B    bool          `env:"BOOL"`
	I    int           `env:"INT"`
	U    uint          `env:"U"`
	NegU uint          `env:"NEG_U"` // negative input should be ignored for uint
}

type envCfg struct {
	// No tag: fallback to SCREAMING_SNAKE
	S string

	// Tests boundary logic for toScreamingSnake (lower->upper + letter->digit)
	ApiKey2FA string

	// Nested struct with explicit segment
	Inner envInner `env:"INNER"`

	// Pointer-to-struct field: must allocate on demand
	PtrInner *envInner `env:"PINNER"`

	// Pointer scalars: must allocate on demand
	PtrStr  *string        `env:"PSTR"`
	PtrBool *bool          `env:"PBOOL"`
	PtrInt  *int           `env:"PINT"`
	PtrDur  *time.Duration `env:"PDUR"`
	PtrUint *uint          `env:"PU"`
}

func TestLoadFromEnv_AllBranches_WithPrefix(t *testing.T) {
	const prefix = "APP"

	// Clean environment for safety; t.Setenv will restore after test.
	clearFn := func(keys ...string) {
		for _, k := range keys {
			_ = os.Unsetenv(k)
		}
	}
	clearFn(
		prefix+"_S",
		prefix+"_API_KEY2FA",
		prefix+"_INNER_STR",
		prefix+"_INNER_BOOL",
		prefix+"_INNER_INT",
		prefix+"_INNER_DUR",
		prefix+"_INNER_U",
		prefix+"_INNER_NEG_U",
		prefix+"_INNER_SKIP", // should be ignored due to env:"-"
		prefix+"_PINNER_STR",
		prefix+"_PSTR",
		prefix+"_PBOOL",
		prefix+"_PINT",
		prefix+"_PDUR",
		prefix+"_PU",
	)

	// Set environment covering all supported kinds
	t.Setenv(prefix+"_S", "top")
	t.Setenv(prefix+"_API_KEY2FA", "k2fa")
	t.Setenv(prefix+"_INNER_STR", "in")
	t.Setenv(prefix+"_INNER_BOOL", "true")
	t.Setenv(prefix+"_INNER_INT", "42")
	t.Setenv(prefix+"_INNER_DUR", "1h30m")
	t.Setenv(prefix+"_INNER_U", "5")
	t.Setenv(prefix+"_INNER_NEG_U", "-3")        // must be ignored for uint
	t.Setenv(prefix+"_INNER_SKIP", "shouldSkip") // must be ignored due to env:"-"
	t.Setenv(prefix+"_PINNER_STR", "pinner")     // forces allocation of PtrInner
	t.Setenv(prefix+"_PSTR", "hello")            // forces allocation
	t.Setenv(prefix+"_PBOOL", "1")               // forces allocation & true
	t.Setenv(prefix+"_PINT", "7")                // forces allocation
	t.Setenv(prefix+"_PDUR", "500ms")            // forces allocation
	t.Setenv(prefix+"_PU", "9")                  // forces allocation

	var c envCfg
	p := New[envCfg](
		WithEnvPrefix[envCfg](prefix),
		WithDefaultFn(func() *envCfg { return &envCfg{} }),
	)
	p.loadFromEnv(&c)

	// Top-level, default SCREAMING_SNAKE
	if c.S != "top" {
		t.Fatalf("S: got %q, want %q", c.S, "top")
	}

	// SCREAMING_SNAKE boundary checks via ApiKey2FA -> API_KEY2FA
	if c.ApiKey2FA != "k2fa" {
		t.Fatalf("ApiKey2FA: got %q, want %q", c.ApiKey2FA, "k2fa")
	}

	// Nested struct values
	if c.Inner.Str != "in" {
		t.Fatalf("Inner.Str: got %q, want %q", c.Inner.Str, "in")
	}
	if c.Inner.B != true {
		t.Fatalf("Inner.B: got %v, want %v", c.Inner.B, true)
	}
	if c.Inner.I != 42 {
		t.Fatalf("Inner.I: got %d, want %d", c.Inner.I, 42)
	}
	if c.Inner.Dur != (time.Hour + 30*time.Minute) {
		t.Fatalf("Inner.Dur: got %v, want %v", c.Inner.Dur, time.Hour+30*time.Minute)
	}
	if c.Inner.U != 5 {
		t.Fatalf("Inner.U: got %d, want %d", c.Inner.U, 5)
	}
	// Negative value for uint should be ignored (stay zero)
	if c.Inner.NegU != 0 {
		t.Fatalf("Inner.NegU: got %d, want 0 (negatives ignored)", c.Inner.NegU)
	}
	// Skip must remain default
	if c.Inner.Skip != "" {
		t.Fatalf("Inner.Skip: got %q, want empty (env:\"-\" ignored)", c.Inner.Skip)
	}

	// Pointer-to-struct allocated & set
	if c.PtrInner == nil || c.PtrInner.Str != "pinner" {
		t.Fatalf("PtrInner.Str: got %v, want 'pinner'", c.PtrInner)
	}

	// Pointer scalar allocations
	if c.PtrStr == nil || *c.PtrStr != "hello" {
		t.Fatalf("PtrStr: got %v, want ptr to 'hello'", c.PtrStr)
	}
	if c.PtrBool == nil || *c.PtrBool != true {
		t.Fatalf("PtrBool: got %v, want ptr to true", c.PtrBool)
	}
	if c.PtrInt == nil || *c.PtrInt != 7 {
		t.Fatalf("PtrInt: got %v, want ptr to 7", c.PtrInt)
	}
	if c.PtrDur == nil || *c.PtrDur != 500*time.Millisecond {
		t.Fatalf("PtrDur: got %v, want ptr to 500ms", c.PtrDur)
	}
	if c.PtrUint == nil || *c.PtrUint != 9 {
		t.Fatalf("PtrUint: got %v, want ptr to 9", c.PtrUint)
	}
}

func TestLoadFromEnv_NoPrefix_FallbackNames(t *testing.T) {
	// Verify that when envPrefix == "", buildEnvName joins only segments.
	// We'll set env vars without prefix and ensure they are picked up.
	_ = os.Unsetenv("S")
	_ = os.Unsetenv("INNER_STR")

	t.Setenv("S", "nopfx")
	t.Setenv("INNER_STR", "inNoPfx")

	var c envCfg
	p := New[envCfg](
		WithDefaultFn(func() *envCfg { return &envCfg{} }),
	)
	p.loadFromEnv(&c)

	if c.S != "nopfx" {
		t.Fatalf("S (no prefix): got %q, want %q", c.S, "nopfx")
	}
	if c.Inner.Str != "inNoPfx" {
		t.Fatalf("Inner.Str (no prefix): got %q, want %q", c.Inner.Str, "inNoPfx")
	}
}

func TestLoadFromEnv_NilPointer_NoOp(t *testing.T) {
	p := New[envCfg](
		WithEnvPrefix[envCfg]("APP"),
		WithDefaultFn(func() *envCfg { return &envCfg{} }),
	)
	// Calling with nil must be a no-op
	p.loadFromEnv(nil)
	// Nothing to assert other than "did not panic"
}

func TestLoadFromEnv_NoAllocation_WhenNoEnv(t *testing.T) {
	// Ensure pointer fields are NOT allocated when no env var is present
	var c envCfg
	p := New[envCfg](
		WithEnvPrefix[envCfg]("APP"),
		WithDefaultFn(func() *envCfg { return &envCfg{} }),
	)
	// Make sure relevant env vars are NOT set
	for _, k := range []string{
		"APP_PINNER_STR", "APP_PSTR", "APP_PBOOL", "APP_PINT", "APP_PDUR", "APP_PU",
	} {
		_ = os.Unsetenv(k)
	}
	p.loadFromEnv(&c)

	if c.PtrInner != nil {
		t.Fatalf("PtrInner should remain nil when no env present")
	}
	if c.PtrStr != nil || c.PtrBool != nil || c.PtrInt != nil || c.PtrDur != nil || c.PtrUint != nil {
		t.Fatalf("pointer scalar fields should remain nil when no env present")
	}
}

// Additional targeted tests for edge parsing errors on duration/bool/int
func TestLoadFromEnv_ParseFailures_DoNotAllocate(t *testing.T) {
	// Provide invalid values to ensure we don't allocate/set for pointer scalars.
	_ = os.Unsetenv("APP_PBOOL")
	_ = os.Unsetenv("APP_PINT")
	_ = os.Unsetenv("APP_PDUR")

	t.Setenv("APP_PBOOL", "notabool")
	t.Setenv("APP_PINT", "NaN")
	t.Setenv("APP_PDUR", "notaduration")

	var c envCfg
	p := New[envCfg](
		WithEnvPrefix[envCfg]("APP"),
		WithDefaultFn(func() *envCfg { return &envCfg{} }),
	)
	p.loadFromEnv(&c)

	if c.PtrBool != nil || c.PtrInt != nil || c.PtrDur != nil {
		t.Fatalf("invalid parse should not allocate pointer fields")
	}
}

// Ensure SCREAMING_SNAKE fallback naming is correct for mixed-case with digits.
func TestToScreamingSnake_Boundaries(t *testing.T) {
	name := "ApiKey2FA"
	got := toScreamingSnake(name)
	// expect underscore between lower->upper and letter->digit, but not between consecutive uppers
	want := "API_KEY2FA"
	if got != want {
		t.Fatalf("toScreamingSnake(%q) = %q, want %q", name, got, want)
	}
}

// Ensure buildEnvName behaves for all branches
func TestBuildEnvName(t *testing.T) {
	type tc struct {
		prefix   string
		segments []string
		want     string
	}
	cases := []tc{
		{"", nil, ""},
		{"", []string{"A"}, "A"},
		{"P", nil, "P"},
		{"P", []string{"A", "B"}, "P_A_B"},
	}
	for _, c := range cases {
		got := buildEnvName(c.prefix, c.segments)
		if got != c.want {
			t.Fatalf("buildEnvName(%q,%v)=%q, want %q", c.prefix, c.segments, got, c.want)
		}
	}
}

// Quick sanity check for getBool/getInt/getDuration (via env)
func TestPrimitiveParsers(t *testing.T) {
	t.Setenv("X_BOOL", "true")
	t.Setenv("X_INT", "123")
	t.Setenv("X_DUR", "2s")
	if b, ok := getBool("X_BOOL"); !ok || !b {
		t.Fatalf("getBool failed")
	}
	if n, ok := getInt("X_INT"); !ok || n != 123 {
		t.Fatalf("getInt failed: %v %v", n, ok)
	}
	if d, ok := getDuration("X_DUR"); !ok || d != 2*time.Second {
		t.Fatalf("getDuration failed: %v %v", d, ok)
	}
	// Negative int for unsigned path is handled in applyEnv; parsers just return the value.
}

// Guard that loadFromEnv does not explode on non-struct pointers accidentally passed
func TestLoadFromEnv_NonStructPointer_NoPanic(t *testing.T) {
	p := New[int](WithEnvPrefix[int]("APP"), WithDefaultFn(func() *int { var z int; return &z }))
	// use reflection path: method expects *T; passing &z simulates non-struct *int
	var z int
	// This should be a no-op (applyEnv requires a struct)
	p.loadFromEnv(&z) // should not panic
	_ = z
}
