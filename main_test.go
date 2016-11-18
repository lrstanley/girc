// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

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

func TestIsValidChannel(t *testing.T) {
	type args struct {
		channel string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "valid channel", args: args{channel: "#valid"}, want: true},
		{name: "valid channel", args: args{channel: "#invalid,"}, want: false},
		{name: "valid channel", args: args{channel: "#inva lid"}, want: false},
		{name: "valid channel", args: args{channel: "#valid"}, want: true},
		{name: "valid channel with numerics", args: args{channel: "#1valid0"}, want: true},
		{name: "valid channel with special", args: args{channel: "#valid[]test"}, want: true},
		{name: "valid channel with special x2", args: args{channel: "#[]valid[]test[]"}, want: true},
		{name: "just hash", args: args{channel: "#"}, want: false},
		{name: "empty", args: args{channel: ""}, want: false},
		{name: "invalid prefix", args: args{channel: "$invalid"}, want: false},
		{name: "too long", args: args{channel: "#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, want: false},
		{name: "valid id prefix", args: args{channel: "!12345test"}, want: true},
		{name: "invalid id length", args: args{channel: "!1234"}, want: false},
		{name: "invalid id length x2", args: args{channel: "!12345"}, want: false},
		{name: "invalid id prefix", args: args{channel: "!test1invalid"}, want: false},
	}
	for _, tt := range tests {
		if got := IsValidChannel(tt.args.channel); got != tt.want {
			t.Errorf("%q. IsValidChannel() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
