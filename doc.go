// Package config provides a small, opinionated configuration loader for Go applications.
//
// It supports:
//  1. Constructing a config instance via a user-provided default factory.
//  2. Loading overrides from YAML/JSON files (optionally persisted under a user
//     config directory).
//  3. Applying environment variable overrides using `env` tags or field names
//     converted to SCREAMING_SNAKE_CASE.
//  4. Optional integration with github.com/ygrebnov/model for struct defaults
//     (via `default` tags) and validation (via `validate` tags).
//
// Typical usage:
//
//	p := config.New[Cfg](
//	    config.WithPersistence[Cfg]("myapp"),
//	    config.WithEnvPrefix[Cfg]("MYAPP"),
//	    config.WithDefaultFn(func() *Cfg { return &Cfg{ Name: "default" } }),
//	)
//	cfg, path, created, err := p.Get()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	_ = cfg; _ = path; _ = created
package config
