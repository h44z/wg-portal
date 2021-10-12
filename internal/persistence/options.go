package persistence

type StringConfigOption struct {
	Value       string `gorm:"column:v"`
	Overridable bool   `gorm:"column:o"`
}

func (o StringConfigOption) GetValue() string {
	return o.Value
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
	return StringConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

type IntConfigOption struct {
	Value       int  `gorm:"column:v"`
	Overridable bool `gorm:"column:o"`
}

func (o IntConfigOption) GetValue() int {
	return o.Value
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
	return IntConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

type Int32ConfigOption struct {
	Value       int32 `gorm:"column:v"`
	Overridable bool  `gorm:"column:o"`
}

func (o Int32ConfigOption) GetValue() int32 {
	return o.Value
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
	return Int32ConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}

type BoolConfigOption struct {
	Value       bool `gorm:"column:v"`
	Overridable bool `gorm:"column:o"`
}

func (o BoolConfigOption) GetValue() bool {
	return o.Value
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
	return BoolConfigOption{
		Value:       value,
		Overridable: overridable,
	}
}
