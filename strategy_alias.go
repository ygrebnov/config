package config

import pip "github.com/ygrebnov/config/pipeline"

type SetStrategy = pip.SetStrategy

const (
	SetOverride SetStrategy = pip.SetOverride
	SetFillZero SetStrategy = pip.SetFillZero
)

type ValidationStrategy = pip.ValidationStrategy

const (
	ValidateAllErrors  ValidationStrategy = pip.ValidateAllErrors
	ValidateFirstError ValidationStrategy = pip.ValidateFirstError
)
