package config

import (
	"context"
	"reflect"
	"sync"

	modellib "github.com/ygrebnov/model"
)

type modelBinding[T any] interface {
	ApplyDefaults(*T) error
	Validate(context.Context, *T) error
}

var modelBindingCache sync.Map

func getModelBinding[T any]() (modelBinding[T], error) {
	var zero *T
	typ := reflect.TypeOf(zero).Elem()
	if cached, ok := modelBindingCache.Load(typ); ok {
		return cached.(modelBinding[T]), nil
	}

	binding, err := modellib.NewBinding[T]()
	if err != nil {
		return nil, err
	}

	actual, _ := modelBindingCache.LoadOrStore(typ, binding)
	return actual.(modelBinding[T]), nil
}

