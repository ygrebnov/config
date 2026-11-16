package store

import (
	"reflect"
	"sync"
)

type Store struct {
	mu sync.RWMutex
	kv map[string]any
}

func New() *Store {
	return &Store{kv: make(map[string]any)}
}

func (s *Store) Set(key string, value any) {
	value = clone(value)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = value
}

func (s *Store) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if v, ok := s.kv[key]; ok {
		return clone(v), ok
	}
	return nil, false
}

type visit struct {
	typ reflect.Type
	ptr uintptr
	len int
	cap int
}

// clone separates reference values crossing the store boundary. Struct fields
// that reflection cannot set and opaque values such as channels and functions
// retain their ordinary Go copy semantics.
func clone(value any) any {
	if value == nil {
		return nil
	}
	switch value.(type) {
	case bool, string,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, uintptr,
		float32, float64, complex64, complex128:
		return value
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return value
	}
	return cloneValue(v, make(map[visit]reflect.Value)).Interface()
}

func cloneValue(v reflect.Value, seen map[visit]reflect.Value) reflect.Value {
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type()).Elem()
		out.Set(cloneValue(v.Elem(), seen))
		return out
	case reflect.Pointer:
		if v.IsNil() {
			return v
		}
		id := visit{typ: v.Type(), ptr: v.Pointer()}
		if prior, ok := seen[id]; ok {
			return prior
		}
		out := reflect.New(v.Type().Elem())
		seen[id] = out
		out.Elem().Set(cloneValue(v.Elem(), seen))
		return out
	case reflect.Map:
		if v.IsNil() {
			return v
		}
		id := visit{typ: v.Type(), ptr: v.Pointer()}
		if prior, ok := seen[id]; ok {
			return prior
		}
		out := reflect.MakeMapWithSize(v.Type(), v.Len())
		seen[id] = out
		iter := v.MapRange()
		for iter.Next() {
			out.SetMapIndex(cloneValue(iter.Key(), seen), cloneValue(iter.Value(), seen))
		}
		return out
	case reflect.Slice:
		if v.IsNil() {
			return v
		}
		id := visit{typ: v.Type(), ptr: v.Pointer(), len: v.Len(), cap: v.Cap()}
		if prior, ok := seen[id]; ok {
			return prior
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		seen[id] = out
		for i := range v.Len() {
			out.Index(i).Set(cloneValue(v.Index(i), seen))
		}
		return out
	case reflect.Array:
		out := reflect.New(v.Type()).Elem()
		for i := range v.Len() {
			out.Index(i).Set(cloneValue(v.Index(i), seen))
		}
		return out
	case reflect.Struct:
		out := reflect.New(v.Type()).Elem()
		out.Set(v)
		for i := range v.NumField() {
			if out.Field(i).CanSet() && v.Field(i).CanInterface() {
				out.Field(i).Set(cloneValue(v.Field(i), seen))
			}
		}
		return out
	default:
		return v
	}
}
