// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"bytes"
	"net"

	"sync/atomic"
	"testing"
	"time"
)

func mockBuffers() (in *bytes.Buffer, out *bytes.Buffer, irc *ircConn) {
	in = &bytes.Buffer{}
	out = &bytes.Buffer{}
	irc = &ircConn{
		io:        bufio.NewReadWriter(bufio.NewReader(in), bufio.NewWriter(out)),
		connected: atomic.Value{},
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
	c.lastWrite.Store(time.Now())
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

func mockReadBuffer(conn net.Conn) {
	// Accept all outgoing writes from the client.
	b := bufio.NewReader(conn)
	for {
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		_, err := b.ReadString(byte('\n'))
		if err != nil {
			return
		}
	}
}
