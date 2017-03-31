// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc_test

import (
	"log"
	"os"
	"strings"

	"github.com/lrstanley/girc"
)

// The bare-minimum needed to get started with girc. Just connects and idles.
func Example_bare() {
	client := girc.New(girc.Config{
		Server: "irc.byteirc.org",
		Port:   6667,
		Nick:   "test",
		User:   "user",
		Debug:  os.Stdout,
	})

	log.Fatal(client.Connect())
}

// Very simple example that connects, joins a channel, and responds to
// "hello" with "hello world!".
func Example_simple() {
	client := girc.New(girc.Config{
		Server: "irc.byteirc.org",
		Port:   6667,
		Nick:   "test",
		User:   "user",
		Name:   "Example bot",
		Debug:  os.Stdout,
	})

	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Commands.Join("#dev")
	})

	client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.Contains(e.Trailing, "hello") {
			c.Commands.Message(e.Params[0], "hello world!")
		}
	})

	log.Fatal(client.Connect())
}

// Another basic example, however with this, we add simple !<command>
// responses to things. E.g. "!hello", "!stop", and "!restart".
func Example_commands() {
	client := girc.New(girc.Config{
		Server: "irc.byteirc.org",
		Port:   6667,
		Nick:   "test",
		User:   "user",
		Name:   "Example bot",
		Out:    os.Stdout,
	})

	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Commands.Join("#channel", "#other-channel")
	})

	client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.HasPrefix(e.Trailing, "!hello") {
			c.Commands.Message(e.Params[0], "hello world!")
			return
		}

		if strings.HasPrefix(e.Trailing, "!stop") {
			c.Close(true)
			return
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}
}
