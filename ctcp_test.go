// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"reflect"
	"testing"
	"time"
)

func TestEncodeCTCP(t *testing.T) {
	type args struct {
		ctcp *CTCPEvent
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{name: "command only", args: args{ctcp: &CTCPEvent{Command: "TEST", Text: ""}}, want: "\001TEST\001"},
		{name: "command with args", args: args{ctcp: &CTCPEvent{Command: "TEST", Text: "TEST"}}, want: "\001TEST TEST\001"},
		{name: "nil command", args: args{ctcp: &CTCPEvent{Command: "", Text: "TEST"}}, want: ""},
		{name: "nil event", args: args{ctcp: nil}, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := encodeCTCP(tt.args.ctcp); got != tt.want {
				t.Errorf("encodeCTCP() = %q, want %q", got, tt.want)
			}
		})
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
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "this is a test"},
		}, want: nil},
		{name: "empty trailing", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: ""},
		}, want: nil},
		{name: "too many args", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1", "user2"}, Trailing: "this is a test"},
		}, want: nil},
		{name: "missing delim", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "\001TEST this is a test"},
		}, want: nil},
		{name: "missing delim", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "TEST this is a test\001"},
		}, want: nil},
		{name: "invalid command", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "\001TEST-1 this is a test\001"},
		}, want: nil},
		{name: "invalid command", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "\001TEST-1\001"},
		}, want: nil},
		{name: "invalid nick param", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"!user1"}, Trailing: "\001TEST this is a test\001"},
		}, want: nil},
		{name: "is reply", args: args{event: &Event{
			Command: "NOTICE", Params: []string{"user1"}, Trailing: "\001TEST this is a test\001"},
		}, want: &CTCPEvent{Command: "TEST", Text: "this is a test", Reply: true}},
		{name: "is reply, tag only", args: args{event: &Event{
			Command: "NOTICE", Params: []string{"user1"}, Trailing: "\001TEST\001"},
		}, want: &CTCPEvent{Command: "TEST", Text: "", Reply: true}},
		{name: "is reply", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "\001TEST\001"},
		}, want: &CTCPEvent{Command: "TEST", Text: ""}},
		{name: "has args", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "\001TEST 1 2 3 4\001"},
		}, want: &CTCPEvent{Command: "TEST", Text: "1 2 3 4"}},
		{name: "has args", args: args{event: &Event{
			Command: "PRIVMSG", Params: []string{"user1"}, Trailing: "\001TEST :1 2 3 4\001"},
		}, want: &CTCPEvent{Command: "TEST", Text: ":1 2 3 4"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeCTCP(tt.args.event)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("decodeCTCP() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCall(t *testing.T) {
	counter := 0
	ctcp := newCTCP()

	t.Run("regular execution", func(t *testing.T) {
		ctcp.Set("TEST", func(client *Client, event CTCPEvent) {
			counter++
		})

		if ctcp.call(&CTCPEvent{Command: "TEST"}, New(Config{})); counter != 1 {
			t.Fatal("call() didn't increase counter")
		}
		ctcp.Clear("TEST")
	})

	t.Run("goroutine execution", func(t *testing.T) {
		ctcp.SetBg("TEST", func(client *Client, event CTCPEvent) {
			counter++
		})

		ctcp.call(&CTCPEvent{Command: "TEST"}, New(Config{}))
		if time.Sleep(250 * time.Millisecond); counter != 2 {
			t.Fatal("call() in goroutine didn't increase counter")
		}
		ctcp.Clear("TEST")
	})

	t.Run("wildcard execution", func(t *testing.T) {
		ctcp.Set("*", func(client *Client, event CTCPEvent) {
			counter++
		})

		if ctcp.call(&CTCPEvent{Command: "TEST"}, New(Config{})); counter != 3 {
			t.Fatal("call() didn't increase counter")
		}
		ctcp.Clear("*")
	})

	t.Run("empty execution", func(t *testing.T) {
		ctcp.Clear("TEST")

		if ctcp.call(&CTCPEvent{Command: "TEST"}, New(Config{})); counter != 3 {
			t.Fatal("call() with no handler incremented the counter")
		}
	})
}

func TestSet(t *testing.T) {
	ctcp := newCTCP()

	t.Run("invalid command", func(t *testing.T) {
		ctcp.Set("TEST-1", func(client *Client, event CTCPEvent) {})
		if _, ok := ctcp.handlers["TEST"]; ok {
			t.Fatal("Set('TEST') allowed invalid command")
		}
	})

	t.Run("store", func(t *testing.T) {
		ctcp.Set("TEST", func(client *Client, event CTCPEvent) {})
		// Make sure it's there.
		if _, ok := ctcp.handlers["TEST"]; !ok {
			t.Fatal("Set('TEST') didn't set")
		}
	})
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
