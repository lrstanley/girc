// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

var testsEncodeCTCP = []struct {
	name string
	test *CTCPEvent
	want string
}{
	{name: "command only", test: &CTCPEvent{Command: "TEST", Text: ""}, want: "\001TEST\001"},
	{name: "command with args", test: &CTCPEvent{Command: "TEST", Text: "TEST"}, want: "\001TEST TEST\001"},
	{name: "nil command", test: &CTCPEvent{Command: "", Text: "TEST"}, want: ""},
	{name: "nil event", test: nil, want: ""},
}

func FuzzEncodeCTCP(f *testing.F) {
	for _, tc := range testsEncodeCTCP {
		if tc.test == nil {
			continue
		}
		f.Add(tc.test.Command, tc.test.Text)
	}

	f.Fuzz(func(t *testing.T, cmd, text string) {
		got := EncodeCTCP(&CTCPEvent{Command: cmd, Text: text})

		if utf8.ValidString(cmd) && utf8.ValidString(text) && !utf8.ValidString(got) {
			t.Errorf("produced invalid UTF-8 string %q", got)
		}
	})
}

func TestEncodeCTCP(t *testing.T) {
	for _, tt := range testsEncodeCTCP {
		if got := EncodeCTCP(tt.test); got != tt.want {
			t.Errorf("%s: encodeCTCP() = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNewCTCP(t *testing.T) {
	ctcp := newCTCP()

	if ctcp == nil {
		t.Fatalf("newCTCP() = nil, wanted *CTCP")
	}
}

func TestDecodeCTCP(t *testing.T) {
	type args struct {
		event *Event
	}

	tests := []struct {
		name string
		args args
		want *CTCPEvent
	}{
		{name: "non-ctcp", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "this is a test"}},
		}, want: nil},
		{name: "empty trailing", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", ""}},
		}, want: nil},
		{name: "too many args", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "user2", "this is a test"}},
		}, want: nil},
		{name: "missing delim", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "\001TEST this is a test"}},
		}, want: nil},
		{name: "missing delim", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "TEST this is a test\001"}},
		}, want: nil},
		{name: "invalid command", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "\001TEST-1 this is a test\001"}},
		}, want: nil},
		{name: "invalid command", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "\001TEST-1\001"}},
		}, want: nil},
		{name: "is reply", args: args{event: &Event{
			Command: "NOTICE", Params: []string{"user1", "\001TEST this is a test\001"}},
		}, want: &CTCPEvent{Command: "TEST", Text: "this is a test", Reply: true}},
		{name: "is reply, tag only", args: args{event: &Event{
			Command: "NOTICE", Params: []string{"user1", "\001TEST\001"}},
		}, want: &CTCPEvent{Command: "TEST", Text: "", Reply: true}},
		{name: "is reply", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "\001TEST\001"}},
		}, want: &CTCPEvent{Command: "TEST", Text: ""}},
		{name: "has args", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "\001TEST 1 2 3 4\001"}},
		}, want: &CTCPEvent{Command: "TEST", Text: "1 2 3 4"}},
		{name: "has args", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "\001TEST :1 2 3 4\001"}},
		}, want: &CTCPEvent{Command: "TEST", Text: ":1 2 3 4"}},
	}

	for _, tt := range tests {
		got := DecodeCTCP(tt.args.event)
		if got != nil {
			got.Origin = tt.want.Origin
		}

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: decodeCTCP() = %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

func TestCall(t *testing.T) {
	var counter uint64
	ctcp := newCTCP()

	ctcp.Set("TEST", func(client *Client, event CTCPEvent) {
		atomic.AddUint64(&counter, 1)
	})

	ctcp.call(New(Config{}), &CTCPEvent{Command: "TEST"})
	if atomic.LoadUint64(&counter) != 1 {
		t.Fatal("regular execution: call() didn't increase counter")
	}
	ctcp.Clear("TEST")

	ctcp.SetBg("TEST", func(client *Client, event CTCPEvent) {
		atomic.AddUint64(&counter, 1)
	})

	ctcp.call(New(Config{}), &CTCPEvent{Command: "TEST"})
	time.Sleep(250 * time.Millisecond)
	if atomic.LoadUint64(&counter) != 2 {
		t.Fatal("goroutine execution: call() in goroutine didn't increase counter")
	}
	ctcp.Clear("TEST")

	ctcp.Set("*", func(client *Client, event CTCPEvent) {
		atomic.AddUint64(&counter, 1)
	})

	ctcp.call(New(Config{}), &CTCPEvent{Command: "TEST"})
	if atomic.LoadUint64(&counter) != 3 {
		t.Fatal("wildcard execution: call() didn't increase counter")
	}
	ctcp.Clear("*")
	ctcp.Clear("TEST")

	ctcp.call(New(Config{}), &CTCPEvent{Command: "TEST"})
	if atomic.LoadUint64(&counter) != 3 {
		t.Fatal("empty execution: call() with no handler incremented the counter")
	}
}

func TestSet(t *testing.T) {
	ctcp := newCTCP()

	ctcp.Set("TEST-1", func(client *Client, event CTCPEvent) {})
	if _, ok := ctcp.handlers["TEST"]; ok {
		t.Fatal("Set('TEST') allowed invalid command")
	}

	ctcp.Set("TEST", func(client *Client, event CTCPEvent) {})
	// Make sure it's there.
	if _, ok := ctcp.handlers["TEST"]; !ok {
		t.Fatal("store: Set('TEST') didn't set")
	}
}

func TestClear(t *testing.T) {
	ctcp := newCTCP()

	ctcp.Set("TEST", func(client *Client, event CTCPEvent) {})
	ctcp.Clear("TEST")

	if _, ok := ctcp.handlers["TEST"]; ok {
		t.Fatal("ctcp.Clear('TEST') didn't remove handler")
	}
}

func TestClearAll(t *testing.T) {
	ctcp := newCTCP()

	ctcp.Set("TEST1", func(client *Client, event CTCPEvent) {})
	ctcp.Set("TEST2", func(client *Client, event CTCPEvent) {})
	ctcp.ClearAll()

	_, first := ctcp.handlers["TEST1"]
	_, second := ctcp.handlers["TEST2"]

	if first || second {
		t.Fatalf("ctcp.ClearAll() didn't remove all handlers: 1: %v 2: %v", first, second)
	}
}
