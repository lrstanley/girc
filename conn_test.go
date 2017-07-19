// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"bytes"
	"net"
	"testing"
	"time"
)

func TestNewConn(t *testing.T) {
	conf := Config{Server: "", Port: 6667, Nick: "nick", User: "user", Name: "realname"}
	conn, err := newConn(conf, conf.Server+":6667")
	if err == nil {
		t.Fatal("invalid server but no error")
	}
	conf.Server = "irc.byteirc.org"
	conn, err = newConn(conf, conf.Server+":6667")
	if err != nil {
		t.Fatal(err)
	}
	if !conn.connected {
		t.Fatal("conn provided but not connected")
	}
	conn.Close()
}

func mockBuffers() (in *bytes.Buffer, out *bytes.Buffer, irc *ircConn) {
	in = &bytes.Buffer{}
	out = &bytes.Buffer{}
	irc = &ircConn{
		io:        bufio.NewReadWriter(bufio.NewReader(in), bufio.NewWriter(out)),
		connected: true,
	}

	return in, out, irc
}

func TestDecode(t *testing.T) {
	in, _, c := mockBuffers()

	e := mockEvent()

	in.Write(e.Bytes())
	in.Write(endline)

	event, err := c.decode()
	if err != nil {
		t.Fatalf("received error during decode: %s", err)
	}

	if event.String() != e.String() {
		t.Fatalf("event returned from decode not the same as mock event. want %#v, got %#v", e, event)
	}

	// Test a failure.
	in.WriteString("::abcd\r\n")
	event, err = c.decode()
	if err == nil {
		t.Fatalf("should have failed to parse decoded event. got: %#v", event)
	}

	return
}

func TestEncode(t *testing.T) {
	_, out, c := mockBuffers()

	e := mockEvent()

	err := c.encode(e)
	if err != nil {
		t.Fatalf("received error during encode: %s", err)
	}

	line, err := out.ReadString(delim)
	if err != nil {
		t.Fatalf("received error during check for encoded event: %s", err)
	}

	want := e.String() + "\r\n"

	if want != line {
		t.Fatalf("encoded line wanted: %q, got: %q", want, line)
	}

	return
}

func TestRate(t *testing.T) {
	_, _, c := mockBuffers()
	c.lastWrite = time.Now()
	if delay := c.rate(100); delay > time.Second {
		t.Fatal("first instance of rate is > second")
	}

	for i := 0; i < 500; i++ {
		c.rate(200)
	}

	if delay := c.rate(200); delay > (3 * time.Second) {
		t.Fatal("rate delay too high")
	}

	return
}

func genMockConn() (client *Client, clientConn net.Conn, serverConn net.Conn) {
	client = New(Config{
		Server: "dummy.int",
		Port:   6667,
		Nick:   "test",
		User:   "test",
		Name:   "Testing123",
	})

	conn1, conn2 := net.Pipe()

	return client, conn1, conn2
}

func TestConnect(t *testing.T) {
	c, conn, server := genMockConn()
	b := bufio.NewReader(conn)

	defer conn.Close()
	defer server.Close()

	go c.MockConnect(server)
	defer c.Close()

	var counter int
	var events []*Event

	for {
		counter++

		if counter > 3 {
			break
		}

		conn.SetReadDeadline(time.Now().Add(time.Second))
		out, err := b.ReadString(byte('\n'))
		if err != nil {
			panic(err)
		}

		events = append(events, ParseEvent(out))
	}

	if len(events) < 3 {
		t.Fatal("TestConnect returned less than 3 initial events during connect")
	}

	// The events should at least consist of:
	// NICK test
	// USER test +iw * :Testing123
	// CAP LS 302

	if events[0].Command != "NICK" || events[0].Params[0] != c.Config.Nick {
		t.Fatalf("TestConnect: invalud nick command: %#v", events[0])
	}

	if events[1].Command != "USER" || events[1].Params[0] != c.Config.User || events[1].Trailing != c.Config.Name {
		t.Fatalf("TestConnect: invalid user command: %#v", events[1])
	}

	if c.Config.disableTracking {
		if events[1].Command != "CAP" || events[1].Params[0] != "LS" {
			t.Fatalf("TestConnect: invalud cap command: %#v", events[1])
		}
	}
}
