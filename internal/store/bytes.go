package store

import (
	"encoding/json"

	"github.com/ygrebnov/errorc"
	"gopkg.in/yaml.v3"

	"github.com/ygrebnov/config/pkg/errors"
	"github.com/ygrebnov/config/pkg/keys"
)

// GetJSON builds a hierarchy of settings based on the dot-separated keys and returns a JSON representation of the settings.
func (s *Store) GetJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h := s.buildHierarchy()

	if len(h) == 0 {
		return []byte("{}"), nil // return a valid JSON.
	}

	j, err := json.Marshal(h)
	if err != nil {
		return nil, errorc.With(errors.ErrMarshalingError, errorc.Error(keys.Cause, err))
	}

	return j, nil
}

// GetYAML builds a hierarchy of settings based on the dot-separated keys and returns a YAML representation of the settings.
func (s *Store) GetYAML() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h := s.buildHierarchy()

	if len(h) == 0 {
		return []byte("{}\n"), nil
	}

	y, err := yaml.Marshal(h)
	if err != nil {
		return nil, errorc.With(errors.ErrMarshalingError, errorc.Error(keys.Cause, err))
	}

	return y, nil
}

// FromBytes parses the given slice of bytes and loads settings to the store.
func (s *Store) FromBytes(b []byte) error {
	m := make(map[string]any)
	err := yaml.Unmarshal(b, &m)
	if err != nil {
		return errorc.With(errors.ErrParsingError, errorc.Error(keys.Cause, err))
	}

	s.fromMap("", m)

	return nil
}

func (s *Store) fromMap(prefix string, m map[string]any) {
	for k, v := range m {
		name := k
		if prefix != "" {
			name = prefix + "." + k
		}

		if v == nil {
			s.Set(name, nil)
			continue
		}

		switch vv := v.(type) {
		case map[string]any:
			s.fromMap(name, vv)
		// TODO: hande slice of maps case
		default:
			s.Set(name, v)
		}
	}
}
