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
		if got := Format(tt.args.text); got != tt.want {
			t.Errorf("%q. Format() = %v, want %v", tt.name, got, tt.want)
		}
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
		if got := StripFormat(tt.args.text); got != tt.want {
			t.Errorf("%q. StripFormat() = %v, want %v", tt.name, got, tt.want)
		}
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
		if got := StripColors(Format(tt.args.text)); got != tt.want {
			t.Errorf("%q. StripColors() = %v, want %v", tt.name, got, tt.want)
		}
	}
}
