package config

import (
	"os"
	"testing"
	"time"
)

type envInner struct {
	Str  string        `env:"STR"`
	Skip string        `env:"-"`
	Dur  time.Duration `env:"DUR"`
	B    bool          `env:"BOOL"`
	I    int           `env:"INT"`
	U    uint          `env:"U"`
	NegU uint          `env:"NEG_U"`
}
type envCfg struct {
	S         string
	ApiKey2FA string
	Inner     envInner       `env:"INNER"`
	PtrInner  *envInner      `env:"PINNER"`
	PtrStr    *string        `env:"PSTR"`
	PtrBool   *bool          `env:"PBOOL"`
	PtrInt    *int           `env:"PINT"`
	PtrDur    *time.Duration `env:"PDUR"`
	PtrUint   *uint          `env:"PU"`
}

func TestApplyEnv_AllBranches_WithPrefix(t *testing.T) {
	const prefix = "APP"
	t.Setenv(prefix+"_S", "top")
	t.Setenv(prefix+"_API_KEY2FA", "k2fa")
	t.Setenv(prefix+"_INNER_STR", "in")
	t.Setenv(prefix+"_INNER_BOOL", "true")
	t.Setenv(prefix+"_INNER_INT", "42")
	t.Setenv(prefix+"_INNER_DUR", "1h30m")
	t.Setenv(prefix+"_INNER_U", "5")
	t.Setenv(prefix+"_INNER_NEG_U", "-3")
	t.Setenv(prefix+"_INNER_SKIP", "shouldSkip")
	t.Setenv(prefix+"_PINNER_STR", "pinner")
	t.Setenv(prefix+"_PSTR", "hello")
	t.Setenv(prefix+"_PBOOL", "1")
	t.Setenv(prefix+"_PINT", "7")
	t.Setenv(prefix+"_PDUR", "500ms")
	t.Setenv(prefix+"_PU", "9")
	var c envCfg
	applyEnvToTarget(&c, prefix, SetOverride)
	if c.S != "top" || c.ApiKey2FA != "k2fa" {
		t.Fatalf("top-level fields not applied: %+v", c)
	}
	if c.Inner.Str != "in" || !c.Inner.B || c.Inner.I != 42 || c.Inner.Dur != (time.Hour+30*time.Minute) || c.Inner.U != 5 {
		t.Fatalf("inner fields not applied: %+v", c.Inner)
	}
	if c.Inner.NegU != 0 || c.Inner.Skip != "" {
		t.Fatalf("ignored fields not preserved: %+v", c.Inner)
	}
	if c.PtrInner == nil || c.PtrInner.Str != "pinner" {
		t.Fatalf("PtrInner not allocated correctly: %+v", c.PtrInner)
	}
	if c.PtrStr == nil || *c.PtrStr != "hello" || c.PtrBool == nil || !*c.PtrBool || c.PtrInt == nil || *c.PtrInt != 7 || c.PtrDur == nil || *c.PtrDur != 500*time.Millisecond || c.PtrUint == nil || *c.PtrUint != 9 {
		t.Fatalf("pointer scalar fields not applied: %+v", c)
	}
}
func TestApplyEnv_NoPrefix_FallbackNames(t *testing.T) {
	t.Setenv("S", "nopfx")
	t.Setenv("INNER_STR", "inNoPfx")
	var c envCfg
	applyEnvToTarget(&c, "", SetOverride)
	if c.S != "nopfx" || c.Inner.Str != "inNoPfx" {
		t.Fatalf("unexpected cfg: %+v", c)
	}
}
func TestApplyEnv_NilPointer_NoOp(t *testing.T) {
	applyEnvToTarget(nil, "APP", SetOverride)
}
func TestApplyEnv_NoAllocation_WhenNoEnv(t *testing.T) {
	var c envCfg
	for _, key := range []string{"APP_PINNER_STR", "APP_PSTR", "APP_PBOOL", "APP_PINT", "APP_PDUR", "APP_PU"} {
		_ = os.Unsetenv(key)
	}
	applyEnvToTarget(&c, "APP", SetOverride)
	if c.PtrInner != nil || c.PtrStr != nil || c.PtrBool != nil || c.PtrInt != nil || c.PtrDur != nil || c.PtrUint != nil {
		t.Fatalf("expected no allocations, got %+v", c)
	}
}
func TestApplyEnv_ParseFailures_DoNotAllocate(t *testing.T) {
	t.Setenv("APP_PBOOL", "notabool")
	t.Setenv("APP_PINT", "NaN")
	t.Setenv("APP_PDUR", "notaduration")
	var c envCfg
	applyEnvToTarget(&c, "APP", SetOverride)
	if c.PtrBool != nil || c.PtrInt != nil || c.PtrDur != nil {
		t.Fatalf("invalid env values should not allocate pointers: %+v", c)
	}
}
func TestApplyEnv_NonStructPointer_NoPanic(t *testing.T) {
	var z int
	applyEnvToTarget(&z, "APP", SetOverride)
}
func TestEnvSetStrategy_FillZero(t *testing.T) {
	const prefix = "APPZ"
	t.Setenv(prefix+"_S", "envTop")
	t.Setenv(prefix+"_INNER_INT", "99")
	c := envCfg{S: "preset"}
	applyEnvToTarget(&c, prefix, SetFillZero)
	if c.S != "preset" || c.Inner.I != 99 {
		t.Fatalf("unexpected cfg after fill-zero: %+v", c)
	}
}
func TestToScreamingSnake_Boundaries(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "camelCase", want: "CAMEL_CASE"},
		{in: "APIKey", want: "API_KEY"},
		{in: "ApiKey2FA", want: "API_KEY2FA"},
		{in: "key2FA", want: "KEY2FA"},
		{in: "X", want: "X"},
		{in: "already_UPPER", want: "ALREADY_UPPER"},
		{in: "digit1End", want: "DIGIT1_END"},
	}

	for _, tt := range tests {
		if got := toScreamingSnake(tt.in); got != tt.want {
			t.Fatalf("toScreamingSnake(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}
func TestBuildEnvName(t *testing.T) {
	cases := []struct {
		prefix   string
		segments []string
		want     string
	}{
		{"", nil, ""},
		{"", []string{"A"}, "A"},
		{"P", nil, "P"},
		{"P", []string{"A", "B"}, "P_A_B"},
	}
	for _, tc := range cases {
		if got := buildEnvName(tc.prefix, tc.segments); got != tc.want {
			t.Fatalf("buildEnvName(%q,%v)=%q want %q", tc.prefix, tc.segments, got, tc.want)
		}
	}
}
func TestPrimitiveParsers(t *testing.T) {
	t.Setenv("X_BOOL", "true")
	t.Setenv("X_INT", "123")
	t.Setenv("X_DUR", "2s")
	if b, ok := getBool("X_BOOL"); !ok || !b {
		t.Fatalf("getBool failed")
	}
	if n, ok := getInt("X_INT"); !ok || n != 123 {
		t.Fatalf("getInt failed")
	}
	if d, ok := getDuration("X_DUR"); !ok || d != 2*time.Second {
		t.Fatalf("getDuration failed")
	}
}
