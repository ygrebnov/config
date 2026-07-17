# config

`config` is a small, opinionated configuration loader for Go.

The package has two primary entry points:

- `Load(ctx, &cfg, opts...)` for one-shot initialization
- `NewController[Cfg](opts...)` for read/update/save flows

## Quick start

### One-shot load

```go
type Config struct {
    Name string `yaml:"name" env:"NAME" default:"svc" validate:"min(1)"`
    Port int    `yaml:"port" env:"PORT" default:"8080" validate:"min(1),nonzero"`
}

var cfg Config
err := config.Load(ctx, &cfg,
    config.WithAppName[Config]("myapp"),
    config.WithEnvPrefix[Config]("MYAPP"),
    config.WithModel[Config](),
)
if err != nil {
    return err
}
```

If you want custom initial values, set them on `cfg` before calling `Load`.

### Interactive load / mutate / save

```go
controller := config.NewController[Config](
    config.WithPath[Config]("./config.yaml"),
)

var cfg Config
if err := controller.Load(ctx, &cfg); err != nil {
    return err
}

controller.Set("Port", 9090)

if err := controller.Save(ctx); err != nil {
    return err
}
```

## API overview

### `Load`

`Load` populates a target struct from:

1. the target's current state
2. model defaults from `github.com/ygrebnov/model` tags, if `WithModel()` is enabled
3. a YAML/JSON file, if one is configured or discovered
4. environment variables
5. model validation, if `WithModel()` is enabled

`Load` is safe for concurrent use with the same target pointer. A given target
pointer is initialized at most once; later calls with that same pointer reuse the
first completed result.

### `Controller`

`Controller` owns an explicit config lifecycle for a single target instance.
It supports:

- `Load(ctx, &cfg)`
- `Validate(ctx)`
- `Get("DB.Host")`
- `Set("DB.Host", value)`
- `Save(ctx)`

Use `Controller` when you want mutable configuration with explicit persistence,
rather than one-shot initialization.

## Options

Supported options are:

- `WithPath[T](path)`
  - explicit config file path
  - highest precedence for file resolution
- `WithAppName[T](name)`
  - resolves a well-known config path under the user config directory
- `WithEnvPrefix[T](prefix)`
  - enables environment overrides
  - also enables `${PREFIX}_CONFIG_PATH`
- `WithValidationRules(rules...)`
  - registers custom `github.com/ygrebnov/model` validation rules
- `WithModel[T]()`
  - enables cached `github.com/ygrebnov/model` binding-based defaults and validation
- `WithEnvSetStrategy[T](strategy)`
  - controls how environment values are applied
  - default: `SetOverride`
- `WithValidationStrategy[T](strategy)`
  - controls how validation errors are surfaced
  - default: `ValidateAllErrors`
- `WithStreams[T](streams)`
  - writes user-facing load/save messages to provided streams

## Option validation

Option constructors do not panic.

Invalid option values are reported when the configuration is used:

- `Load(...)`
- `Controller.Load(...)`

Examples:

- `WithPath("")` -> `ErrEmptyPath`
- `WithAppName("")` -> `ErrEmptyAppName`
- `WithEnvPrefix("")` -> `ErrEmptyEnvPrefix`

## Path resolution

Config path resolution order is:

1. `WithPath`
2. `${PREFIX}_CONFIG_PATH` when `WithEnvPrefix` is set
3. `WithAppName` -> `$(XDG_CONFIG_HOME | os.UserConfigDir())/<app>/config.yml`

Missing config files are not an error for `Load`; the target can still be
initialized from its current state, model defaults, and environment variables.

## Environment mapping

Environment overrides use `env` tags when present. Otherwise field names are
converted to `SCREAMING_SNAKE_CASE`.

Examples:

- `Name` -> `NAME`
- `ApiKey2FA` -> `API_KEY2FA`
- with prefix `MYAPP`: `MYAPP_NAME`, `MYAPP_API_KEY2FA`

`WithEnvSetStrategy` controls how env values are applied:

- `SetOverride` replaces existing values
- `SetFillZero` only fills zero-valued fields

## Model integration

`WithModel()` uses a cached `github.com/ygrebnov/model` binding for the target
type. It applies `default` tags before file/env loading and validates after all
overrides have been applied.

`WithValidationStrategy(ValidateFirstError)` reduces a model validation error to
the first issue when multiple field issues are present.

## Controller option paths

`Controller.Get` and `Controller.Set` use exact exported model paths. `Get`
returns `ErrConfigurationOptionNotFound` for an unknown path.

`Set` does not validate an updated value. Call `Validate(ctx)` after a batch of
updates for immediate feedback; `Save(ctx)` validates before writing the file.

Examples:

- `Name`
- `Port`
- `DB.Host`
- `RateLimit.MaxConn`

## Contributing

Contributions are welcome!
Please open an [issue](https://github.com/ygrebnov/config/issues) or submit a [pull request](https://github.com/ygrebnov/config/pulls).

## License

Distributed under the BSD 3-Clause License. See the [LICENSE](LICENSE) file for details.
