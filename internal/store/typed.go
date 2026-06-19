package store

import (
	"reflect"

	"github.com/ygrebnov/errorc"

	"github.com/ygrebnov/config/pkg/errors"
	"github.com/ygrebnov/config/pkg/keys"
)

func (s *Store) GetString(key string) (string, error) {
	v, ok := s.Get(key)
	if !ok {
		return "", errorc.With(errors.ErrSettingNotFound, errorc.String(keys.SettingName, key))
	}

	// if value underlying type is string, convert it to string.
	t := reflect.TypeOf(v)
	if t == nil {
		return "", errorc.With(
			errors.ErrSettingTypeMismatch,
			errorc.String(keys.SettingName, key),
			errorc.String(keys.SettingType, "nil"),
			errorc.String(keys.SettingRequestedType, "string"),
		)
	}

	if t.Kind() != reflect.String {
		return "", errorc.With(
			errors.ErrSettingTypeMismatch,
			errorc.String(keys.SettingName, key),
			errorc.String(keys.SettingType, t.Kind().String()),
			errorc.String(keys.SettingRequestedType, "string"),
		)
	}

	return t.String(), nil
}
