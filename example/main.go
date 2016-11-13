package main

import (
	"log"

	"os"

	"github.com/Liamraystanley/girc"
)

func main() {
	conf := girc.Config{
		Server:         "irc.byteirc.org",
		Port:           6667,
		Nick:           "test",
		User:           "test1",
		Name:           "Example bot",
		MaxRetries:     3,
		Logger:         os.Stdout,
		DisableHelpers: false,
	}

	client := girc.New(conf)

	client.AddCallback(girc.CONNECTED, func(c *girc.Client, e girc.Event) {
		c.Join("#dev", "")
	})

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect: %s", err)
	}

	client.Wait()
}
