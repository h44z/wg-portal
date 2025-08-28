package model

import (
	"github.com/fedor-git/wg-portal-2/internal"
	"github.com/fedor-git/wg-portal-2/internal/domain"
)

type ConfigOption[T any] struct {
	Value       T    `json:"Value"`
	Overridable bool `json:"Overridable"`
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
