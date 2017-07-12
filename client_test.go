// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"testing"
	"time"
)

func TestDisableTracking(t *testing.T) {
	client := New(Config{
		Server: "dummy.int",
		Port:   6667,
		Nick:   "test",
		User:   "test",
		Name:   "Testing123",
	})

	if len(client.Handlers.internal) < 1 {
		t.Fatal("Client.Handlers empty, though just initialized")
	}

	client.DisableTracking()
	if _, ok := client.Handlers.internal[CAP]; ok {
		t.Fatal("Client.Handlers contains capability tracking handlers, though disabled")
	}

	client.state.Lock()
	defer client.state.Unlock()

	if client.state.channels != nil {
		t.Fatal("Client.DisableTracking() called but channel state still exists")
	}
}

func TestClientLifetime(t *testing.T) {
	client := New(Config{
		Server: "dummy.int",
		Port:   6667,
		Nick:   "test",
		User:   "test",
		Name:   "Testing123",
	})

	time.Sleep(500 * time.Millisecond)
	tm := client.Lifetime()

	if tm < 400*time.Millisecond || tm > 10*time.Second {
		t.Fatalf("Client.Lifetime() = %q, out of bounds", tm)
	}
}

func TestClientUptime(t *testing.T) {
	c, conn, server := genMockConn()

	defer conn.Close()
	defer server.Close()

	go c.MockConnect(server)
	defer c.Close()

	time.Sleep(500 * time.Millisecond)

	uptime, err := c.Uptime()
	if err != nil {
		t.Fatalf("Client.Uptime() = %s, wanted time", err)
	}

	since := time.Since(*uptime)
	connsince, err := c.ConnSince()
	if err != nil {
		t.Fatalf("Client.ConnSince() = %s, wanted time", err)
	}

	if since < 400*time.Millisecond || since > 10*time.Second || *connsince < 400*time.Millisecond || *connsince > 10*time.Second {
		t.Fatalf("Client.Uptime() = %q (%q, connsince: %q), out of bounds", uptime, since, connsince)
	}

	// Verify the time we got from Client.Uptime() and Client.ConnSince() are
	// within reach of eachother.

	if *connsince-since > 1*time.Second {
		t.Fatalf("Client.Uptime() (diff) = %q, Client.ConnSince() = %q, differ too much", since, connsince)
	}

	if !c.IsConnected() {
		t.Fatal("Client.IsConnected() = false, though mock should be true")
	}
}

func TestClientGet(t *testing.T) {
	c, conn, server := genMockConn()

	defer conn.Close()
	defer server.Close()

	go c.MockConnect(server)
	defer c.Close()

	time.Sleep(1 * time.Second)

	if nick := c.GetNick(); nick != c.Config.Nick {
		t.Fatalf("Client.GetNick() = %q though should be %q", nick, c.Config.Nick)
	}

	if user := c.GetIdent(); user != c.Config.User {
		t.Fatalf("Client.GetIdent() = %q though should be %q", user, c.Config.User)
	}
}
