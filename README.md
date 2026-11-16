# config

`config` loads a Go struct from YAML or JSON, model defaults, and environment
variables. It also provides a controller for inspecting, updating, validating,
and persisting configuration.

## Installation

```sh
go get github.com/ygrebnov/config
```

## One-shot loading

Use `Load` when the application only needs the completed configuration object.

```go
type Config struct {
    Name string `yaml:"name" default:"api" env:"NAME"`
    Port int    `yaml:"port" default:"8080" env:"PORT" validate:"min(1)"`
}

var cfg Config
if err := config.Load(ctx, &cfg, config.WithPath("config.yaml")); err != nil {
    return err
}
```

For a configured source, values are read from the file first. Model defaults
fill zero-valued fields, environment values override them, and the completed
object is validated.

## Controller lifecycle

Use `Controller` for explicit read/update/validate/save flows. Controller paths
are exact exported model paths, not YAML or JSON field names.

```go
controller, err := config.NewController[Config](config.WithPath("config.yaml"))
if err != nil {
    return err
}

controller.Set("Port", 9090)
if err := controller.Validate(ctx); err != nil {
    return err
}
if err := controller.Save(ctx); err != nil {
    return err
}

port, err := controller.Get("Port")
if err != nil {
    return err
}
fmt.Println(port)
```

`Set` does not validate immediately, allowing a batch of related updates.
Call `Validate` for immediate feedback. `Save` always validates before writing.
`Get` returns `ErrConfigurationOptionNotFound` for a missing path.
The controller copies maps, slices, pointers, and their exported nested fields
when values enter or leave its store. Mutating a value after `Set`, `Get`, or
`Load` does not change the controller's state. Channels, functions, and
unexported reference fields retain Go's ordinary copy semantics.

Common paths include:

- `Name`
- `Server.Host`
- `Items[]`

Call `Load(ctx, &cfg)` on a controller to copy its current state into a typed
configuration object. Concurrent loads must use separate destination objects or
caller-side synchronization.

## Sources and paths

Configuration file resolution uses this precedence:

1. `WithPath(path)`
2. `${PREFIX}_CONFIG_PATH` with `WithEnvPrefix("PREFIX")`
3. `WithAppName("myapp")`, which resolves to
   `$(XDG_CONFIG_HOME | os.UserConfigDir())/myapp/config.yml`

`WithAppName` accepts one application name, without path separators, reserved
device names, or `..`. Its config directory must be inside the user's home;
`XDG_CONFIG_HOME` must be absolute and inside that home. Use `WithPath` or
`WithSavePath` for an intentional location elsewhere. Explicit and
`${PREFIX}_CONFIG_PATH` paths use their parent directory as the filesystem
boundary.

Missing files are allowed, so an application can start from defaults and
environment values and `Save` can create the file. Custom paths supplied through
`WithPath`, `${PREFIX}_CONFIG_PATH`, or `WithSavePath` must have an existing
parent directory; otherwise they return `ErrConfigurationDirectoryNotFound`.
`WithAppName` may create its private app directory during `Save`. Existing
config files must be regular and have private permissions. Saved files are
created with private permissions; `Save` rejects a symlink in place of an
existing destination file.

YAML (`.yaml`, `.yml`) and JSON (`.json`) files are supported.

## Environment variables

`WithEnvPrefix("MYAPP")` enables environment overrides and the optional
`MYAPP_CONFIG_PATH` file location. Field-level `env` tags control names; model
otherwise derives names from exported fields.

```go
type Config struct {
    Port int `env:"HTTP_PORT"`
}

var cfg Config

// Reads MYAPP_HTTP_PORT.
config.Load(ctx, &cfg, config.WithEnvPrefix("MYAPP"))
```

## Custom validation rules

Pass rules created with `github.com/ygrebnov/model` to
`WithValidationRules`. They run during controller construction, `Validate`, and
`Load`; `Save` also runs validation before persistence.

```go
type Config struct {
    Environment string `default:"production" validate:"environment"`
}

environment, err := model.NewRule[string](
    "environment",
    func(value string, _ ...string) error {
        if value != "production" && value != "development" {
            return fmt.Errorf("unsupported environment %q", value)
        }
        return nil
    },
)
if err != nil {
    return err
}

controller, err := config.NewController[Config](
    config.WithValidationRules(environment),
)
```

## Runnable examples

See [`examples/examples_test.go`](examples/examples_test.go) for complete,
runnable examples covering one-shot loading, controller lifecycle operations,
and custom validation rules.

## License

Distributed under the BSD 3-Clause License. See [LICENSE](LICENSE).
