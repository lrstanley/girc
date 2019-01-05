// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc_test

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/lrstanley/girc"
)

func ExampleNew() {
	client := girc.New(girc.Config{
		Server: "irc.byteirc.org",
		Port:   6667,
		Nick:   "test",
		User:   "user",
		SASL:   &girc.SASLPlain{User: "user1", Pass: "securepass1"},
		Out:    os.Stdout,
	})

	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}
}

// The bare-minimum needed to get started with girc. Just connects and idles.
func Example_bare() {
	client := girc.New(girc.Config{
		Server: "irc.byteirc.org",
		Port:   6667,
		Nick:   "test",
		User:   "user",
		Debug:  os.Stdout,
	})

	if err := client.Connect(); err != nil {
		log.Fatal(err)
	}
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
		c.Cmd.Join("#dev")
	})

	client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.Contains(e.Last(), "hello") {
			c.Cmd.ReplyTo(e, "hello world!")
			return
		}

		if strings.Contains(e.Last(), "quit") {
			c.Close()
		}
	})

	// An example of how you would add reconnect logic.
	for {
		if err := client.Connect(); err != nil {
			log.Printf("error: %s", err)

			log.Println("reconnecting in 30 seconds...")
			time.Sleep(30 * time.Second)
		} else {
			return
		}
	}
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
		c.Cmd.Join("#channel", "#other-channel")
	})

	client.Handlers.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.HasPrefix(e.Last(), "!hello") {
			c.Cmd.ReplyTo(e, girc.Fmt("{b}hello{b} {blue}world{c}!"))
			return
		}

		if strings.HasPrefix(e.Last(), "!stop") {
			c.Close()
			return
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}
}
