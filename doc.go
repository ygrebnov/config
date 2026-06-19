// Package config provides a small configuration library for Go applications.
//
// The public API is centered on two entry points:
//
//	Load(ctx, &cfg, opts...)
//
// for one-shot initialization, and
//
//	controller := NewController[Cfg](opts...)
//	controller.Load(ctx, &cfg)
//	controller.Get("db.host")
//	controller.Set("db.host", "localhost")
//	controller.Save(ctx)
//
// for interactive read/update/save flows.
//
// Load is safe for concurrent use with the same target pointer and initializes
// that pointer at most once. Later calls with the same pointer reuse the first
// completed result. Use Controller when you want an explicit mutable lifecycle
// and persistence owner for a config instance.
//
// Loading order is deterministic:
//  1. start from the target's current state
//  2. apply github.com/ygrebnov/model defaults, if WithModel is enabled
//  3. read YAML/JSON config from an explicit or well-known path, if configured
//  4. apply environment overrides
//  5. validate via github.com/ygrebnov/model, if WithModel is enabled
//
// Supported options include WithPath, WithAppName, WithEnvPrefix, and WithStreams.
package config
