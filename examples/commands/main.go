// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package main

import (
	"log"
	"strings"

	"os"

	"github.com/lrstanley/girc"
)

func main() {
	conf := girc.Config{
		Server:     "irc.byteirc.org",
		Port:       6667,
		Nick:       "test",
		User:       "user",
		Name:       "Example bot",
		MaxRetries: 3,
		Logger:     os.Stdout,
	}
	channels := []string{"#dev"}

	client := girc.New(conf)

	client.Callbacks.Add(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		for _, ircchan := range channels {
			c.Join(ircchan, "")
		}
	})

	client.Callbacks.Add(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.HasPrefix(e.Trailing, "!hello") {
			c.Message(e.Params[0], "hello world!")
			return
		}

		if strings.HasPrefix(e.Trailing, "!stop") {
			c.Quit("goodbye!")
			c.Stop()
			return
		}

		if strings.HasPrefix(e.Trailing, "!restart") {
			go c.Reconnect()
			return
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}

	client.Loop()
}
