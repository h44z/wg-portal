package authentication

import (
	"reflect"
	"testing"
)

func Test_mapDefaultString(t *testing.T) {
	type args struct {
		m            map[string]interface{}
		key          string
		defaultValue string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "match",
			args: args{
				m:            map[string]interface{}{"hello": "world"},
				key:          "hello",
				defaultValue: "",
			},
			want: "world",
		}, {
			name: "no_match",
			args: args{
				m:            map[string]interface{}{"hello": "world"},
				key:          "hi",
				defaultValue: "",
			},
			want: "",
		}, {
			name: "nil_value",
			args: args{
				m:            map[string]interface{}{"hello": nil},
				key:          "hello",
				defaultValue: "",
			},
			want: "",
		}, {
			name: "default_nil_value",
			args: args{
				m:            map[string]interface{}{"hello": nil},
				key:          "hello",
				defaultValue: "world",
			},
			want: "world",
		}, {
			name: "nil_map",
			args: args{
				m:            nil,
				key:          "hi",
				defaultValue: "world",
			},
			want: "world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapDefaultString(tt.args.m, tt.args.key, tt.args.defaultValue); got != tt.want {
				t.Errorf("mapDefaultString() = %v, want %v", got, tt.want)
			}
		})
	}

}

func Test_uniqueStringSlice(t *testing.T) {
	type args struct {
		slice []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Empty",
			args: args{},
			want: []string{},
		},
		{
			name: "Single",
			args: args{slice: []string{"1"}},
			want: []string{"1"},
		},
		{
			name: "Normal",
			args: args{slice: []string{"1", "2", "3"}},
			want: []string{"1", "2", "3"},
		},
		{
			name: "Duplicate",
			args: args{slice: []string{"1", "2", "2"}},
			want: []string{"1", "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uniqueStringSlice(tt.args.slice); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("uniqueStringSlice() = %v, want %v", got, tt.want)
			}
		})
	}
}
