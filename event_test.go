// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"reflect"
	"testing"
	"unicode/utf8"
)

func mockEvent() *Event {
	return &Event{
		Source:  &Source{Name: "nick", Ident: "user", Host: "host.com"},
		Command: "PRIVMSG",
		Params:  []string{"#channel", "1 2 3"},
	}
}

var testsParseSource = []struct {
	name    string
	test    string
	wantSrc *Source
}{
	{name: "full", test: "nick!user@hostname.com", wantSrc: &Source{
		Name: "nick", Ident: "user", Host: "hostname.com",
	}},
	{name: "special chars", test: "^[]nick!~user@test.host---name.com", wantSrc: &Source{
		Name: "^[]nick", Ident: "~user", Host: "test.host---name.com",
	}},
	{name: "short", test: "a!b@c", wantSrc: &Source{
		Name: "a", Ident: "b", Host: "c",
	}},
	{name: "short", test: "a!b", wantSrc: &Source{
		Name: "a", Ident: "b", Host: "",
	}},
	{name: "short", test: "a@b", wantSrc: &Source{
		Name: "a", Ident: "", Host: "b",
	}},
	{name: "short", test: "test", wantSrc: &Source{
		Name: "test", Ident: "", Host: "",
	}},
}

func FuzzParseSource(f *testing.F) {
	for _, tc := range testsParseSource {
		f.Add(tc.test)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		got := ParseSource(orig)

		_ = got.IsHostmask()
		_ = got.IsServer()
		_ = got.Len()

		if utf8.ValidString(orig) {
			if !utf8.ValidString(got.Host) {
				t.Errorf("produced invalid UTF-8 string %q", got.Host)
			}

			if !utf8.ValidString(got.Ident) {
				t.Errorf("produced invalid UTF-8 string %q", got.Ident)
			}

			if !utf8.ValidString(got.Name) {
				t.Errorf("produced invalid UTF-8 string %q", got.Name)
			}

			if !utf8.ValidString(got.ID()) {
				t.Errorf("produced invalid UTF-8 string %q", got.ID())
			}

			if !utf8.ValidString(got.String()) {
				t.Errorf("produced invalid UTF-8 string %q", got.String())
			}

			if !utf8.Valid(got.Bytes()) {
				t.Errorf("produced invalid UTF-8 []byte %q", got.Bytes())
			}
		}
	})
}

func TestParseSource(t *testing.T) {
	for _, tt := range testsParseSource {
		gotSrc := ParseSource(tt.test)

		if !reflect.DeepEqual(gotSrc, tt.wantSrc) {
			t.Errorf("ParseSource() = %v, want %v", gotSrc, tt.wantSrc)
		}

		if gotSrc.Len() != tt.wantSrc.Len() {
			t.Errorf("ParseSource().Len() = %v, want %v", gotSrc.Len(), tt.wantSrc.Len())
		}

		if gotSrc.String() != tt.wantSrc.String() {
			t.Errorf("ParseSource().String() = %v, want %v", gotSrc.String(), tt.wantSrc.String())
		}

		if gotSrc.IsServer() != tt.wantSrc.IsServer() {
			t.Errorf("ParseSource().IsServer() = %v, want %v", gotSrc.IsServer(), tt.wantSrc.IsServer())
		}

		if gotSrc.IsHostmask() != tt.wantSrc.IsHostmask() {
			t.Errorf("ParseSource().IsHostmask() = %v, want %v", gotSrc.IsHostmask(), tt.wantSrc.IsHostmask())
		}

		if !reflect.DeepEqual(gotSrc.Bytes(), tt.wantSrc.Bytes()) {
			t.Errorf("ParseSource().Bytes() = %v, want %v", gotSrc, tt.wantSrc)
		}
	}
}

var testsParseEvent = []struct {
	in   string
	want string
}{
	{in: "", want: ""},
	{in: ":host.domain.com TEST", want: ":host.domain.com TEST"},
	{in: ":host.domain.com TEST\r\n", want: ":host.domain.com TEST"},
	{in: ":host.domain.com TEST arg1 arg2", want: ":host.domain.com TEST arg1 arg2"},
	{in: ":host.domain.com TEST :", want: ":host.domain.com TEST :"},
	{in: ":host.domain.com TEST ::", want: ":host.domain.com TEST ::"},
	{in: ":host.domain.com TEST :test1", want: ":host.domain.com TEST test1"},
	{in: ":host.domain.com TEST :test:test", want: ":host.domain.com TEST test:test"},
	{in: ":host.domain.com TEST :test1 :test", want: ":host.domain.com TEST :test1 :test"},
	{in: ":host.domain.com TEST :test1 test2", want: ":host.domain.com TEST :test1 test2"},
	{in: ":host.domain.com TEST arg1 arg2 :test1", want: ":host.domain.com TEST arg1 arg2 test1"},
	{in: ":host.domain.com TEST arg1 arg=:10 :test1", want: ":host.domain.com TEST arg1 arg=:10 test1"},
	{in: ":nick!user@host TEST :test1", want: ":nick!user@host TEST test1"},
	{in: ":nick!user@host TEST :test1 test2", want: ":nick!user@host TEST :test1 test2"},
	{in: "@aaa=bbb :nick!user@host TEST :test1", want: "@aaa=bbb :nick!user@host TEST test1"},
	{in: "@aaa=bbb;+ccc;example.com/ddd=eee :nick!user@host TEST :test1", want: "@aaa=bbb;+ccc;example.com/ddd=eee :nick!user@host TEST test1"},
	{in: "@bbb=aaa;aaa :nick!user@host TEST :test1 test2", want: "@aaa;bbb=aaa :nick!user@host TEST :test1 test2"},
}

func FuzzParseEvent(f *testing.F) {
	for _, tc := range testsParseEvent {
		f.Add(tc.in)
	}

	for _, tc := range testsIRCDocs {
		f.Add(tc)
	}

	f.Fuzz(func(t *testing.T, orig string) {
		got := ParseEvent(orig)

		if got == nil {
			return
		}

		_ = got.IsAction()
		_ = got.IsFromChannel()
		_ = got.IsFromUser()
		_ = got.Len()
		_, _ = got.IsCTCP()

		if utf8.ValidString(orig) {
			if !utf8.ValidString(got.Command) {
				t.Errorf("produced invalid UTF-8 string %q", got.Command)
			}

			if !utf8.ValidString(got.Last()) {
				t.Errorf("produced invalid UTF-8 string %q", got.Last())
			}

			if !utf8.ValidString(got.String()) {
				t.Errorf("produced invalid UTF-8 string %q", got.String())
			}

			if !utf8.ValidString(got.StripAction()) {
				t.Errorf("produced invalid UTF-8 string %q", got.StripAction())
			}

			if !utf8.Valid(got.Bytes()) {
				t.Errorf("produced invalid UTF-8 []byte %q", got.Bytes())
			}
		}
	})
}

func TestParseEvent(t *testing.T) {
	for _, tt := range testsParseEvent {
		got := ParseEvent(tt.in)

		if got == nil && tt.want == "" {
			continue
		}

		if got == nil {
			t.Errorf("ParseEvent: got nil, want: %s", tt.want)
		}

		if got.String() != tt.want {
			if got.Tags != nil {
				if len(got.String()) != len(tt.want) {
					t.Fatalf("ParseEvent: length exception in tag parse: got %q, want %q", got.String(), tt.want)
				}
			} else {
				t.Fatalf("ParseEvent: got %q, want %q", got.String(), tt.want)
			}
		}

		if got.Len() != len(tt.want) {
			if got.Len() > 510 {
				continue
			}
			t.Fatalf("Event.Len: got %d from %q, want %d", got.Len(), got.String(), len(tt.want))
		}
	}
}

func TestEventCopy(t *testing.T) {
	var nilEvent *Event

	if event := nilEvent.Copy(); event != nil {
		t.Fatalf("Event.Copy: returned non-nil on nil event: %#v", event)
	}

	msg := "@aaa=bbb;ccc;example.com/ddd=eee :nick!user@host TEST arg1 arg2 :test1"
	event := ParseEvent(msg)

	eventCopy := event.Copy()

	if !reflect.DeepEqual(event, eventCopy) {
		t.Fatalf("Event.Copy: want %#v, got %#v", event, eventCopy)
	}

	// Since Event.Copy() calls Source.Copy()...
	if !reflect.DeepEqual(event.Source, eventCopy.Source) {
		t.Fatalf("Source.Copy: want %#v, got %#v", event.Source, eventCopy.Source)
	}

	event.Source = nil
	if src := event.Source.Copy(); src != nil {
		t.Fatalf("Source.Copy: returned non-nil on nil source: %#v", src)
	}
}

func TestEventIs(t *testing.T) {
	event := ParseEvent(":nick!user@host PRIVMSG #test :\x01ACTION this is a test\x01")

	if !event.IsAction() {
		t.Fatalf("Event.IsAction: returned false on %#v", event)
	}
	event.Command = "TEST"
	if event.IsAction() {
		t.Fatalf("Event.IsAction: returned true though not privmsg;  %#v", event)
	}
	event.Command = "PRIVMSG"

	event.Params[len(event.Params)-1] = event.StripAction()
	if event.IsAction() || event.Last() != "this is a test" {
		t.Fatalf("Event.IsAction: returned true on %#v or trailing is not \"this is a test\": %q", event, event.Last())
	}

	if !event.IsFromChannel() {
		t.Fatalf("Event.IsFromChannel: returned false on %#v", event)
	}

	event.Command = "TEST"
	if event.IsFromChannel() {
		t.Fatalf("Event.IsFromChannel: returned true though not privmsg; %#v", event)
	}

	event.Params[0] = "user1"
	if event.IsFromUser() {
		t.Fatalf("Event.IsFromUser: returned true when not privmsg; %#v", event)
	}

	event.Command = "PRIVMSG"
	if !event.IsFromUser() {
		t.Fatalf("Event.IsFromUser: returned false on %#v", event)
	}
}

func TestEventSourceTagEquals(t *testing.T) {
	// This should test events themselves, as well as tags and sources.
	cases := []struct {
		before, after string
		equals        bool
	}{
		{
			before: ":nick!user@host PRIVMSG #test :This is a test",
			after:  ":nick!user@host PRIVMSG #test :This is a test",
			equals: true,
		},
		{
			before: ":nick!user@host PRIVMSG #test :This is a test",
			after:  ":nick!user@host1 PRIVMSG #test :This is a test",
			equals: false,
		},
		{
			before: ":nick!user@host PRIVMSG #test :This is a test",
			after:  ":nick!user@host PRIVMSG #tes :This is a test",
			equals: false,
		},
		{
			before: "@aaa=bbb;ccc;example.com/ddd=eee :nick!user@host PRIVMSG #test :This is a test",
			after:  "@aaa=bbb;ccc;example.com/ddd=eee :nick!user@host PRIVMSG #test :This is a test",
			equals: true,
		},
		{
			before: "@aaa=bbb;ccc;example.com/ddd=eee :nick!user@host PRIVMSG #test :This is a test",
			after:  "@aaa=bbb;ccc :nick!user@host PRIVMSG #test :This is a test",
			equals: true,
		},
		{
			before: ":nick!user@host PRIVMSG #test :This is a test",
			after:  "@aaa=bbb;ccc :nick!user@host PRIVMSG #test :This is a test",
			equals: true,
		},
		{
			before: "@account=bbb;ccc :nick!user@host PRIVMSG #test :This is a test",
			after:  "@aaa=bbb;ccc :nick!user@host PRIVMSG #test :This is a test",
			equals: false,
		},
	}

	for _, tt := range cases {
		before := ParseEvent(tt.before)
		after := ParseEvent(tt.after)
		equals := before.Equals(after)

		// It should be equal the opposite direction too.
		if op := after.Equals(before); equals != op {
			t.Fatalf("Event.Equals reverse order doesn't match forward order. before: %#v, after: %#v", before, after)
		}

		if equals != tt.equals {
			t.Fatalf("Event.Equals: returned %t (wanted %t) on copied event. before: %#v, after: %#v", equals, tt.equals, before, after)
		}
	}
}

// // Some of these are pulled from https://github.com/ircdocs/parser-tests.
var testsIRCDocs = []string{
	"foo bar baz asdf",
	"foo bar baz :asdf",
	":src AWAY",
	":src AWAY :",
	":coolguy foo bar baz asdf",
	":coolguy foo bar baz :asdf",
	"foo bar baz :asdf quux",
	"foo bar baz :",
	"foo bar baz ::asdf",
	":coolguy foo bar baz :asdf quux",
	":coolguy foo bar baz :  asdf quux ",
	":coolguy PRIVMSG bar :lol :) ",
	":coolguy foo bar baz :",
	":coolguy foo bar baz :  ",
	":coolguy foo b\tar baz",
	":coolguy foo b\tar :baz",
	"@asd :coolguy foo bar baz :  ",
	"@a=b\\\\and\\nk;d=gh\\:764 foo",
	"@d=gh\\:764;a=b\\\\and\\nk foo",
	"@a=b\\\\and\\nk;d=gh\\:764 foo par1 par2",
	"@a=b\\\\and\\nk;d=gh\\:764 foo par1 :par2",
	"@d=gh\\:764;a=b\\\\and\\nk foo par1 par2",
	"@d=gh\\:764;a=b\\\\and\\nk foo par1 :par2",
	"@foo=\\\\\\\\\\:\\\\s\\s\\r\\n COMMAND",
	"foo bar baz asdf",
	":coolguy foo bar baz asdf",
	"foo bar baz :asdf quux",
	"foo bar baz :",
	"foo bar baz ::asdf",
	":coolguy foo bar baz :asdf quux",
	":coolguy foo bar baz :  asdf quux ",
	":coolguy PRIVMSG bar :lol :) ",
	":coolguy foo bar baz :",
	":coolguy foo bar baz :  ",
	"@a=b;c=32;k;rt=ql7 foo",
	"@a=b\\\\and\\nk;c=72\\s45;d=gh\\:764 foo",
	"@c;h=;a=b :quux ab cd",
	":src JOIN #chan",
	":src JOIN :#chan",
	":src AWAY",
	":src AWAY ",
	":cool\tguy foo bar baz",
	":coolguy!ag@net\x035w\x03ork.admin PRIVMSG foo :bar baz",
	":coolguy!~ag@n\x02et\x0305w\x0fork.admin PRIVMSG foo :bar baz",
	"@tag1=value1;tag2;vendor1/tag3=value2;vendor2/tag4= :irc.example.com COMMAND param1 param2 :param3 param3",
	":irc.example.com COMMAND param1 param2 :param3 param3",
	"@tag1=value1;tag2;vendor1/tag3=value2;vendor2/tag4 COMMAND param1 param2 :param3 param3",
	"COMMAND",
	"@foo=\\\\\\\\\\:\\\\s\\s\\r\\n COMMAND",
	":gravel.mozilla.org 432  #momo :Erroneous Nickname: Illegal characters",
	":gravel.mozilla.org MODE #tckk +n ",
	":services.esper.net MODE #foo-bar +o foobar  ",
	"@tag1=value\\\\ntest COMMAND",
	"@tag1=value\\1 COMMAND",
	"@tag1=value1\\ COMMAND",
	"@tag1=1;tag2=3;tag3=4;tag1=5 COMMAND",
	"@tag1=1;tag2=3;tag3=4;tag1=5;vendor/tag2=8 COMMAND",
	":SomeOp MODE #channel :+i",
	":SomeOp MODE #channel +oo SomeUser :AnotherUser",
}

func TestEventIRCDocsParseTests(t *testing.T) {
	for _, tt := range testsIRCDocs {
		// Basic test to just verify it doesn't panic.
		_ = ParseEvent(tt)
	}
}
