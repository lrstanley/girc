// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"strings"
	"testing"
)

func BenchmarkFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fmt("{red}test{c}")
	}

	return
}

func BenchmarkFormatLong(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fmt("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	}

	return
}

func BenchmarkStripFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		TrimFmt("{red}test{c}")
	}

	return
}

func BenchmarkStripFormatLong(b *testing.B) {
	for i := 0; i < b.N; i++ {
		TrimFmt("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	}

	return
}

func BenchmarkStripRaw(b *testing.B) {
	text := Fmt("{red}test{c}")
	for i := 0; i < b.N; i++ {
		StripRaw(text)
	}

	return
}

func BenchmarkStripRawLong(b *testing.B) {
	text := Fmt("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	for i := 0; i < b.N; i++ {
		StripRaw(text)
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
		{name: "middle", args: args{text: "test{red}test{c}test"}, want: "test\x0304test\x03test"},
		{name: "middle with bold", args: args{text: "test{red}{b}test{c}test"}, want: "test\x0304\x02test\x03test"},
		{name: "start, end", args: args{text: "{red}test{c}"}, want: "\x0304test\x03"},
		{name: "start, middle, end", args: args{text: "{red}te{red}st{c}"}, want: "\x0304te\x0304st\x03"},
		{name: "partial", args: args{text: "{redtest{c}"}, want: "{redtest\x03"},
		{name: "inside", args: args{text: "{re{c}d}test{c}"}, want: "{re\x03d}test\x03"},
		{name: "nothing", args: args{text: "this is a test."}, want: "this is a test."},
		{name: "fg and bg", args: args{text: "{red,yellow}test{c}"}, want: "\x0304,08test\x03"},
		{name: "just bg", args: args{text: "{,yellow}test{c}"}, want: "test\x03"},
		{name: "just red", args: args{text: "{red}test"}, want: "\x0304test"},
		{name: "just cyan", args: args{text: "{cyan}test"}, want: "\x0311test"},
	}

	for _, tt := range tests {
		if got := Fmt(tt.args.text); got != tt.want {
			t.Errorf("%s: Format(%q) = %q, want %q", tt.name, tt.args.text, got, tt.want)
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
		if got := TrimFmt(tt.args.text); got != tt.want {
			t.Errorf("%s: StripFormat(%q) = %q, want %q", tt.name, tt.args.text, got, tt.want)
		}
	}
}

func TestStripRaw(t *testing.T) {
	type args struct {
		text string
	}

	tests := []struct {
		name string
		args args // gets passed to Format() before sent
		want string
	}{
		{name: "start, end", args: args{text: "{red}{b}test{c}"}, want: "test"},
		{name: "start, end in numbers", args: args{text: "{red}1234{c}"}, want: "1234"},
		{name: "start, middle, end", args: args{text: "{red}te{red}st{c}"}, want: "test"},
		{name: "partial", args: args{text: "{redtest{c}"}, want: "{redtest"},
		{name: "inside", args: args{text: "{re{c}d}test{c}"}, want: "{red}test"},
		{name: "fg+bg colors start", args: args{text: "{red,yellow}test{c}"}, want: "test"},
		{name: "fg+bg colors start in numbers", args: args{text: "{red,yellow}1234{c}"}, want: "1234"},
		{name: "fg+bg colors end", args: args{text: "test{,yellow}"}, want: "test"},
		{name: "bg colors start", args: args{text: "{,yellow}test{c}"}, want: "test"},
		{name: "inside", args: args{text: "{re{c}d}test{c}"}, want: "{red}test"},
		{name: "nothing", args: args{text: "this is a test."}, want: "this is a test."},
	}

	for _, tt := range tests {
		if got := StripRaw(Fmt(tt.args.text)); got != tt.want {
			t.Fatalf("%s: StripRaw(%q) = %q, want %q", tt.name, tt.args.text, got, tt.want)
		}
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
		{name: "RFC1459 non-lowercase-converted", args: args{nick: "test^"}, want: true},
		{name: "RFC1459 non-lowercase-converted", args: args{nick: "test~"}, want: false},
	}
	for _, tt := range tests {
		if got := IsValidNick(tt.args.nick); got != tt.want {
			t.Errorf("%s: IsValidNick(%q) = %v, want %v", tt.name, tt.args.nick, got, tt.want)
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
		{name: "invalid channel comma", args: args{channel: "#invalid,"}, want: false},
		{name: "invalid channel space", args: args{channel: "#inva lid"}, want: false},
		{name: "valid channel with numerics", args: args{channel: "#1valid0"}, want: true},
		{name: "valid channel with special", args: args{channel: "#valid[]test"}, want: true},
		{name: "valid channel with special", args: args{channel: "#[]valid[]test[]"}, want: true},
		{name: "just hash", args: args{channel: "#"}, want: false},
		{name: "empty", args: args{channel: ""}, want: false},
		{name: "invalid prefix", args: args{channel: "$invalid"}, want: false},
		{name: "too long", args: args{channel: "#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}, want: false},
		{name: "valid id prefix", args: args{channel: "!12345test"}, want: true},
		{name: "invalid id length", args: args{channel: "!1234"}, want: false},
		{name: "invalid id length", args: args{channel: "!12345"}, want: false},
		{name: "invalid id prefix", args: args{channel: "!test1invalid"}, want: false},
	}
	for _, tt := range tests {
		if got := IsValidChannel(tt.args.channel); got != tt.want {
			t.Errorf("%s: IsValidChannel(%q) = %v, want %v", tt.name, tt.args.channel, got, tt.want)
		}
	}
}

func TestIsValidUser(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "user without ident server", args: args{name: "~test"}, want: true},
		{name: "user with ident server", args: args{name: "test"}, want: true},
		{name: "non-alphanumeric first index", args: args{name: "-test"}, want: false},
		{name: "non-alphanumeric first index", args: args{name: "[test]"}, want: false},
		{name: "numeric first index", args: args{name: "0test"}, want: true},
		{name: "blank", args: args{name: ""}, want: false},
		{name: "just tilde", args: args{name: "~"}, want: false},
		{name: "special chars", args: args{name: "test-----"}, want: true},
		{name: "special chars", args: args{name: "test-[]-"}, want: true},
		{name: "special chars, invalid after first index", args: args{name: "t!--"}, want: false},
	}
	for _, tt := range tests {
		if got := IsValidUser(tt.args.name); got != tt.want {
			t.Errorf("%s: IsValidUser(%q) = %v, want %v", tt.name, tt.args.name, got, tt.want)
		}
	}
}

func TestToRFC1459(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"a", "a"},
		{"abcd", "abcd"},
		{"AbcD", "abcd"},
		{"!@#$%^&*()_+-=", "!@#$%~&*()_+-="},
		{"Abcd[]", "abcd{}"},
	}

	for _, tt := range cases {
		if got := ToRFC1459(tt.in); got != tt.want {
			t.Errorf("ToRFC1459() = %q, want %q", got, tt.want)
		}
	}

	return
}

//func BenchmarkGlob(b *testing.B) {
//	for i := 0; i < b.N; i++ {
//		if !Glob("*quick*fox*dog", "The quick brown fox jumped over the lazy dog") {
//			b.Fatalf("should match")
//		}
//	}
//
//	return
//}

func testGlobMatch(t *testing.T, subj, pattern string) {
	if !Glob(subj, pattern) {
		t.Fatalf("'%s' should match '%s'", pattern, subj)
	}

	return
}

func testGlobNoMatch(t *testing.T, subj, pattern string) {
	if Glob(subj, pattern) {
		t.Fatalf("'%s' should not match '%s'", pattern, subj)
	}

	return
}

func TestEmptyPattern(t *testing.T) {
	testGlobMatch(t, "", "")
	testGlobNoMatch(t, "test", "")

	return
}

func TestEmptySubject(t *testing.T) {
	cases := []string{
		"",
		"*",
		"**",
		"***",
		"****************",
		strings.Repeat("*", 1000000),
	}

	for _, pattern := range cases {
		testGlobMatch(t, "", pattern)
	}

	cases = []string{
		// No globs/non-glob characters.
		"test",
		"*test*",

		// Trailing characters.
		"*x",
		"*****************x",
		strings.Repeat("*", 1000000) + "x",

		// Leading characters.
		"x*",
		"x*****************",
		"x" + strings.Repeat("*", 1000000),

		// Mixed leading/trailing characters.
		"x*x",
		"x****************x",
		"x" + strings.Repeat("*", 1000000) + "x",
	}

	for _, pattern := range cases {
		testGlobNoMatch(t, pattern, "")
	}

	return
}

func TestPatternWithoutGlobs(t *testing.T) {
	testGlobMatch(t, "test", "test")

	return
}

func TestGlob(t *testing.T) {
	cases := []string{
		"*test",           // Leading.
		"this*",           // Trailing.
		"this*test",       // Middle.
		"*is *",           // String in between two.
		"*is*a*",          // Lots.
		"**test**",        // Double glob characters.
		"**is**a***test*", // Varying number.
		"* *",             // White space between.
		"*",               // Lone.
		"**********",      // Nothing but globs.
		"*Ѿ*",             // Unicode.
		"*is a ϗѾ *",      // Mixed ASCII/unicode.
	}

	for _, pattern := range cases {
		testGlobMatch(t, "this is a ϗѾ test", pattern)
	}

	cases = []string{
		"test*", // Implicit substring match.
		"*is",   // Partial match.
		"*no*",  // Globs without a match between them.
		" ",     // Plain white space.
		"* ",    // Trailing white space.
		" *",    // Leading white space.
		"*ʤ*",   // Non-matching unicode.
	}

	// Non-matches
	for _, pattern := range cases {
		testGlobNoMatch(t, "this is a test", pattern)
	}

	return
}
