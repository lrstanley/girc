// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"bytes"
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

func TestFlushTx(t *testing.T) {
	c := &Client{tx: make(chan *Event, 50)}

	for i := 0; i < 25; i++ {
		c.tx <- &Event{}
	}

	c.flushTx()
	if len(c.tx) > 0 {
		t.Fatalf("flush failed too flush all events: %d remaining", len(c.tx))
	}
}
