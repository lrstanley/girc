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
	"bytes"
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
	// Sender is a Sender{} interface implementation.
	Sender Sender

	// state represents the throw-away state for the irc session.
	state *state
	// initTime represents the creation time of the client.
	initTime time.Time

	// cbLock is the internal locking mechanism for the callbacks map.
	cbMux sync.Mutex
	// callbacks is an internal mapping of COMMAND -> callback.
	callbacks map[string][]Callback

	// reader is the socket buffer reader from the IRC server.
	reader *Decoder
	// reader is the socket buffer write to the IRC server.
	writer *Encoder
	// conn is a net.Conn reference to the IRC server.
	conn net.Conn
	// tries represents the internal reconnect count to the IRC server.
	tries int
	// log is used if a writer is supplied for Client.Config.Logger.
	log *log.Logger
	// quitChan is used to close the connection to the IRC server.
	quitChan chan struct{}
	// hasQuit is used to determine if we've finished quitting/cleaning up.
	hasQuit bool
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

// New creates a new IRC client with the specified server, name and
// config.
func New(config Config) *Client {
	client := &Client{
		Config:    config,
		Events:    make(chan *Event, 40), // buffer 40 events
		quitChan:  make(chan struct{}),
		callbacks: make(map[string][]Callback),
		tries:     0,
		initTime:  time.Now(),
	}

	// Register builtin helpers.
	client.registerHelpers()

	return client
}

// Quit disconnects from the server.s
func (c *Client) Quit(message string) {
	c.Send(&Event{Command: QUIT, Trailing: message})

	c.hasQuit = true

	if c.conn != nil {
		c.conn.Close()
	}

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

	return c.Sender.Send(event)
}

// Connect attempts to connect to the given IRC server
func (c *Client) Connect() error {
	var conn net.Conn
	var err error

	// Sanity check a few options.
	if c.Config.Server == "" || c.Config.Port == 0 || c.Config.Nick == "" || c.Config.User == "" {
		return errors.New("invalid configuration (server/port/nick/user)")
	}

	// Reset the state.
	c.state = newState()

	if c.Config.Logger == nil {
		c.Config.Logger = ioutil.Discard
	}

	c.log = log.New(c.Config.Logger, "", log.Ldate|log.Ltime|log.Lshortfile)

	if c.Config.TLSConfig == nil {
		conn, err = net.Dial("tcp", c.Server())
	} else {
		conn, err = tls.Dial("tcp", c.Server(), c.Config.TLSConfig)
	}
	if err != nil {
		return err
	}

	c.conn = conn
	c.reader = NewDecoder(conn)
	c.writer = NewEncoder(conn)
	c.Sender = serverSender{writer: c.writer}
	for _, event := range c.connectMessages() {
		if err := c.Send(event); err != nil {
			return err
		}
	}

	c.tries = 0
	go c.ReadLoop()

	// Consider the connection a success at this point.
	c.state.connected = true

	return nil
}

// connectMessages is a list of IRC messages to send when attempting
// to connect to the IRC server.
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

// Reconnect checks to make sure we want to, and then attempts to
// reconnect to the server.
func (c *Client) Reconnect() (err error) {
	if c.hasQuit {
		return nil
	}

	if c.Config.ReconnectDelay < (10 * time.Second) {
		c.Config.ReconnectDelay = 10 * time.Second
	}

	if c.Config.MaxRetries > 0 {
		var err error
		c.conn.Close()

		// Re-setup events.
		c.Events = make(chan *Event, 40)

		// Delay so we're not slaughtering the server with a bunch of
		// connections.
		c.log.Printf("reconnecting to %s in %s", c.Server(), c.Config.ReconnectDelay)
		time.Sleep(c.Config.ReconnectDelay)

		for err = c.Connect(); err != nil && c.tries < c.Config.MaxRetries; c.tries++ {
			c.log.Printf("reconnecting to %s in %s (%d tries)", c.Server(), c.Config.ReconnectDelay, c.tries)
			time.Sleep(c.Config.ReconnectDelay)
		}

		return err
	}

	close(c.Events)
	return nil
}

// ReadLoop sets a timeout of 300 seconds, and then attempts to read
// from the IRC server. If there is an error, it calls Reconnect.
func (c *Client) ReadLoop() error {
	for {
		c.conn.SetDeadline(time.Now().Add(300 * time.Second))
		event, err := c.reader.Decode()
		if err != nil {
			return c.Reconnect()
		}

		c.Events <- event
	}
}

// Loop reads from the events channel and sends the events to be
// handled for every message it receives.
func (c *Client) Loop() {
	for {
		select {
		case event := <-c.Events:
			c.handleEvent(event)
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
		return ""
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
		return nil
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

// contains '*', even though this isn't RFC compliant, it's commonly used
var validChannelPrefixes = [...]string{"&", "#", "+", "!", "*"}

// IsValidChannel checks if channel is an RFC complaint channel or not
//
// channel      =  ( "#" / "+" / ( "!" channelid ) / "&" ) chanstring
//                 [ ":" chanstring ]
//   chanstring =  0x01-0x07 / 0x08-0x09 / 0x0B-0x0C / 0x0E-0x1F / 0x21-0x2B
//   chanstring =  / 0x2D-0x39 / 0x3B-0xFF
//                   ; any octet except NUL, BELL, CR, LF, " ", "," and ":"
//   channelid  = 5( 0x41-0x5A / digit )   ; 5( A-Z / 0-9 )
func IsValidChannel(channel string) bool {
	if len(channel) <= 1 || len(channel) > 50 {
		return false
	}

	// #, +, !<channelid>, or &
	// Including "*" in the prefix list, as this is commonly used (e.g. ZNC)
	if bytes.IndexByte([]byte{0x21, 0x23, 0x26, 0x2A, 0x2B}, channel[0]) == -1 {
		return false
	}

	// !<channelid> -- not very commonly supported, but we'll check it anyway.
	// The ID must be 5 chars. This means min-channel size should be:
	//   1 (prefix) + 5 (id) + 1 (+, channel name)
	if channel[0] == 0x21 {
		if len(channel) < 7 {
			return false
		}

		// check for valid ID
		for i := 1; i < 6; i++ {
			if (channel[i] < 0x30 || channel[i] > 0x39) && (channel[i] < 0x41 || channel[i] > 0x5A) {
				return false
			}
		}
	}

	// Check for invalid octets here.
	bad := []byte{0x00, 0x07, 0x0D, 0x0A, 0x20, 0x2C, 0x3A}
	for i := 1; i < len(channel); i++ {
		if bytes.IndexByte(bad, channel[i]) != -1 {
			return false
		}
	}

	return true
}

// IsValidNick valids an IRC nickame. Note that this does not valid IRC
// nickname length.
//
// nickname   =  ( letter / special ) *8( letter / digit / special / "-" )
//   letter   =  0x41-0x5A / 0x61-0x7A
//   digit    =  0x30-0x39
//   special  =  0x5B-0x60 / 0x7B-0x7D
func IsValidNick(nick string) bool {
	if len(nick) <= 0 {
		return false
	}

	// Check the first index. Some characters aren't allowed for the first
	// index of an IRC nickname.
	if nick[0] < 0x41 || nick[0] > 0x7D {
		// a-z, A-Z, and _\[]{}^|
		return false
	}

	for i := 1; i < len(nick); i++ {
		if (nick[i] < 0x41 || nick[i] > 0x7D) && (nick[i] < 0x30 || nick[i] > 0x39) && nick[i] != 0x2D {
			// a-z, A-Z, 0-9, -, and _\[]{}^|
			return false
		}
	}

	return true
}
