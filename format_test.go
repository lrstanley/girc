// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"testing"
)

func BenchmarkFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Format("{red}test{c}")
	}

	return
}

func BenchmarkFormatLong(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Format("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	}

	return
}

func BenchmarkStripFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		StripFormat("{red}test{c}")
	}

	return
}

func BenchmarkStripFormatLong(b *testing.B) {
	for i := 0; i < b.N; i++ {
		StripFormat("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	}

	return
}

func BenchmarkStripColors(b *testing.B) {
	text := Format("{red}test{c}")
	for i := 0; i < b.N; i++ {
		StripColors(text)
	}

	return
}

func BenchmarkStripColorsLong(b *testing.B) {
	text := Format("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	for i := 0; i < b.N; i++ {
		StripColors(text)
	}

	return
}

func TestFormat(t *testing.T) {
	type args struct {
		text string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "start, end", args: args{text: "{red}test{c}"}, want: "\x0304test\x03"},
		{name: "start, middle, end", args: args{text: "{red}te{red}st{c}"}, want: "\x0304te\x0304st\x03"},
		{name: "partial", args: args{text: "{redtest{c}"}, want: "{redtest\x03"},
		{name: "inside", args: args{text: "{re{c}d}test{c}"}, want: "{re\x03d}test\x03"},
		{name: "nothing", args: args{text: "this is a test."}, want: "this is a test."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Format(tt.args.text); got != tt.want {
				t.Errorf("Format() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripFormat(t *testing.T) {
	type args struct {
		text string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "start, end", args: args{text: "{red}test{c}"}, want: "test"},
		{name: "start, middle, end", args: args{text: "{red}te{red}st{c}"}, want: "test"},
		{name: "partial", args: args{text: "{redtest{c}"}, want: "{redtest"},
		{name: "inside", args: args{text: "{re{c}d}test{c}"}, want: "{red}test"},
		{name: "nothing", args: args{text: "this is a test."}, want: "this is a test."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripFormat(tt.args.text); got != tt.want {
				t.Errorf("StripFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStripColors(t *testing.T) {
	type args struct {
		text string
	}

	tests := []struct {
		name string
		args args // gets passed to Format() before sent
		want string
	}{
		{name: "start, end", args: args{text: "{red}test{c}"}, want: "test"},
		{name: "start, middle, end", args: args{text: "{red}te{red}st{c}"}, want: "test"},
		{name: "partial", args: args{text: "{redtest{c}"}, want: "{redtest"},
		{name: "inside", args: args{text: "{re{c}d}test{c}"}, want: "{red}test"},
		{name: "nothing", args: args{text: "this is a test."}, want: "this is a test."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := StripColors(Format(tt.args.text)); got != tt.want {
				t.Errorf("StripColors() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidNick(tt.args.nick); got != tt.want {
				t.Errorf("IsValidNick() = %v, want %v", got, tt.want)
			}
		})
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
		{name: "invalid channel comma", args: args{channel: "#invalid,"}, want: false},
		{name: "invalid channel space", args: args{channel: "#inva lid"}, want: false},
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
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidChannel(tt.args.channel); got != tt.want {
				t.Errorf("IsValidChannel() = %v, want %v", got, tt.want)
			}
		})
	}
}
