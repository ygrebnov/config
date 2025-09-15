[![GoDoc](https://pkg.go.dev/badge/github.com/ygrebnov/config)](https://pkg.go.dev/github.com/ygrebnov/config)
[![Build Status](https://github.com/ygrebnov/config/actions/workflows/build.yml/badge.svg)](https://github.com/ygrebnov/config/actions/workflows/build.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ygrebnov/config)](https://goreportcard.com/report/github.com/ygrebnov/config)

# config — a tiny, opinionated config loader for Go apps

**config** is a small, composable library that helps your Go program load configuration in a sane order:
1.	**Defaults** — start from a fresh struct using your default factory
2.	**File** — optionally read YAML/JSON from a user config directory or an env-overridden path
3.	**Environment** — override fields from env vars (with env tags or auto names)
4.	**Validation (optional)** — integrate with github.com/ygrebnov/model to apply defaults via default tags and validate with validate tags

It’s thread-safe, runs initialization **exactly once**, and lets you choose how user-facing messages are emitted (stdout/stderr, logger, in-memory buffer, or discarded) through **streams adapters**.

---

## Why use this library?

- **Clear lifecycle:** deterministic order (defaults → file → env → validate)
- **Minimal API:** one constructor, a few options, one Get()
- **Safe & concurrent:** Get() is safe from multiple goroutines; init runs only once
- **Flexible I/O:** plug in streams adapters (silent mode, buffers, slog)
- **Optional power-ups:** easy defaults + validation via github.com/ygrebnov/model

---

## Install

```bash
go get github.com/ygrebnov/config
```

If you plan to use the optional validation/defaults integration:
```bash
go get github.com/ygrebnov/model
```

And for the streams adapters:
```bash
go get github.com/ygrebnov/streams
```

---

## Quick start (minimal example)

```go
package main

import (
	"fmt"
	"log"

	"github.com/ygrebnov/config"
)

type Cfg struct {
	Name string `yaml:"name" env:"NAME"`
	Port int    `yaml:"port" env:"PORT"`
}

func main() {
	// Build a provider with:
	// - default factory (used if no file/env overrides)
	// - persistence under ~/.config/myapp/config.yml (or XDG_CONFIG_HOME)
	// - env prefix "MYAPP" so MYAPP_NAME & MYAPP_PORT override values
	// Note: defaults come from the factory unless WithModel is used
	p := config.New[Cfg](
		config.WithDefaultFn(func() *Cfg { return &Cfg{Name: "default", Port: 8080} }),
		config.WithPersistence[Cfg]("myapp"),
		config.WithEnvPrefix[Cfg]("MYAPP"),
	)

	cfg, path, created, err := p.Get()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Config: %+v\n", *cfg)
	fmt.Printf("File path: %s\n", path)
	fmt.Printf("Created: %v\n", created)
}
```

**What happens?**
- A new Cfg is created with defaults (Name="default", Port=8080)
- If ~/.config/myapp/config.yml exists, it’s loaded to override defaults
- Env vars (like MYAPP_NAME, MYAPP_PORT) override everything else
- You get the final *Cfg, the file path (if any), and whether a file was created

---

## File format & precedence

- **Supported formats:** .yml, .yaml, .json
- **Config path:**
  - If WithEnvPrefix("MYAPP") is set and MYAPP_CONFIG_PATH is set, that path **wins**
  - Else if WithPersistence("dir") is set, path is $(XDG_CONFIG_HOME|UserConfigDir)/dir/config.yml
  - Else (non-persistent), no file I/O is performed
- **Precedence:**
  defaults → file → env

---

## Environment variable mapping

By default, env var names are derived from field names:
- Field ApiKey2FA → API_KEY2FA (lower→upper; **no** split between letters & digits)
- If WithEnvPrefix("MYAPP") is set, names become MYAPP_API_KEY2FA

You can override the name with an env tag:
```go
type Cfg struct {
    Name string `env:"NAME"`
    // Skip an env override:
    Ignored string `env:"-"`
}
```

Pointers are allocated **on demand**:
- For pointer-to-struct fields, allocation happens only if an env variable with that segment exists (e.g., MYAPP_POINTER_FIELD_*)
- For pointer scalars, allocation happens when the env var is present (MYAPP_PSTR, etc.)

---

## Functional options (detailed)

### WithDefaultFn

Provide a factory that returns a *T. This is the starting point before file/env overrides.
```go
p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg {
    return &Cfg{
      Name: "default",
      Port: 8080,
    }
  }),
)
```

If you don’t pass WithDefaultFn, New uses a zero-value factory (var t T; return &t).

---

### WithPersistence

Enable loading/saving a config file under a user config directory.
```go
p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{Name: "default"} }),
  config.WithPersistence[Cfg]("myapp"), // -> ~/.config/myapp/config.yml (XDG-aware)
)
```

Behavior:
- If the file exists, it’s loaded
- If it doesn’t exist, it’s created with your default config (YAML by default)
- If you also set WithEnvPrefix("MYAPP") and define MYAPP_CONFIG_PATH, that path overrides persistence

---

### WithEnvPrefix

Set a prefix to turn on env overrides and to allow env-specified config path.
```go
p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{Port: 8080} }),
  config.WithEnvPrefix[Cfg]("MYAPP"),
)
// Env vars: MYAPP_PORT, MYAPP_NAME, ...
// Optional path override: MYAPP_CONFIG_PATH=/some/config.json
```


---

### WithStreams

Control where user-facing messages go:
- “created new config …”
- “loaded from …”
- non-fatal warnings

By default, **nothing** is printed. Use adapters from config/streams:

#### Discard everything (“silent”)
```go
import "github.com/ygrebnov/config/streams"

p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
  config.WithStreams(streams.Discard()),
)
```

#### Capture to buffers; flush later
```go
bs := streams.Buffers()

p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
  config.WithStreams(bs),
)

cfg, _, _, err := p.Get() // messages captured
if err != nil { /* handle */ }

// After Get(): print or inspect accumulated messages
out, errOut := bs.Strings()
fmt.Print(out)
fmt.Print(errOut)

// Reset if reusing:
bs.Reset()
```

#### Thread-safe buffers (for concurrent writers)
```go
ts := streams.ThreadSafeBuffers()

p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
  config.WithStreams(ts),
)
// after Get():
out, errOut := ts.Strings()
fmt.Print(out, errOut)
ts.Reset()
```

#### Send messages to a slog.Logger
```go
import (
  "log/slog"
  "os"
  "github.com/ygrebnov/config/streams"
)

logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
  config.WithStreams(streams.Slog(logger, slog.LevelInfo, slog.LevelError)),
)
```

#### Custom writers
```go
// Writers(out, err io.Writer) -> BasicIOStreams
var outBuf, errBuf bytes.Buffer

p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
  config.WithStreams(streams.Writers(&outBuf, &errBuf)),
)
```

---

## Defaults & Validation with github.com/ygrebnov/model

The config library can **optionally** integrate with the [model](github.com/ygrebnov/model) library to:
- Set defaults based on default tags (recursively, with dive)
- Validate based on validate tags (using built-in or custom rules)

### How it works (order of operations)

When you pass WithModel, Get() will:
1.	Create your *Cfg using WithDefaultFn
2.	**Call model.SetDefaults()** to fill zero values from tags
3.	Load from file (if any)
4.	Apply env overrides
5.	**Call model.Validate()**; if validation fails, Get() returns the error (you can errors.As it to *model.ValidationError)

### WithModel usage
```go
import modellib "github.com/ygrebnov/model"

type Cfg struct {
  Name string `yaml:"name" env:"NAME"    default:"svc"   validate:"nonempty"`
  Port int    `yaml:"port" env:"PORT"    default:"8080"  validate:"positive,nonzero"`
}

p := config.New[Cfg](
  config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
  config.WithEnvPrefix[Cfg]("MYAPP"),
  config.WithPersistence[Cfg]("myapp"),
  config.WithModel(func(c *Cfg) (*modellib.Model[Cfg], error) {
    return modellib.New(
      c,
      // Register builtin rules; add your custom rules as needed.
      modellib.WithRules[Cfg, string](modellib.BuiltinStringRules()),
      modellib.WithRules[Cfg, int](modellib.BuiltinIntRules()),
      // You can add other rule sets here (time.Duration, etc.)
    )
  }),
)

cfg, path, created, err := p.Get()
if err != nil {
  var ve *modellib.ValidationError
  if errors.As(err, &ve) {
    // Handle rich validation error; you can inspect field errors, marshal JSON, etc.
    fmt.Println("validation failed:", ve.Error())
  }
  log.Fatal(err)
}

fmt.Println("Final config:", *cfg, path, created)
```

Tip: model supports rich ValidationError with structured details (per-field), errors.Is/As, and JSON marshaling. See its README for advanced usage (custom rules, params, recursive validation with validateElem:"dive", etc.).

---

## Defaults: WithDefaultFn vs WithModel

When both `WithDefaultFn` and `WithModel` are used, the precedence for setting default values is as follows:

- The config loader first calls the factory function provided by `WithDefaultFn` to create the initial configuration struct with your specified defaults.
- Then, `model.SetDefaults()` is called, which applies default values from `default` tags **only to fields that are still zero-valued**.
- Therefore, if a field is set by the factory, it will **not** be overwritten by the `default` tag from the model.
- This means the factory's values take precedence over the model's default tags.

### Example

```go
package main

import (
	"fmt"
	"log"

	"github.com/ygrebnov/config"
	modellib "github.com/ygrebnov/model"
)

type Cfg struct {
	Name string `yaml:"name" env:"NAME" default:"model-default"`
	Port int    `yaml:"port" env:"PORT"`
}

func main() {
	p := config.New[Cfg](
		// Factory sets Port to 8080, but leaves Name zero-value
		config.WithDefaultFn(func() *Cfg {
			return &Cfg{
				Port: 8080,
			}
		}),
		config.WithModel(func(c *Cfg) (*modellib.Model[Cfg], error) {
			return modellib.New(
				c,
				modellib.WithRules[Cfg, string](modellib.BuiltinStringRules()),
				modellib.WithRules[Cfg, int](modellib.BuiltinIntRules()),
			)
		}),
	)

	cfg, _, _, err := p.Get()
	if err != nil {
		log.Fatal(err)
	}

	// Output:
	// Name is set by model default tag: "model-default"
	// Port is set by factory: 8080
	fmt.Printf("Name: %q\n", cfg.Name)
	fmt.Printf("Port: %d\n", cfg.Port)
}
```

### Rule of thumb

- Use `WithDefaultFn` when you want explicit programmatic defaults or complex initialization logic.
- Use model `default` tags with `WithModel` for declarative, tag-based defaults that apply only when fields are zero-valued.
- You can combine both, but remember factory defaults take precedence.

---

## A complete example (putting it all together)
```go
package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/ygrebnov/config"
	"github.com/ygrebnov/config/streams"
	modellib "github.com/ygrebnov/model"
)

type Cfg struct {
	Name     string        `yaml:"name" env:"NAME"        default:"svc"   validate:"nonempty"`
	Port     int           `yaml:"port" env:"PORT"        default:"8080"  validate:"positive,nonzero"`
	Timeout  time.Duration `yaml:"timeout" env:"TIMEOUT"  default:"5s"    validate:"positive"`
	ApiKey2FA string       `yaml:"api_key_2fa"            validate:"nonempty"`
}

func main() {
	// Route provider messages to slog (INFO/ERROR)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	p := config.New[Cfg](
		config.WithDefaultFn(func() *Cfg { return &Cfg{} }),
		config.WithPersistence[Cfg]("myapp"),
		config.WithEnvPrefix[Cfg]("MYAPP"),
		config.WithStreams(streams.Slog(logger, slog.LevelInfo, slog.LevelError)),

		config.WithModel(func(c *Cfg) (*modellib.Model[Cfg], error) {
			return modellib.New(
				c,
				modellib.WithRules[Cfg, string](modellib.BuiltinStringRules()),
				modellib.WithRules[Cfg, int](modellib.BuiltinIntRules()),
				modellib.WithRules[Cfg, time.Duration](modellib.BuiltinDurationRules()),
			)
		}),
	)

	cfg, path, created, err := p.Get()
	if err != nil {
		var ve *modellib.ValidationError
		if errors.As(err, &ve) {
			fmt.Println("Validation error:")
			fmt.Println(ve.Error())
		}
		log.Fatal(err)
	}

	fmt.Printf("Ready: %+v\n", *cfg)
	fmt.Println("Config path:", path, "Created:", created)
}
```

---

## Concurrency & Once semantics

- Provider.Get() is guarded with sync.Once: initialization runs **at most once**
- All subsequent Get() calls return the same *T, path, and fileCreated value
- Streams output for “created”/“loaded” messages is printed exactly once

---

## Error handling

You can detect error classes with errors.Is/errors.As:
- ErrEnsureConfigDir — we failed to create the config directory
- ErrUnsupportedConfigFileType — only .yaml, .yml, .json are supported
- ErrParse — file read/marshal failed (yaml/json unmarshal errors included)
- ErrFormat — file write/marshal failed (e.g., unsupported type; we guard against panic and wrap)
- ErrWrite — writing/renaming the temp file failed

With model enabled, validation errors come back as *model.ValidationError:
```go
if err != nil {
  var ve *model.ValidationError
  if errors.As(err, &ve) {
    // Inspect fields, marshal to JSON, etc.
  }
}
```

---

## FAQ

**Q: Do I need model to use this library?**
No. model is optional. If you don’t pass WithModel, Get() will skip defaults/validation based on tags and stick to your WithDefaultFn, file, and env.

**Q: What if I want a custom config path?**
Set WithEnvPrefix("MYAPP") and use MYAPP_CONFIG_PATH=/my/path/config.json. That path takes precedence over persistence.

**Q: YAML or JSON?**
Both are supported. On write, the extension decides the format. If we create a file under the user config dir, we write YAML (config.yml).

**Q: How do I silence all messages?**
Use config.WithStreams(streams.Discard()).

**Q: What happens if both WithDefaultFn and WithModel defaults set the same field?**
The value set by the factory function in `WithDefaultFn` takes precedence. The model's `default` tag is only applied to zero-valued fields after the factory runs, so if a field is already set by the factory, the model default will not overwrite it.

---

## License

Distributed under the MIT License. See the [LICENSE](LICENSE) file for details.