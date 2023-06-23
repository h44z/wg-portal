package model

import (
	"github.com/h44z/wg-portal/internal"
	"github.com/h44z/wg-portal/internal/domain"
)

type StringConfigOption struct {
	Value       string `json:"Value"`
	Overridable bool   `json:"Overridable"`
}

func NewStringConfigOption(value string, overridable bool) StringConfigOption {
	return StringConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

func StringConfigOptionFromDomain(opt domain.StringConfigOption) StringConfigOption {
	return StringConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

func StringConfigOptionToDomain(opt StringConfigOption) domain.StringConfigOption {
	return domain.StringConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

type StringSliceConfigOption struct {
	Value       []string `json:"Value"`
	Overridable bool     `json:"Overridable"`
}

func NewStringSliceConfigOption(value []string, overridable bool) StringSliceConfigOption {
	return StringSliceConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

func StringSliceConfigOptionFromDomain(opt domain.StringConfigOption) StringSliceConfigOption {
	return StringSliceConfigOption{
		Value:       internal.SliceString(opt.Value),
		Overridable: opt.Overridable,
	}
}

func StringSliceConfigOptionToDomain(opt StringSliceConfigOption) domain.StringConfigOption {
	return domain.StringConfigOption{
		Value:       internal.SliceToString(opt.Value),
		Overridable: opt.Overridable,
	}
}

type IntConfigOption struct {
	Value       int  `json:"Value"`
	Overridable bool `json:"Overridable"`
}

func NewIntConfigOption(value int, overridable bool) IntConfigOption {
	return IntConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

func IntConfigOptionFromDomain(opt domain.IntConfigOption) IntConfigOption {
	return IntConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

func IntConfigOptionToDomain(opt IntConfigOption) domain.IntConfigOption {
	return domain.IntConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

type Int32ConfigOption struct {
	Value       int32 `json:"Value"`
	Overridable bool  `json:"Overridable"`
}

func NewInt32ConfigOption(value int32, overridable bool) Int32ConfigOption {
	return Int32ConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

func Int32ConfigOptionFromDomain(opt domain.Int32ConfigOption) Int32ConfigOption {
	return Int32ConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

func Int32ConfigOptionToDomain(opt Int32ConfigOption) domain.Int32ConfigOption {
	return domain.Int32ConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

type BoolConfigOption struct {
	Value       bool `json:"Value"`
	Overridable bool `json:"Overridable"`
}

func NewBoolConfigOption(value bool, overridable bool) BoolConfigOption {
	return BoolConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

func BoolConfigOptionFromDomain(opt domain.BoolConfigOption) BoolConfigOption {
	return BoolConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}

func BoolConfigOptionToDomain(opt BoolConfigOption) domain.BoolConfigOption {
	return domain.BoolConfigOption{
		Value:       opt.Value,
		Overridable: opt.Overridable,
	}
}
