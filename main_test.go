package girc

import "testing"

func TestIsValidNick(t *testing.T) {
	type args struct {
		nick string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "normal", args: args{nick: "test"}, want: true},
		{name: "empty", args: args{nick: ""}, want: false},
		{name: "hyphen and special", args: args{nick: "test[-]"}, want: true},
		{name: "invalid middle", args: args{nick: "test!test"}, want: false},
		{name: "invalid dot middle", args: args{nick: "test.test"}, want: false},
		{name: "end", args: args{nick: "test!"}, want: false},
		{name: "invalid start", args: args{nick: "!test"}, want: false},
		{name: "backslash and numeric", args: args{nick: "test[\\0"}, want: true},
		{name: "long", args: args{nick: "test123456789AZBKASDLASMDLKM"}, want: true},
		{name: "index 0 dash", args: args{nick: "-test"}, want: false},
		{name: "index 0 numeric", args: args{nick: "0test"}, want: false},
	}
	for _, tt := range tests {
		if got := IsValidNick(tt.args.nick); got != tt.want {
			t.Errorf("%q. IsValidNick() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
