package persistence

// ConfigOption is an Overridable configuration option
type ConfigOption struct {
	Value       interface{}
	Overridable bool
}

type StringConfigOption struct {
	ConfigOption
}

func (o StringConfigOption) GetValue() string {
	if o.Value == nil {
		return ""
	}
	return o.Value.(string)
}

func (o *StringConfigOption) SetValue(value string) {
	o.Value = value
}

func (o *StringConfigOption) TrySetValue(value string) bool {
	if o.Overridable {
		o.Value = value
		return true
	}
	return false
}

func NewStringConfigOption(value string, overridable bool) StringConfigOption {
	return StringConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type IntConfigOption struct {
	ConfigOption
}

func (o IntConfigOption) GetValue() int {
	if o.Value == nil {
		return 0
	}
	return o.Value.(int)
}

func (o *IntConfigOption) SetValue(value int) {
	o.Value = value
}

func (o *IntConfigOption) TrySetValue(value int) bool {
	if o.Overridable {
		o.Value = value
		return true
	}
	return false
}

func NewIntConfigOption(value int, overridable bool) IntConfigOption {
	return IntConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type Int32ConfigOption struct {
	ConfigOption
}

func (o Int32ConfigOption) GetValue() int32 {
	if o.Value == nil {
		return 0
	}

	return o.Value.(int32)
}

func (o *Int32ConfigOption) SetValue(value int32) {
	o.Value = value
}

func (o *Int32ConfigOption) TrySetValue(value int32) bool {
	if o.Overridable {
		o.Value = value
		return true
	}
	return false
}

func NewInt32ConfigOption(value int32, overridable bool) Int32ConfigOption {
	return Int32ConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}

type BoolConfigOption struct {
	ConfigOption
}

func (o BoolConfigOption) GetValue() bool {
	if o.Value == nil {
		return false
	}

	return o.Value.(bool)
}

func (o *BoolConfigOption) SetValue(value bool) {
	o.Value = value
}

func (o *BoolConfigOption) TrySetValue(value bool) bool {
	if o.Overridable {
		o.Value = value
		return true
	}
	return false
}

func NewBoolConfigOption(value bool, overridable bool) BoolConfigOption {
	return BoolConfigOption{ConfigOption{
		Value:       value,
		Overridable: overridable,
	}}
}
