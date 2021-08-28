package wireguard

import (
	"reflect"
	"testing"
)

func TestBoolConfigOption_GetValue(t *testing.T) {
	type fields struct {
		ConfigOption ConfigOption
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{
			name:   "Empty",
			fields: fields{},
			want:   false,
		},
		{
			name:   "True",
			fields: fields{ConfigOption: ConfigOption{Value: true}},
			want:   true,
		},
		{
			name:   "False",
			fields: fields{ConfigOption: ConfigOption{Value: false}},
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := BoolConfigOption{
				ConfigOption: tt.fields.ConfigOption,
			}
			if got := o.GetValue(); got != tt.want {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInt32ConfigOption_GetValue(t *testing.T) {
	type fields struct {
		ConfigOption ConfigOption
	}
	tests := []struct {
		name   string
		fields fields
		want   int32
	}{
		{
			name:   "Empty",
			fields: fields{},
			want:   0,
		},
		{
			name:   "Leet",
			fields: fields{ConfigOption: ConfigOption{Value: int32(1337)}},
			want:   1337,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := Int32ConfigOption{
				ConfigOption: tt.fields.ConfigOption,
			}
			if got := o.GetValue(); got != tt.want {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntConfigOption_GetValue(t *testing.T) {
	type fields struct {
		ConfigOption ConfigOption
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name:   "Empty",
			fields: fields{},
			want:   0,
		},
		{
			name:   "Leet",
			fields: fields{ConfigOption: ConfigOption{Value: 1337}},
			want:   1337,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := IntConfigOption{
				ConfigOption: tt.fields.ConfigOption,
			}
			if got := o.GetValue(); got != tt.want {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStringConfigOption_GetValue(t *testing.T) {
	type fields struct {
		ConfigOption ConfigOption
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "Empty",
			fields: fields{},
			want:   "",
		},
		{
			name:   "Leet",
			fields: fields{ConfigOption: ConfigOption{Value: "leet"}},
			want:   "leet",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := StringConfigOption{
				ConfigOption: tt.fields.ConfigOption,
			}
			if got := o.GetValue(); got != tt.want {
				t.Errorf("GetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewBoolConfigOption(t *testing.T) {
	type args struct {
		value       bool
		overridable bool
	}
	tests := []struct {
		name string
		args args
		want BoolConfigOption
	}{
		{
			name: "Overridable",
			args: args{value: false, overridable: true},
			want: BoolConfigOption{ConfigOption: ConfigOption{Value: false, Overridable: true}},
		},
		{
			name: "Not Overridable",
			args: args{value: true, overridable: false},
			want: BoolConfigOption{ConfigOption: ConfigOption{Value: true, Overridable: false}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewBoolConfigOption(tt.args.value, tt.args.overridable); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewBoolConfigOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewInt32ConfigOption(t *testing.T) {
	type args struct {
		value       int32
		overridable bool
	}
	tests := []struct {
		name string
		args args
		want Int32ConfigOption
	}{
		{
			name: "Overridable",
			args: args{value: 1337, overridable: true},
			want: Int32ConfigOption{ConfigOption: ConfigOption{Value: int32(1337), Overridable: true}},
		},
		{
			name: "Not Overridable",
			args: args{value: 1337, overridable: false},
			want: Int32ConfigOption{ConfigOption: ConfigOption{Value: int32(1337), Overridable: false}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewInt32ConfigOption(tt.args.value, tt.args.overridable); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewInt32ConfigOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewIntConfigOption(t *testing.T) {
	type args struct {
		value       int
		overridable bool
	}
	tests := []struct {
		name string
		args args
		want IntConfigOption
	}{
		{
			name: "Overridable",
			args: args{value: 1337, overridable: true},
			want: IntConfigOption{ConfigOption: ConfigOption{Value: 1337, Overridable: true}},
		},
		{
			name: "Not Overridable",
			args: args{value: 1337, overridable: false},
			want: IntConfigOption{ConfigOption: ConfigOption{Value: 1337, Overridable: false}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewIntConfigOption(tt.args.value, tt.args.overridable); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewIntConfigOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewStringConfigOption(t *testing.T) {
	type args struct {
		value       string
		overridable bool
	}
	tests := []struct {
		name string
		args args
		want StringConfigOption
	}{
		{
			name: "Overridable",
			args: args{value: "leet", overridable: true},
			want: StringConfigOption{ConfigOption: ConfigOption{Value: "leet", Overridable: true}},
		},
		{
			name: "Not Overridable",
			args: args{value: "leet", overridable: false},
			want: StringConfigOption{ConfigOption: ConfigOption{Value: "leet", Overridable: false}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewStringConfigOption(tt.args.value, tt.args.overridable); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewStringConfigOption() = %v, want %v", got, tt.want)
			}
		})
	}
}
