// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
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
		if strings.HasPrefix(e.Trailing, "!stop") {
			c.Message(e.Params[0], "hello world!")
			return
		}

		if strings.HasPrefix(e.Trailing, "!stop") {
			c.Quit("goodbye!")
			c.Stop()
			return
		}

		if strings.HasPrefix(e.Trailing, "!restart") {
			c.Reconnect()
			return
		}
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect to %s: %s", client.Server(), err)
	}

	go client.Loop()

	// Everything after this line is just for fancy prompt stuff. Not needed.
	reader := bufio.NewReader(os.Stdin)

	client.Callbacks.Add(girc.ALLEVENTS, func(c *girc.Client, e girc.Event) {
		fmt.Print("\r>> ")
	})

	for {
		fmt.Print("\r>> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if len(input) == 0 {
			continue
		}

		if input == "quit" {
			client.Quit("g'day")
			client.Stop()
			os.Exit(0)
		}

		e := girc.ParseEvent(input)
		if e == nil {
			fmt.Println("ERRONOUS INPUT")
			continue
		}

		client.Send(e)
	}
}
