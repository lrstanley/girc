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

	client := girc.New(conf)

	client.AddCallback(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Join("#dev", "")
	})

	client.AddCallback(girc.PRIVMSG, func(c *girc.Client, e girc.Event) {
		if strings.Contains(e.Trailing, "hello") {
			c.Message(e.Params[0], "hello world!")
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}

	// Don't push this into a goroutine normally.
	client.Loop()
}
