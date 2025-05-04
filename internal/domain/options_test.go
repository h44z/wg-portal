package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigOption_GetValueReturnsCorrectValue(t *testing.T) {
	option := ConfigOption[int]{Value: 42}
	assert.Equal(t, 42, option.GetValue())
}

func TestConfigOption_SetValueUpdatesValue(t *testing.T) {
	option := ConfigOption[int]{Value: 42}
	option.SetValue(100)
	assert.Equal(t, 100, option.GetValue())
}

func TestConfigOption_TrySetValueUpdatesValueWhenOverridable(t *testing.T) {
	option := ConfigOption[int]{Value: 42, Overridable: true}
	result := option.TrySetValue(100)
	assert.True(t, result)
	assert.Equal(t, 100, option.GetValue())
}

func TestConfigOption_TrySetValueDoesNotUpdateValueWhenNotOverridable(t *testing.T) {
	option := ConfigOption[int]{Value: 42, Overridable: false}
	result := option.TrySetValue(100)
	assert.False(t, result)
	assert.Equal(t, 42, option.GetValue())
}

func TestNewConfigOptionCreatesCorrectOption(t *testing.T) {
	option := NewConfigOption(42, true)
	assert.Equal(t, 42, option.GetValue())
	assert.True(t, option.Overridable)

	option2 := NewConfigOption("str", false)
	assert.Equal(t, "str", option2.GetValue())
	assert.False(t, option2.Overridable)
}
