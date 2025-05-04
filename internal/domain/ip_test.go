package domain

import (
	"net/netip"
	"reflect"
	"testing"
)

func TestCidrFromString(t *testing.T) {
	type args struct {
		str string
	}
	tests := []struct {
		name    string
		args    args
		want    Cidr
		wantErr bool
	}{
		{
			name:    "IPv4",
			args:    args{str: "1.2.3.4/24"},
			want:    CidrFromPrefix(netip.MustParsePrefix("1.2.3.4/24")),
			wantErr: false,
		},
		{
			name:    "IPv4 Network",
			args:    args{str: "1.2.3.0/24"},
			want:    CidrFromPrefix(netip.MustParsePrefix("1.2.3.0/24")),
			wantErr: false,
		},
		{
			name:    "IPv4 error",
			args:    args{str: "1.1/24"},
			want:    Cidr{},
			wantErr: true,
		},
		{
			name:    "IPv6 short",
			args:    args{str: "fe00:1234::1/64"},
			want:    CidrFromPrefix(netip.MustParsePrefix("fe00:1234::1/64")),
			wantErr: false,
		},
		{
			name:    "IPv6",
			args:    args{str: "2A02:810A:900:333E:3B74:D237:E076:8B36/128"},
			want:    CidrFromPrefix(netip.MustParsePrefix("2A02:810A:900:333E:3B74:D237:E076:8B36/128")),
			wantErr: false,
		},
		{
			name:    "IPv6 Network",
			args:    args{str: "fe00::/56"},
			want:    CidrFromPrefix(netip.MustParsePrefix("fe00::/56")),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CidrFromString(tt.args.str)
			if (err != nil) != tt.wantErr {
				t.Errorf("CidrFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CidrFromString() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCidr_BroadcastAddr(t *testing.T) {
	type fields struct {
		Prefix netip.Prefix
	}
	tests := []struct {
		name   string
		fields fields
		want   Cidr
	}{
		{
			name:   "V4",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.4/24")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.2.3.255/24")),
		},
		{
			name:   "V6",
			fields: fields{Prefix: netip.MustParsePrefix("fe00:d3ad:b33f:c0d3::/64")},
			want:   CidrFromPrefix(netip.MustParsePrefix("fe00:d3ad:b33f:c0d3:ffff:ffff:ffff:ffff/64")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CidrFromPrefix(tt.fields.Prefix)
			if got := c.BroadcastAddr(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BroadcastAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCidr_NetworkAddr(t *testing.T) {
	type fields struct {
		Prefix netip.Prefix
	}
	tests := []struct {
		name   string
		fields fields
		want   Cidr
	}{

		{
			name:   "V4",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.4/24")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.2.3.0/24")),
		},
		{
			name:   "V6",
			fields: fields{Prefix: netip.MustParsePrefix("fe00:d3ad:b33f:c0d3::1234/64")},
			want:   CidrFromPrefix(netip.MustParsePrefix("fe00:d3ad:b33f:c0d3::/64")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CidrFromPrefix(tt.fields.Prefix)
			if got := c.NetworkAddr(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NetworkAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCidr_NextAddr(t *testing.T) {
	type fields struct {
		Prefix netip.Prefix
	}
	tests := []struct {
		name   string
		fields fields
		want   Cidr
	}{
		{
			name:   "V4 normal",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.4/24")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.2.3.5/24")),
		},
		{
			name:   "V4 broadcast",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.254/24")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.2.3.255/24")),
		},
		{
			name:   "V4 overflow",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.255/24")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.2.4.0/24")),
		},
		{
			name:   "V6 normal",
			fields: fields{Prefix: netip.MustParsePrefix("fe00::1/64")},
			want:   CidrFromPrefix(netip.MustParsePrefix("fe00::2/64")),
		},
		{
			name:   "V6 overflow",
			fields: fields{Prefix: netip.MustParsePrefix("fe00::ffff:ffff:ffff:ffff/64")},
			want:   CidrFromPrefix(netip.MustParsePrefix("fe00:0:0:1::/64")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CidrFromPrefix(tt.fields.Prefix)
			if got := c.NextAddr(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NextAddr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCidr_NextSubnet(t *testing.T) {
	type fields struct {
		Prefix netip.Prefix
	}
	tests := []struct {
		name   string
		fields fields
		want   Cidr
	}{
		{
			name:   "V4",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.4/24")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.2.4.0/24")),
		},
		{
			name:   "V4 bigger subnet",
			fields: fields{Prefix: netip.MustParsePrefix("1.2.3.4/16")},
			want:   CidrFromPrefix(netip.MustParsePrefix("1.3.0.0/16")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := CidrFromPrefix(tt.fields.Prefix)
			if got := c.NextSubnet(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NextSubnet() = %v, want %v", got, tt.want)
			}
		})
	}
}
