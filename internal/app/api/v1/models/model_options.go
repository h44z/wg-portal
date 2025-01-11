package models

import (
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
)

type ConfigOption[T any] struct {
	Value       T    `json:"Value"`
	Overridable bool `json:"Overridable,omitempty"`
}

func NewConfigOption[T any](value T, overridable bool) ConfigOption[T] {
	return ConfigOption[T]{
		Value:       value,
		Overridable: overridable,
	}
}

func ConfigOptionFromDomain[T any](opt domain.ConfigOption[T]) ConfigOption[T] {
	return ConfigOption[T]{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

func ConfigOptionToDomain[T any](opt ConfigOption[T]) domain.ConfigOption[T] {
	return domain.ConfigOption[T]{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

func StringSliceConfigOptionFromDomain(opt domain.ConfigOption[string]) ConfigOption[[]string] {
	return ConfigOption[[]string]{
		Value:       internal.SliceString(opt.Value),
		Overridable: opt.Overridable,
	}
}

func StringSliceConfigOptionToDomain(opt ConfigOption[[]string]) domain.ConfigOption[string] {
	return domain.ConfigOption[string]{
		Value:       internal.SliceToString(opt.Value),
		Overridable: opt.Overridable,
	}
}
