package config

import (
	"context"
	stdErrors "errors"
	"strings"
	"testing"

	"github.com/ygrebnov/model/field"

	configerrors "github.com/ygrebnov/config/pkg/errors"
)

type failingValueSink struct {
	err error
}

func (s failingValueSink) Set(string, any) error {
	return s.err
}

var _ field.ValueSink = failingValueSink{}

func TestNewControllerCtx_ReturnsWriteValuesError(t *testing.T) {
	sinkErr := stdErrors.New("sink failure")

	_, err := newControllerCtx[struct{ Name string }](
		context.Background(),
		func(storeService) field.ValueSink {
			return failingValueSink{err: sinkErr}
		},
	)
	if !stdErrors.Is(err, configerrors.ErrCannotInitializeConfigurationObject) {
		t.Fatalf(
			"newControllerCtx() error = %v, want ErrCannotInitializeConfigurationObject",
			err,
		)
	}
	if !strings.Contains(err.Error(), sinkErr.Error()) {
		t.Fatalf("newControllerCtx() error = %v, want sink error", err)
	}
}
