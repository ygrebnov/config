package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ygrebnov/errorc"

	configerrors "github.com/ygrebnov/config/pkg/errors"
	configkeys "github.com/ygrebnov/config/pkg/keys"
)

type accessField struct {
	path  string
	steps []int
	typ   reflect.Type
}

type accessMeta struct {
	byPath map[string]accessField
}

var accessMetaCache sync.Map

func getAccessMeta(target any) (*accessMeta, error) {
	if err := validateTarget(target); err != nil {
		return nil, err
	}

	t := reflect.TypeOf(target)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if cached, ok := accessMetaCache.Load(t); ok {
		return cached.(*accessMeta), nil
	}

	meta := buildAccessMeta(t)
	actual, _ := accessMetaCache.LoadOrStore(t, meta)
	return actual.(*accessMeta), nil
}

func buildAccessMeta(t reflect.Type) *accessMeta {
	meta := &accessMeta{byPath: make(map[string]accessField)}
	var walk func(rt reflect.Type, prefix string, steps []int)
	walk = func(rt reflect.Type, prefix string, steps []int) {
		if rt.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < rt.NumField(); i++ {
			sf := rt.Field(i)
			if sf.PkgPath != "" {
				continue
			}
			segment, ok := optionSegment(sf)
			if !ok {
				continue
			}
			path := joinOptionPath(prefix, segment)
			fieldSteps := append(append([]int(nil), steps...), i)
			if _, exists := meta.byPath[path]; !exists {
				meta.byPath[path] = accessField{path: path, steps: fieldSteps, typ: sf.Type}
			}
			ft := sf.Type
			switch {
			case ft == reflect.TypeOf(time.Duration(0)):
				continue
			case ft.Kind() == reflect.Struct:
				walk(ft, path, fieldSteps)
			case ft.Kind() == reflect.Pointer && ft.Elem().Kind() == reflect.Struct:
				walk(ft.Elem(), path, fieldSteps)
			}
		}
	}
	walk(t, "", nil)
	return meta
}

func optionSegment(sf reflect.StructField) (string, bool) {
	for _, tagName := range []string{"yaml", "json"} {
		tag := strings.TrimSpace(sf.Tag.Get(tagName))
		if tag == "-" {
			return "", false
		}
		if tag == "" {
			continue
		}
		name := strings.TrimSpace(strings.Split(tag, ",")[0])
		if name != "" {
			return name, true
		}
	}
	return sf.Name, true
}

func joinOptionPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func (m *accessMeta) get(target any, name string) (any, error) {
	field, ok := m.byPath[name]
	if !ok {
		return nil, optionNotFound(name)
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	value, ok := navigateField(rv, field.steps, false)
	if !ok {
		return nil, nil
	}
	return unwrapValue(value), nil
}

func (m *accessMeta) set(target any, name string, value any) error {
	field, ok := m.byPath[name]
	if !ok {
		return optionNotFound(name)
	}

	rv := reflect.ValueOf(target)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	fv, ok := navigateField(rv, field.steps, true)
	if !ok {
		return errorc.With(configerrors.ErrOptionNotSettable, errorc.String(configkeys.OptionPath, name))
	}
	if err := assignValue(fv, value); err != nil {
		return errorc.With(err, errorc.String(configkeys.OptionPath, name))
	}
	return nil
}

func navigateField(root reflect.Value, steps []int, create bool) (reflect.Value, bool) {
	current := root
	for i, idx := range steps {
		field := current.Field(idx)
		if i == len(steps)-1 {
			return field, true
		}
		switch field.Kind() {
		case reflect.Pointer:
			if field.IsNil() {
				if !create {
					return reflect.Value{}, false
				}
				field.Set(reflect.New(field.Type().Elem()))
			}
			current = field.Elem()
		case reflect.Struct:
			current = field
		default:
			return reflect.Value{}, false
		}
	}
	return current, true
}

func unwrapValue(v reflect.Value) any {
	for v.IsValid() && v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return nil
	}
	return v.Interface()
}

func assignValue(dst reflect.Value, value any) error {
	if !dst.IsValid() || !dst.CanSet() {
		return configerrors.ErrOptionNotSettable
	}

	if value == nil {
		switch dst.Kind() {
		case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface:
			dst.SetZero()
			return nil
		default:
			return configerrors.ErrInvalidOptionValue
		}
	}

	if dst.Kind() == reflect.Pointer {
		if dst.IsNil() {
			dst.Set(reflect.New(dst.Type().Elem()))
		}
		return assignValue(dst.Elem(), value)
	}

	input := reflect.ValueOf(value)
	if input.IsValid() {
		if input.Type().AssignableTo(dst.Type()) {
			dst.Set(input)
			return nil
		}
		if input.Type().ConvertibleTo(dst.Type()) && input.Kind() != reflect.String {
			dst.Set(input.Convert(dst.Type()))
			return nil
		}
	}

	if dst.Type() == reflect.TypeOf(time.Duration(0)) {
		duration, err := toDuration(value)
		if err != nil {
			return err
		}
		dst.SetInt(int64(duration))
		return nil
	}

	switch dst.Kind() {
	case reflect.String:
		dst.SetString(fmt.Sprint(value))
		return nil
	case reflect.Bool:
		b, err := toBool(value)
		if err != nil {
			return err
		}
		dst.SetBool(b)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := toInt64(value)
		if err != nil {
			return err
		}
		dst.SetInt(n)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := toUint64(value)
		if err != nil {
			return err
		}
		dst.SetUint(n)
		return nil
	case reflect.Float32, reflect.Float64:
		n, err := toFloat64(value)
		if err != nil {
			return err
		}
		dst.SetFloat(n)
		return nil
	default:
		// Fall through to the JSON-based assignment path below for composite kinds.
	}

	if dst.CanAddr() {
		payload, err := json.Marshal(value)
		if err != nil {
			return invalidOptionValueError(value, err)
		}
		if err := json.Unmarshal(payload, dst.Addr().Interface()); err != nil {
			return invalidOptionValueError(value, err)
		}
		return nil
	}

	return invalidOptionValueError(value, nil)
}

func toBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return false, invalidOptionValueError(value, err)
		}
		return parsed, nil
	default:
		return false, invalidOptionValueError(value, nil)
	}
}

func toDuration(value any) (time.Duration, error) {
	switch v := value.(type) {
	case time.Duration:
		return v, nil
	case string:
		parsed, err := time.ParseDuration(strings.TrimSpace(v))
		if err != nil {
			return 0, invalidOptionValueError(value, err)
		}
		return parsed, nil
	default:
		n, err := toInt64(value)
		if err != nil {
			return 0, err
		}
		return time.Duration(n), nil
	}
}

func toInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
	case float32:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, invalidOptionValueError(value, err)
		}
		return parsed, nil
	default:
		return 0, invalidOptionValueError(value, nil)
	}
}

func toUint64(value any) (uint64, error) {
	switch v := value.(type) {
	case int:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case int8:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case int16:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case int32:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case int64:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case uint:
		return uint64(v), nil
	case uint8:
		return uint64(v), nil
	case uint16:
		return uint64(v), nil
	case uint32:
		return uint64(v), nil
	case uint64:
		return v, nil
	case float32:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case float64:
		if v < 0 {
			return 0, invalidOptionValueError(value, nil)
		}
		return uint64(v), nil
	case string:
		parsed, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, invalidOptionValueError(value, err)
		}
		return parsed, nil
	default:
		return 0, invalidOptionValueError(value, nil)
	}
}

func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float32:
		return float64(v), nil
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, invalidOptionValueError(value, err)
		}
		return parsed, nil
	default:
		return 0, invalidOptionValueError(value, nil)
	}
}

func invalidOptionValueError(value any, cause error) error {
	if value == nil {
		return configerrors.ErrInvalidOptionValue
	}
	return errorc.With(
		configerrors.ErrInvalidOptionValue,
		errorc.String(configkeys.ValueType, reflect.TypeOf(value).String()),
		errorc.Error(configkeys.Cause, cause),
	)
}

