// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func BenchmarkFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fmt("{red}test{c}")
	}
}

func BenchmarkFormatLong(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fmt("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	}
}

func BenchmarkStripFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		TrimFmt("{red}test{c}")
	}
}

func BenchmarkStripFormatLong(b *testing.B) {
	for i := 0; i < b.N; i++ {
		TrimFmt("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	}
}

func BenchmarkStripRaw(b *testing.B) {
	text := Fmt("{red}test{c}")
	for i := 0; i < b.N; i++ {
		StripRaw(text)
	}
}

func BenchmarkStripRawLong(b *testing.B) {
	text := Fmt("{red}test {blue}2 {red}3 {brown} {italic}test{c}")
	for i := 0; i < b.N; i++ {
		StripRaw(text)
	}
}

var testsFormat = []struct {
	name string
	test string
	want string
}{
	{name: "middle", test: "test{red}test{c}test", want: "test\x0304test\x03test"},
	{name: "middle with bold", test: "test{red}{b}test{c}test", want: "test\x0304\x02test\x03test"},
	{name: "start, end", test: "{red}test{c}", want: "\x0304test\x03"},
	{name: "start, middle, end", test: "{red}te{red}st{c}", want: "\x0304te\x0304st\x03"},
	{name: "partial", test: "{redtest{c}", want: "{redtest\x03"},
	{name: "inside", test: "{re{c}d}test{c}", want: "{re\x03d}test\x03"},
	{name: "nothing", test: "this is a test.", want: "this is a test."},
	{name: "fg and bg", test: "{red,yellow}test{c}", want: "\x0304,08test\x03"},
	{name: "just bg", test: "{,yellow}test{c}", want: "test\x03"},
	{name: "just red", test: "{red}test", want: "\x0304test"},
	{name: "just cyan", test: "{cyan}test", want: "\x0311test"},
}

func FuzzFormat(f *testing.F) {
	for _, tc := range testsFormat {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		got := Fmt(orig)
		got2 := Fmt(got)

		if utf8.ValidString(orig) {
			if !utf8.ValidString(got) {
				t.Errorf("produced invalid UTF-8 string %q", got)
			}

			if !utf8.ValidString(got2) {
				t.Errorf("produced invalid UTF-8 string %q", got2)
			}
		}
	})
}

func TestFormat(t *testing.T) {
	for _, tt := range testsFormat {
		if got := Fmt(tt.test); got != tt.want {
			t.Errorf("%s: Format(%q) = %q, want %q", tt.name, tt.test, got, tt.want)
		}
	}
}

var testsStripFormat = []struct {
	name string
	test string
	want string
}{
	{name: "start, end", test: "{red}test{c}", want: "test"},
	{name: "start, middle, end", test: "{red}te{red}st{c}", want: "test"},
	{name: "partial", test: "{redtest{c}", want: "{redtest"},
	{name: "inside", test: "{re{c}d}test{c}", want: "{red}test"},
	{name: "nothing", test: "this is a test.", want: "this is a test."},
}

func FuzzStripFormat(f *testing.F) {
	for _, tc := range testsStripFormat {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		got := TrimFmt(orig)
		got2 := TrimFmt(got)

		if utf8.ValidString(orig) {
			if !utf8.ValidString(got) {
				t.Errorf("produced invalid UTF-8 string %q", got)
			}

			if !utf8.ValidString(got2) {
				t.Errorf("produced invalid UTF-8 string %q", got2)
			}
		}
	})
}

func TestStripFormat(t *testing.T) {
	for _, tt := range testsStripFormat {
		if got := TrimFmt(tt.test); got != tt.want {
			t.Errorf("%s: StripFormat(%q) = %q, want %q", tt.name, tt.test, got, tt.want)
		}
	}
}

var testsStripRaw = []struct {
	name string
	test string // gets passed to Format() before sent
	want string
}{
	{name: "start, end", test: "{red}{b}test{c}", want: "test"},
	{name: "start, end in numbers", test: "{red}1234{c}", want: "1234"},
	{name: "start, middle, end", test: "{red}te{red}st{c}", want: "test"},
	{name: "partial", test: "{redtest{c}", want: "{redtest"},
	{name: "inside", test: "{re{c}d}test{c}", want: "{red}test"},
	{name: "fg+bg colors start", test: "{red,yellow}test{c}", want: "test"},
	{name: "fg+bg colors start in numbers", test: "{red,yellow}1234{c}", want: "1234"},
	{name: "fg+bg colors end", test: "test{,yellow}", want: "test"},
	{name: "bg colors start", test: "{,yellow}test{c}", want: "test"},
	{name: "inside", test: "{re{c}d}test{c}", want: "{red}test"},
	{name: "nothing", test: "this is a test.", want: "this is a test."},
}

func FuzzStripRaw(f *testing.F) {
	for _, tc := range testsStripRaw {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		got := StripRaw(orig)
		got2 := StripRaw(got)

		if utf8.ValidString(orig) {
			if !utf8.ValidString(got) {
				t.Errorf("produced invalid UTF-8 string %q", got)
			}

			if !utf8.ValidString(got2) {
				t.Errorf("produced invalid UTF-8 string %q", got2)
			}
		}
	})
}

func TestStripRaw(t *testing.T) {
	for _, tt := range testsStripRaw {
		if got := StripRaw(Fmt(tt.test)); got != tt.want {
			t.Fatalf("%s: StripRaw(%q) = %q, want %q", tt.name, tt.test, got, tt.want)
		}
	}
}

var testsValidNick = []struct {
	name string
	test string
	want bool
}{
	{name: "normal", test: "test", want: true},
	{name: "empty", test: "", want: false},
	{name: "hyphen and special", test: "test[-]", want: true},
	{name: "invalid middle", test: "test!test", want: false},
	{name: "invalid dot middle", test: "test.test", want: false},
	{name: "end", test: "test!", want: false},
	{name: "invalid start", test: "!test", want: false},
	{name: "backslash and numeric", test: "test[\\0", want: true},
	{name: "long", test: "test123456789AZBKASDLASMDLKM", want: true},
	{name: "index 0 dash", test: "-test", want: false},
	{name: "index 0 numeric", test: "0test", want: false},
	{name: "RFC1459 non-lowercase-converted", test: "test^", want: true},
	{name: "RFC1459 non-lowercase-converted", test: "test~", want: false},
}

func FuzzValidNick(f *testing.F) {
	for _, tc := range testsValidNick {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		_ = IsValidNick(orig)
	})
}

func TestIsValidNick(t *testing.T) {
	for _, tt := range testsValidNick {
		if got := IsValidNick(tt.test); got != tt.want {
			t.Errorf("%s: IsValidNick(%q) = %v, want %v", tt.name, tt.test, got, tt.want)
		}
	}
}

var testsValidChannel = []struct {
	name string
	test string
	want bool
}{
	{name: "valid channel", test: "#valid", want: true},
	{name: "invalid channel comma", test: "#invalid,", want: false},
	{name: "invalid channel space", test: "#inva lid", want: false},
	{name: "valid channel with numerics", test: "#1valid0", want: true},
	{name: "valid channel with special", test: "#valid[]test", want: true},
	{name: "valid channel with special", test: "#[]valid[]test[]", want: true},
	{name: "just hash", test: "#", want: false},
	{name: "empty", test: "", want: false},
	{name: "invalid prefix", test: "$invalid", want: false},
	{name: "too long", test: "#aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", want: false},
	{name: "valid id prefix", test: "!12345test", want: true},
	{name: "invalid id length", test: "!1234", want: false},
	{name: "invalid id length", test: "!12345", want: false},
	{name: "invalid id prefix", test: "!test1invalid", want: false},
}

func FuzzValidChannel(f *testing.F) {
	for _, tc := range testsValidChannel {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		_ = IsValidChannel(orig)
	})
}

func TestIsValidChannel(t *testing.T) {
	for _, tt := range testsValidChannel {
		if got := IsValidChannel(tt.test); got != tt.want {
			t.Errorf("%s: IsValidChannel(%q) = %v, want %v", tt.name, tt.test, got, tt.want)
		}
	}
}

var testsValidUser = []struct {
	name string
	test string
	want bool
}{
	{name: "user without ident server", test: "~test", want: true},
	{name: "user with ident server", test: "test", want: true},
	{name: "non-alphanumeric first index", test: "-test", want: false},
	{name: "non-alphanumeric first index", test: "[test]", want: false},
	{name: "numeric first index", test: "0test", want: true},
	{name: "blank", test: "", want: false},
	{name: "just tilde", test: "~", want: false},
	{name: "special chars", test: "test-----", want: true},
	{name: "special chars", test: "test-[]-", want: true},
	{name: "special chars, invalid after first index", test: "t!--", want: false},
}

func FuzzValidUser(f *testing.F) {
	for _, tc := range testsValidUser {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		_ = IsValidUser(orig)
	})
}

func TestIsValidUser(t *testing.T) {
	for _, tt := range testsValidUser {
		if got := IsValidUser(tt.test); got != tt.want {
			t.Errorf("%s: IsValidUser(%q) = %v, want %v", tt.name, tt.test, got, tt.want)
		}
	}
}

var testsToRFC1459 = []struct {
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

func FuzzToRFC1459(f *testing.F) {
	for _, tc := range testsToRFC1459 {
		f.Add(tc.in)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		got := ToRFC1459(orig)

		if utf8.ValidString(orig) && !utf8.ValidString(got) {
			t.Errorf("produced invalid UTF-8 string %q", got)
		}
	})
}

func TestToRFC1459(t *testing.T) {
	for _, tt := range testsToRFC1459 {
		if got := ToRFC1459(tt.in); got != tt.want {
			t.Errorf("ToRFC1459() = %q, want %q", got, tt.want)
		}
	}
}

func BenchmarkGlob(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if !Glob("*quick*fox*dog", "The quick brown fox jumped over the lazy dog") {
			b.Fatalf("should match")
		}
	}
}

func testGlobMatch(t *testing.T, subj, pattern string) {
	if !Glob(subj, pattern) {
		t.Fatalf("'%s' should match '%s'", pattern, subj)
	}
}

func testGlobNoMatch(t *testing.T, subj, pattern string) {
	if Glob(subj, pattern) {
		t.Fatalf("'%s' should not match '%s'", pattern, subj)
	}
}

func TestEmptyPattern(t *testing.T) {
	testGlobMatch(t, "", "")
	testGlobNoMatch(t, "test", "")
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
}

func TestPatternWithoutGlobs(t *testing.T) {
	testGlobMatch(t, "test", "test")
}

var testsGlob = []string{
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

func FuzzGlob(f *testing.F) {
	for _, tc := range testsGlob {
		f.Add(tc, tc)
	}

	f.Fuzz(func(t *testing.T, orig, orig2 string) {
		_ = Glob(orig, orig2)
	})
}

func TestGlob(t *testing.T) {
	for _, pattern := range testsGlob {
		testGlobMatch(t, "this is a ϗѾ test", pattern)
	}

	cases := []string{
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
}
