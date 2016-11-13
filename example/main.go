package main

import (
	"log"
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

	client.AddCallback(girc.CONNECTED, registerConnect)

	if err := client.Connect(); err != nil {
		log.Fatalf("an error occurred while attempting to connect: %s", err)
	}

	client.Wait()
}

func registerConnect(c *girc.Client, e *girc.Event) {
	c.Send(&girc.Event{Command: girc.JOIN, Params: []string{"#dev"}})
}
