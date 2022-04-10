// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package main

import (
	"log"
	"os"
	"time"

	"github.com/lrstanley/girc"
)

func main() {
	client := girc.New(girc.Config{
		Server: "irc.esper.net",
		Port:   6667,
		Nick:   "liam-testing",
		User:   "liam",
		Name:   "Example user",
		Debug:  os.Stdout,
	})

	// client.Handlers.AddBg(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
	// 	// c.Cmd.Join("#dev")
	// 	time.Sleep(30 * time.Second)
	// 	c.Quit("bye")
	// 	// c.Cmd.SendRaw("ERROR")
	// })

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
