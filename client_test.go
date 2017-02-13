// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc_test

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lrstanley/girc"
)

// The bare-minimum needed to get started with girc. Just connects and idles.
func Example_bare() {
	errHandler := func(err error) {
		if err == nil {
			return
		}

		log.Fatalf("error occurred: %s", err)
	}

	client := girc.New(girc.Config{
		Server:      "irc.byteirc.org",
		Port:        6667,
		Nick:        "test",
		User:        "user",
		Debugger:    os.Stdout,
		HandleError: errHandler,
	})

	errHandler(client.Connect())
	client.Loop()
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
	})

	client.Handlers.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Commands.Join("#dev")
	})

	client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.Contains(e.Trailing, "hello") {
			c.Commands.Message(e.Params[0], "hello world!")
		}
	})

	// Log useful IRC events.
	client.Handlers.Add(girc.ALLEVENTS, func(c *girc.Client, e girc.Event) {
		// girc.Event.Pretty() returns true for events which are useful and
		// that can be prettified. Use Event.String() to get the raw string
		// for all events.
		if pretty, ok := e.Pretty(); ok {
			// The use of girc.StripRaw() is to get rid of any potential
			// non-printable characters.
			fmt.Println(girc.StripRaw(pretty))
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}

	client.Loop()
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
			c.Quit()
			c.Stop()
			return
		}

		if strings.HasPrefix(e.Trailing, "!restart") {
			go c.Reconnect()
			return
		}
	})

	// Log ALL events.
	client.Handlers.Add(girc.ALLEVENTS, func(c *girc.Client, e girc.Event) {
		// The use of girc.StripRaw() is to get rid of any potential
		// non-printable characters.
		fmt.Println(girc.StripRaw(e.String()))
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}

	client.Loop()
}
