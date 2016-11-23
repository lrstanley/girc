// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

// Package girc provides a high level, yet flexible IRC library for use
// with interacting with IRC servers. girc has support for user/channel
// tracking, as well as a few other neat features (like auto-reconnect).
//
// Much of what girc can do, can also be disabled. The goal is to
// provide a solid API that you don't necessarily have to work with out
// of the box if you don't want to.
//
// See "example/main.go" for a brief and very useful example taking
// advantage of girc, that should give you a general idea of how the API
// works.
package girc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"sync"
	"time"
)

// Client contains all of the information necessary to run a single IRC
// client.
type Client struct {
	// Config represents the configuration
	Config Config
	// Events is a buffer of events waiting to be processed.
	Events chan *Event

	// state represents the throw-away state for the irc session.
	state *state
	// initTime represents the creation time of the client.
	initTime time.Time

	// cbLock is the internal locking mechanism for the callbacks map.
	cbMux sync.Mutex
	// callbacks is an internal mapping of COMMAND -> callback.
	callbacks map[string][]Callback
	// internalCallbacks is a list of callbacks used internally.
	internalCallbacks []string

	// tries represents the internal reconnect count to the IRC server.
	tries int
	// log is used if a writer is supplied for Client.Config.Logger.
	log *log.Logger
	// quitChan is used to stop the client loop. See Client.Stop().
	quitChan chan struct{}
}

// Config contains configuration options for an IRC client
type Config struct {
	// Server is a host/ip of the server you want to connect to.
	Server string
	// Port is the port that will be used during server connection.
	Port int
	// Password is the server password used to authenticate.
	Password string
	// Nick is an rfc-valid nickname used during connect.
	Nick string
	// User is the username/ident to use on connect. Ignored if identd server
	// is used.
	User string
	// Name is the "realname" that's used during connect.
	Name string
	// Conn is an optional network connection to use (overrides TLSConfig).
	Conn *net.Conn
	// TLSConfig is an optional user-supplied tls configuration, used during
	// socket creation to the server.
	TLSConfig *tls.Config
	// MaxRetries is the number of times the client will attempt to reconnect
	// to the server after the last disconnect.
	MaxRetries int
	// Logger is an optional, user supplied logger to log the raw lines sent
	// from the server. Useful for debugging. Defaults to ioutil.Discard.
	Logger io.Writer
	// ReconnectDelay is the a duration of time to delay before attempting a
	// reconnection. Defaults to 10s (minimum of 10s).
	ReconnectDelay time.Duration
	// DisableTracking disables all channel and user-level tracking. Useful
	// for highly embedded scripts with single purposes.
	DisableTracking bool
	// DisableCapTracking disables all network/server capability tracking.
	// This includes determining what feature the IRC server supports, what
	// the "NETWORK=" variables are, and other useful stuff.
	DisableCapTracking bool
	// DisableNickCollision disables the clients auto-response to nickname
	// collisions. For example, if "test" is already in use, or is blocked by
	// the network/a service, the client will try and use "test_", then it
	// will attempt "test__", "test___", and so on.
	DisableNickCollision bool
}

// ErrCallbackTimedout is used when we need to wait for temporary callbacks.
var ErrCallbackTimedout = errors.New("callback timed out while waiting for response from the server")

// New creates a new IRC client with the specified server, name and
// config.
func New(config Config) *Client {
	client := &Client{
		Config:    config,
		Events:    make(chan *Event, 100), // buffer 100 events
		quitChan:  make(chan struct{}),
		callbacks: make(map[string][]Callback),
		initTime:  time.Now(),
	}

	if client.Config.Logger == nil {
		client.Config.Logger = ioutil.Discard
	}
	client.log = log.New(client.Config.Logger, "", log.Ldate|log.Ltime|log.Lshortfile)

	// Register builtin helpers.
	client.registerHelpers()

	return client
}

// Quit disconnects from the server.
func (c *Client) Quit(message string) {
	c.state.hasQuit = true
	defer func() {
		// aaaand, unset c.hasQuit, so we can reconnect if we want to.
		c.state.hasQuit = false
	}()

	c.Send(&Event{Command: QUIT, Trailing: message})

	if c.state == nil {
		return
	}

	if c.state.conn != nil {
		c.state.conn.Close()
	}
}

// Stop exits the clients main loop. Use Client.Quit() if you want to disconnect
// the client from the server/connection.
func (c *Client) Stop() {
	// Send to the quit channel, so if Client.Loop() is being used, this will
	// return.
	c.quitChan <- struct{}{}
}

// Uptime returns the amount of time that has passed since the
// client was created.
func (c *Client) Uptime() time.Duration {
	return time.Since(c.initTime)
}

// Server returns the string representation of host+port pair for net.Conn
func (c *Client) Server() string {
	return fmt.Sprintf("%s:%d", c.Config.Server, c.Config.Port)
}

// Send sends an event to the server. Use Client.RunCallback() if you are
// are simply looking to trigger callbacks with an event.
func (c *Client) Send(event *Event) error {
	// log the event
	if !event.Sensitive {
		c.log.Print("--> ", event.String())
	}

	return c.state.writer.Encode(event)
}

// Connect attempts to connect to the given IRC server
func (c *Client) Connect() error {
	var conn net.Conn
	var err error

	// Sanity check a few options.
	if c.Config.Server == "" {
		return errors.New("invalid server specified")
	}

	if c.Config.Port < 21 || c.Config.Port > 65535 {
		return errors.New("invalid port (21-65535)")
	}

	if !IsValidNick(c.Config.Nick) || !IsValidUser(c.Config.User) {
		return errors.New("invalid nickname or user")
	}

	// Reset the state.
	c.state = newState()

	c.log.Printf("connecting to %s...", c.Server())

	// Allow the user to specify their own net.Conn.
	if c.Config.Conn == nil {
		if c.Config.TLSConfig == nil {
			conn, err = net.Dial("tcp", c.Server())
		} else {
			conn, err = tls.Dial("tcp", c.Server(), c.Config.TLSConfig)
		}
		if err != nil {
			return err
		}

		c.state.conn = conn
	} else {
		c.state.conn = *c.Config.Conn
	}

	c.state.reader = newDecoder(c.state.conn)
	c.state.writer = newEncoder(c.state.conn)
	for _, event := range c.connectMessages() {
		if err := c.Send(event); err != nil {
			return err
		}
	}

	go c.readLoop()

	// Consider the connection a success at this point.
	c.state.connected = true
	c.tries = 0

	return nil
}

// connectMessages is a list of IRC messages to send when attempting to
// connect to the IRC server.
func (c *Client) connectMessages() (events []*Event) {
	// Passwords first.
	if c.Config.Password != "" {
		events = append(events, &Event{Command: PASS, Params: []string{c.Config.Password}})
	}

	// Then nickname.
	events = append(events, &Event{Command: NICK, Params: []string{c.Config.Nick}})

	// Then username and realname.
	if c.Config.Name == "" {
		c.Config.Name = c.Config.User
	}

	events = append(events, &Event{
		Command:  USER,
		Params:   []string{c.Config.User, "+iw", "*"},
		Trailing: c.Config.Name,
	})

	return events
}

// Reconnect checks to make sure we want to, and then attempts to reconnect
// to the server.
func (c *Client) Reconnect() (err error) {
	if c.state.reconnecting {
		return errors.New("a reconnect is already occurring")
	}

	c.state.reconnecting = true
	defer func() {
		c.state.reconnecting = false
	}()

	if c.state.hasQuit {
		return nil
	}

	if c.Config.ReconnectDelay < (10 * time.Second) {
		c.Config.ReconnectDelay = 10 * time.Second
	}

	if c.state.connected {
		c.Quit("reconnecting...")
	}

	if c.Config.MaxRetries > 0 {
		var err error

		// Delay so we're not slaughtering the server with a bunch of
		// connections.
		c.log.Printf("reconnecting to %s in %s", c.Server(), c.Config.ReconnectDelay)
		time.Sleep(c.Config.ReconnectDelay)

		// Re-setup events. Do this after we've slept (giving callbacks
		// enough time to finish their tasks.)
		c.Events = make(chan *Event, 100)

		for err = c.Connect(); err != nil && c.tries < c.Config.MaxRetries; c.tries++ {
			c.log.Printf("reconnecting to %s in %s (%d tries)", c.Server(), c.Config.ReconnectDelay, c.tries)
			time.Sleep(c.Config.ReconnectDelay)
		}

		if err != nil {
			// Too many errors. Stop the client.
			c.Stop()
		}

		return err
	}

	close(c.Events)
	return nil
}

// readLoop sets a timeout of 300 seconds, and then attempts to read from the
// IRC server. If there is an error, it calls Reconnect.
func (c *Client) readLoop() error {
	for {
		c.state.conn.SetDeadline(time.Now().Add(300 * time.Second))
		event, err := c.state.reader.Decode()
		if err != nil {
			// And attempt a reconnect (if applicable).
			return c.Reconnect()
		}

		c.Events <- event
	}
}

// Loop reads from the events channel and sends the events to be handled for
// every message it receives.
func (c *Client) Loop() {
	for {
		select {
		case event := <-c.Events:
			c.RunCallbacks(event)
		case <-c.quitChan:
			return
		}
	}
}

// IsConnected returns true if the client is connected to the server.
func (c *Client) IsConnected() bool {
	c.state.m.RLock()
	defer c.state.m.RUnlock()

	return c.state.connected
}

// GetNick returns the current nickname of the active connection.
//
// Returns empty string if tracking is disabled.
func (c *Client) GetNick() string {
	if c.Config.DisableTracking {
		panic("GetNick() used when tracking is disabled")
	}

	c.state.m.RLock()
	defer c.state.m.RUnlock()

	if c.state.nick == "" {
		return c.Config.Nick
	}

	return c.state.nick
}

// SetNick changes the client nickname.
func (c *Client) SetNick(name string) {
	c.state.m.Lock()
	defer c.state.m.Unlock()

	c.state.nick = name
	c.Send(&Event{Command: NICK, Params: []string{name}})
}

// GetChannels returns the active list of channels that the client
// is in.
//
// Returns nil if tracking is disabled.
func (c *Client) GetChannels() map[string]*Channel {
	if c.Config.DisableTracking {
		panic("GetChannels() used when tracking is disabled")
	}

	c.state.m.RLock()
	defer c.state.m.RUnlock()

	return c.state.channels
}

// Who tells the client to update it's channel/user records.
//
// Does not update internal state if tracking is disabled.
func (c *Client) Who(target string) {
	c.Send(&Event{Command: WHO, Params: []string{target, "%tcuhn,1"}})
}

// Join attempts to enter an IRC channel with an optional password.
func (c *Client) Join(channel, password string) {
	if password != "" {
		c.Send(&Event{Command: JOIN, Params: []string{channel, password}})
		return
	}

	c.Send(&Event{Command: JOIN, Params: []string{channel}})
}

// Part leaves an IRC channel with an optional leave message.
func (c *Client) Part(channel, message string) {
	if message != "" {
		c.Send(&Event{Command: JOIN, Params: []string{channel}, Trailing: message})
		return
	}

	c.Send(&Event{Command: JOIN, Params: []string{channel}})
}

// Message sends a PRIVMSG to target (either channel, service, or
// user).
func (c *Client) Message(target, message string) {
	c.Send(&Event{Command: PRIVMSG, Params: []string{target}, Trailing: message})
}

// Messagef sends a formated PRIVMSG to target (either channel,
// service, or user).
func (c *Client) Messagef(target, format string, a ...interface{}) {
	c.Message(target, fmt.Sprintf(format, a...))
}

// Action sends a PRIVMSG ACTION (/me) to target (either channel,
// service, or user).
func (c *Client) Action(target, message string) {
	c.Send(&Event{Command: PRIVMSG, Params: []string{target}, Trailing: fmt.Sprintf("\001ACTION %s\001", message)})
}

// Actionf sends a formated PRIVMSG ACTION (/me) to target (either
// channel, service, or user).
func (c *Client) Actionf(target, format string, a ...interface{}) {
	c.Action(target, fmt.Sprintf(format, a...))
}

// Notice sends a NOTICE to target (either channel, service, or user).
func (c *Client) Notice(target, message string) {
	c.Send(&Event{Command: NOTICE, Params: []string{target}, Trailing: message})
}

// Noticef sends a formated NOTICE to target (either channel, service, or user).
func (c *Client) Noticef(target, format string, a ...interface{}) {
	c.Notice(target, fmt.Sprintf(format, a...))
}

// SendRaw sends a raw string back to the server, without carriage returns or
// newlines.
func (c *Client) SendRaw(raw string) {
	e := ParseEvent(raw)
	if e == nil {
		c.log.Printf("invalid event: %q", raw)
		return
	}

	c.Send(e)
}

// SendRawf sends a formated string back to the server, without carriage
// returns or newlines.
func (c *Client) SendRawf(format string, a ...interface{}) {
	c.SendRaw(fmt.Sprintf(format, a...))
}
