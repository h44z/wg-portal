package domain

type ConfigOption[T any] struct {
	Value       T    `gorm:"column:v"`
	Overridable bool `gorm:"column:o"`
}

func (o *ConfigOption[T]) GetValue() T {
	return o.Value
}

func (o *ConfigOption[T]) SetValue(value T) {
	o.Value = value
}

func (o *ConfigOption[T]) TrySetValue(value T) bool {
	if o.Overridable {
		o.Value = value
		return true
	}
	return false
}

func NewConfigOption[T any](value T, overridable bool) ConfigOption[T] {
	return ConfigOption[T]{
		Value:       value,
		Overridable: overridable,
	}
}
