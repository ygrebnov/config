package config

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"

	"github.com/ygrebnov/errorc"
	modelvalidation "github.com/ygrebnov/model/validation"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
)

type loadResult struct {
	path       string
	fileLoaded bool
}

type loadState struct {
	once   sync.Once
	result loadResult
	err    error
}

var loadStateCache sync.Map

// Load populates dst using its current state, optional model defaults, file data, environment overrides, and optional model validation.
//
// Semantics:
//   - Load is safe for concurrent use with the same target pointer.
//   - For a given target pointer, initialization runs at most once; later calls reuse the first completed result.
//   - Because the target pointer owns that initialization state, callers should not expect later calls with different
//     options to reinitialize the same target. To manage a dedicated lifecycle explicitly, use Controller.
func Load[T any](ctx context.Context, dst *T, opts ...Option[T]) error {
	cfg := applyOptions(opts...)
	if err := validateSettings(cfg); err != nil {
		return err
	}

	state, err := getLoadState(dst)
	if err != nil {
		return err
	}

	state.once.Do(func() {
		state.result, state.err = loadInto(ctx, dst, cfg)
	})

	return state.err
}

func loadInto[T any](ctx context.Context, dst *T, cfg settings[T]) (loadResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := validateTarget(dst); err != nil {
		return loadResult{}, err
	}

	var binding modelBinding[T]
	if cfg.useModelBinding {
		var err error
		binding, err = getModelBinding[T]()
		if err != nil {
			return loadResult{}, errorc.With(err, errorc.String(configkeys.Phase, "model.binding"))
		}
		if err := binding.ApplyDefaults(dst); err != nil {
			return loadResult{}, errorc.With(err, errorc.String(configkeys.Phase, "model.defaults"))
		}
	}

	path, err := resolveConfigPath(cfg)
	if err != nil {
		return loadResult{}, err
	}

	result := loadResult{path: path}
	if path != "" {
		err = loadFromFile(path, dst)
		switch {
		case err == nil:
			result.fileLoaded = true
			writeOut(cfg.streams, "config: loaded from %s\n", path)
		case errors.Is(err, os.ErrNotExist):
			// Missing config file is not an error for Load; defaults/env still apply.
		default:
			return loadResult{}, err
		}
	}

	applyEnvToTarget(dst, cfg.envPrefix, cfg.envSetStrategy)

	if binding != nil {
		if err := binding.Validate(ctx, dst); err != nil {
			return loadResult{}, reduceValidationError(err, cfg.validationStrategy)
		}
	}

	return result, nil
}


func validateTarget(target any) error {
	if target == nil {
		return configerrors.ErrNilTarget
	}

	rt := reflect.TypeOf(target)
	if rt == nil || rt.Kind() != reflect.Pointer || rt.Elem().Kind() != reflect.Struct {
		typeName := "<nil>"
		if rt != nil {
			typeName = rt.String()
		}
		return errorc.With(
			configerrors.ErrNotStructPointer,
			errorc.String(configkeys.TargetType, typeName),
		)
	}

	rv := reflect.ValueOf(target)
	if rv.IsNil() {
		return configerrors.ErrNilTarget
	}

	return nil
}

func getLoadState(target any) (*loadState, error) {
	if err := validateTarget(target); err != nil {
		return nil, err
	}

	key := reflect.ValueOf(target).Pointer()
	actual, _ := loadStateCache.LoadOrStore(key, &loadState{})
	return actual.(*loadState), nil
}

func reduceValidationError(err error, strategy ValidationStrategy) error {
	if err == nil || strategy == ValidateAllErrors {
		return err
	}

	var ve *modelvalidation.Error
	if !errors.As(err, &ve) || ve == nil || ve.Len() <= 1 {
		return err
	}

	fields := ve.Fields()
	if len(fields) == 0 {
		return err
	}

	issues := ve.ForField(fields[0])
	if len(issues) == 0 {
		return err
	}

	first := &modelvalidation.Error{}
	first.Add(issues[0])
	return first
}
