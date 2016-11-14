// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"strings"

	"os"

	"github.com/Liamraystanley/girc"
)

func main() {
	conf := girc.Config{
		Server:     "irc.byteirc.org",
		Port:       6667,
		Nick:       "test",
		User:       "test1",
		Name:       "Example bot",
		MaxRetries: 3,
		Logger:     os.Stdout,
	}

	client := girc.New(conf)

	client.AddCallback(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Join("#dev", "")
	})

	client.AddCallback(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if !e.IsAction() && strings.Contains(e.Trailing, "hello") {
			c.Message(e.Params[0], "Hello World!")
		}

		if e.IsAction() && strings.Contains(e.Trailing, "hello") {
			c.Action(e.Params[0], "says Hello World!")
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}

	client.Loop()
}
