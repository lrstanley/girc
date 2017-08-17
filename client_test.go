// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"context"
	"io"
	"strings"
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

func TestConfigValid(t *testing.T) {
	conf := Config{
		Server: "irc.example.com", Port: 6667,
		Nick: "test", User: "test", Name: "Realname",
	}

	var err error
	if err = conf.isValid(); err != nil {
		t.Fatalf("valid config failed Config.isValid() with: %s", err)
	}

	conf.Server = ""
	if err = conf.isValid(); err == nil {
		t.Fatalf("invalid server passed validation check: %s", err)
	}
	conf.Server = "irc.example.com"

	conf.Port = 100000
	if err = conf.isValid(); err == nil {
		t.Fatalf("invalid port passed validation check: %s", err)
	}
	conf.Port = 0 // Assumes "default".
	if err = conf.isValid(); err != nil {
		t.Fatalf("valid default failed validation check: %s", err)
	}
	if conf.Port != 6667 {
		t.Fatal("irc port was not defaulted to 6667")
	}

	conf.Nick = "invalid nick"
	if err = conf.isValid(); err == nil {
		t.Fatalf("invalid nick passed validation check: %s", err)
	}
	conf.User = "test"

	conf.User = "invalid user"
	if err = conf.isValid(); err == nil {
		t.Fatalf("invalid user passed validation check: %s", err)
	}
	conf.User = "test"
}

func TestClientLifetime(t *testing.T) {
	client := New(Config{
		Server: "dummy.int",
		Port:   6667,
		Nick:   "test",
		User:   "test",
		Name:   "Testing123",
	})

	tm := client.Lifetime()

	if tm < 0 || tm > 2*time.Second {
		t.Fatalf("Client.Lifetime() = %q, out of bounds", tm)
	}
}

func TestClientUptime(t *testing.T) {
	c, conn, server := genMockConn()
	defer conn.Close()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	c.Handlers.Add(INITIALIZED, func(c *Client, e Event) {
		cancel()
	})

	go c.MockConnect(server)
	defer c.Close()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Client.Uptime() timed out")
	}

	uptime, err := c.Uptime()
	if err != nil {
		t.Fatalf("Client.Uptime() = %s, wanted time", err)
	}

	since := time.Since(*uptime)
	connsince, err := c.ConnSince()
	if err != nil {
		t.Fatalf("Client.ConnSince() = %s, wanted time", err)
	}

	if since < 0 || since > 4*time.Second || *connsince < 0 || *connsince > 4*time.Second {
		t.Fatalf("Client.Uptime() = %q (%q, connsince: %q), out of bounds", uptime, since, connsince)
	}

	// Verify the time we got from Client.Uptime() and Client.ConnSince() are
	// within reach of eachother.

	if *connsince-since > 2*time.Second {
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

	ctx, cancel := context.WithCancel(context.Background())
	c.Handlers.Add(INITIALIZED, func(c *Client, e Event) {
		cancel()
	})

	go c.MockConnect(server)
	defer c.Close()

	select {
	case <-ctx.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("timed out during connect")
	}

	if nick := c.GetNick(); nick != c.Config.Nick {
		t.Fatalf("Client.GetNick() = %q though should be %q", nick, c.Config.Nick)
	}

	if user := c.GetIdent(); user != c.Config.User {
		t.Fatalf("Client.GetIdent() = %q though should be %q", user, c.Config.User)
	}

	if !strings.Contains(c.String(), "connected:true") {
		t.Fatalf("Client.String() == %q, doesn't contain 'connected:true'", c.String())
	}
}

func TestClientClose(t *testing.T) {
	c, conn, server := genMockConn()
	defer server.Close()
	defer conn.Close()

	errchan := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())

	c.Handlers.AddBg(STOPPED, func(c *Client, e Event) {
		cancel()
	})
	c.Handlers.AddBg(INITIALIZED, func(c *Client, e Event) {
		c.Close()
	})

	go func() {
		errchan <- c.MockConnect(server)
	}()
	defer c.Close()

	select {
	case <-time.After(5 * time.Second):
		t.Fatal("Client.Close() timed out")
		cancel()
	case <-ctx.Done():
	}

	select {
	case err := <-errchan:
		if err != nil && err != io.ErrClosedPipe {
			t.Fatalf("connect returned with error when close was invoked: %s", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out while waiting for connect to return")
	}

	close(errchan)
}
