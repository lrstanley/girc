// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"reflect"
	"testing"
)

func mockEvent() *Event {
	return &Event{
		Source:   &Source{Name: "nick", Ident: "user", Host: "host.com"},
		Command:  "PRIVMSG",
		Params:   []string{"#channel"},
		Trailing: "1 2 3",
	}
}

func TestParseSource(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name    string
		args    args
		wantSrc *Source
	}{
		{name: "full", args: args{raw: "nick!user@hostname.com"}, wantSrc: &Source{
			Name: "nick", Ident: "user", Host: "hostname.com",
		}},
		{name: "special chars", args: args{raw: "^[]nick!~user@test.host---name.com"}, wantSrc: &Source{
			Name: "^[]nick", Ident: "~user", Host: "test.host---name.com",
		}},
		{name: "short", args: args{raw: "a!b@c"}, wantSrc: &Source{
			Name: "a", Ident: "b", Host: "c",
		}},
		{name: "short", args: args{raw: "a!b"}, wantSrc: &Source{
			Name: "a", Ident: "b", Host: "",
		}},
		{name: "short", args: args{raw: "a@b"}, wantSrc: &Source{
			Name: "a", Ident: "", Host: "b",
		}},
		{name: "short", args: args{raw: "test"}, wantSrc: &Source{
			Name: "test", Ident: "", Host: "",
		}},
	}
	for _, tt := range tests {
		gotSrc := ParseSource(tt.args.raw)

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

func TestParseEvent(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: ":host.domain.com TEST", want: ":host.domain.com TEST"},
		{in: ":host.domain.com TEST\r\n", want: ":host.domain.com TEST"},
		{in: ":host.domain.com TEST arg1 arg2", want: ":host.domain.com TEST arg1 arg2"},
		{in: ":host.domain.com TEST :", want: ":host.domain.com TEST :"},
		{in: ":host.domain.com TEST :test1", want: ":host.domain.com TEST :test1"},
		{in: ":host.domain.com TEST arg1 arg2 :test1", want: ":host.domain.com TEST arg1 arg2 :test1"},
		{in: ":host.domain.com TEST arg1 arg=:10 :test1", want: ":host.domain.com TEST arg1 arg=:10 :test1"},
		{in: ":nick!user@host TEST :test1", want: ":nick!user@host TEST :test1"},
		{in: ":nick!user@host TEST :test0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000LONG TEXT TRUNCATED HERE", want: ":nick!user@host TEST :test0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{in: "@aaa=bbb;ccc;example.com/ddd=eee :nick!user@host TEST :test0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000LONG TEXT TRUNCATED HERE", want: "@aaa=bbb;ccc;example.com/ddd=eee :nick!user@host TEST :test0000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"},
		{in: "@aaa=bbb :nick!user@host TEST :test1", want: "@aaa=bbb :nick!user@host TEST :test1"},
		{in: "@aaa=bbb;+ccc;example.com/ddd=eee :nick!user@host TEST :test1", want: "@aaa=bbb;+ccc;example.com/ddd=eee :nick!user@host TEST :test1"},
		{in: "@bbb=aaa;aaa :nick!user@host TEST :test1", want: "@aaa;bbb=aaa :nick!user@host TEST :test1"},
	}

	for _, tt := range tests {
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

	event.Trailing = event.StripAction()
	if event.IsAction() || event.Trailing != "this is a test" {
		t.Fatalf("Event.IsAction: returned true on %#v", event)
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
