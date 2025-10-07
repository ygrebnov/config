// filepath: /Users/yaroslavgrebnov/go/src/config/env_apply.go
package config

import (
	"reflect"

	pip "github.com/ygrebnov/config/pipeline"
)

func init() {
	pip.SetEnvApply(func(target any, prefix string, strategy pip.SetStrategy) {
		if target == nil {
			return
		}
		rv := reflect.ValueOf(target)
		if !rv.IsValid() {
			return
		}
		applyEnvWithMeta(rv, prefix, SetStrategy(strategy))
	})
}
