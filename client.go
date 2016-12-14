// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
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

	// Callbacks is a handler which manages internal and external callbacks.
	Callbacks *Caller

	// tries represents the internal reconnect count to the IRC server.
	tries int
	// log is used if a writer is supplied for Client.Config.Logger.
	log *log.Logger
	// quitChan is used to stop the read loop. See Client.Quit().
	quitChan chan struct{}
	// stopChan is used to stop the client loop. See Client.Stop().
	stopChan chan struct{}
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

// ErrNotConnected is returned if a method is used when the client isn't
// connected.
var ErrNotConnected = errors.New("client is not connected to server")

// ErrAlreadyConnecting implies that a connection attempt is already happening.
var ErrAlreadyConnecting = errors.New("a connection attempt is already occurring")

// ErrInvalidTarget should be returned if the target which you are
// attempting to send an event to is invalid or doesn't match RFC spec.
type ErrInvalidTarget struct {
	Target string
}

func (e *ErrInvalidTarget) Error() string { return "invalid target: " + e.Target }

// New creates a new IRC client with the specified server, name and config.
func New(config Config) *Client {
	client := &Client{
		Config:    config,
		Events:    make(chan *Event, 100), // buffer 100 events
		quitChan:  make(chan struct{}, 1),
		stopChan:  make(chan struct{}, 1),
		Callbacks: newCaller(),
		initTime:  time.Now(),
	}

	if client.Config.Logger == nil {
		client.Config.Logger = ioutil.Discard
	}
	client.log = log.New(client.Config.Logger, "", log.Ldate|log.Ltime|log.Lshortfile)

	// Give ourselves a new state.
	client.state = newState()

	// Register builtin helpers.
	client.registerHelpers()

	return client
}

// Quit disconnects from the server.
func (c *Client) Quit(message string) {
	c.state.quitting = true
	c.quitChan <- struct{}{}
	defer func() {
		// Unset c.quitting, so we can reconnect if we want to.
		c.state.quitting = false
	}()

	c.Send(&Event{Command: QUIT, Trailing: message})

	c.state.connected = false
	if c.state.conn != nil {
		c.state.conn.Close()
	}
}

// Stop exits the clients main loop. Use Client.Quit() first if you want to
// disconnect the client from the server/connection.
func (c *Client) Stop() {
	// Send to the stop channel, so if Client.Loop() is being used, this will
	// return.
	c.stopChan <- struct{}{}
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
	c.tries = 0

	c.state.mu.Lock()
	ctime := time.Now()
	c.state.connTime = &ctime
	c.state.connected = true
	c.state.mu.Unlock()

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
		return ErrAlreadyConnecting
	}

	// Doesn't need to be set to false because a connect should reset it.
	c.state.reconnecting = true

	if c.state.quitting {
		return nil
	}

	if c.Config.ReconnectDelay < (10 * time.Second) {
		c.Config.ReconnectDelay = 10 * time.Second
	}

	if c.IsConnected() {
		c.Quit("reconnecting...")
	}

	if c.Config.MaxRetries > 0 {
		var err error

		// Delay so we're not slaughtering the server with a bunch of
		// connections.
		c.log.Printf("reconnecting to %s in %s", c.Server(), c.Config.ReconnectDelay)
		time.Sleep(c.Config.ReconnectDelay)

		for err = c.Connect(); err != nil && c.tries < c.Config.MaxRetries; c.tries++ {
			c.state.reconnecting = true
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
func (c *Client) readLoop() {
	for {
		select {
		case <-c.quitChan:
			return
		default:
			c.state.conn.SetDeadline(time.Now().Add(300 * time.Second))
			event, err := c.state.reader.Decode()
			if err != nil {
				// And attempt a reconnect (if applicable).
				c.Reconnect()
				<-c.quitChan
				return
			}

			c.Events <- event
		}
	}
}

// Loop reads from the events channel and sends the events to be handled for
// every message it receives.
func (c *Client) Loop() {
	for {
		select {
		case event := <-c.Events:
			c.RunCallbacks(event)
		case <-c.stopChan:
			return
		}
	}
}

// Server returns the string representation of host+port pair for net.Conn.
func (c *Client) Server() string {
	return fmt.Sprintf("%s:%d", c.Config.Server, c.Config.Port)
}

// Lifetime returns the amount of time that has passed since the client was
// created.
func (c *Client) Lifetime() time.Duration {
	return time.Since(c.initTime)
}

// Send sends an event to the server. Use Client.RunCallback() if you are
// simply looking to trigger callbacks with an event.
func (c *Client) Send(event *Event) error {
	// log the event
	if !event.Sensitive {
		c.log.Print("--> ", event.String())
	}

	return c.state.writer.Encode(event)
}

// Uptime is the time at which the client successfully connected to the
// server.
func (c *Client) Uptime() (up *time.Time, err error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	c.state.mu.RLock()
	up = c.state.connTime
	c.state.mu.RUnlock()

	return up, nil
}

// ConnSince is the duration that has past since the client successfully
// connected to the server.
func (c *Client) ConnSince() (since *time.Duration, err error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	c.state.mu.RLock()
	timeSince := time.Since(*c.state.connTime)
	c.state.mu.RUnlock()

	return &timeSince, nil
}

// IsConnected returns true if the client is connected to the server.
func (c *Client) IsConnected() (connected bool) {
	c.state.mu.RLock()
	connected = c.state.connected
	c.state.mu.RUnlock()

	return connected
}

// GetNick returns the current nickname of the active connection. Returns
// empty string if tracking is disabled.
func (c *Client) GetNick() (nick string) {
	if c.Config.DisableTracking {
		panic("GetNick() used when tracking is disabled")
	}

	c.state.mu.RLock()
	if c.state.nick == "" {
		nick = c.Config.Nick
	} else {
		nick = c.state.nick
	}
	c.state.mu.RUnlock()

	return nick
}

// Nick changes the client nickname.
func (c *Client) Nick(name string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	if !IsValidNick(name) {
		return &ErrInvalidTarget{Target: name}
	}

	c.state.mu.Lock()
	c.state.nick = name
	err := c.Send(&Event{Command: NICK, Params: []string{name}})
	c.state.mu.Unlock()

	return err
}

// Channels returns the active list of channels that the client is in.
// Returns nil if tracking is disabled.
func (c *Client) Channels() []string {
	if c.Config.DisableTracking {
		panic("Channels() used when tracking is disabled")
	}

	channels := make([]string, len(c.state.channels))

	c.state.mu.RLock()
	var i int
	for channel := range c.state.channels {
		channels[i] = channel
		i++
	}
	c.state.mu.RUnlock()

	return channels
}

// IsInChannel returns true if the client is in channel.
func (c *Client) IsInChannel(channel string) bool {
	c.state.mu.RLock()
	_, inChannel := c.state.channels[strings.ToLower(channel)]
	c.state.mu.RUnlock()

	return inChannel
}

// Join attempts to enter a list of IRC channels. Specify up to
func (c *Client) Join(channels ...string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}

	// We can join multiple channels at once, however we need to ensure that
	// we are not exceeding the line length. (see maxLength)
	max := maxLength - len(JOIN) - 1

	var buffer string
	var err error

	for i := 0; i < len(channels); i++ {
		if !IsValidChannel(channels[i]) {
			return &ErrInvalidTarget{Target: channels[i]}
		}

		if len(buffer+","+channels[i]) > max {
			err = c.Send(&Event{Command: JOIN, Params: []string{buffer}})
			if err != nil {
				return err
			}
			buffer = ""
			continue
		}

		if len(buffer) == 0 {
			buffer = channels[i]
		} else {
			buffer += "," + channels[i]
		}

		if i == len(channels)-1 {
			return c.Send(&Event{Command: JOIN, Params: []string{buffer}})
		}
	}

	return nil
}

// JoinKey attempts to enter an IRC channel with a password.
func (c *Client) JoinKey(channel, password string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(&Event{Command: JOIN, Params: []string{channel, password}})
}

// Part leaves an IRC channel.
func (c *Client) Part(channel, message string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(&Event{Command: JOIN, Params: []string{channel}})
}

// PartMessage leaves an IRC channel with a specified leave message.
func (c *Client) PartMessage(channel, message string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(&Event{Command: JOIN, Params: []string{channel}, Trailing: message})
}

// Message sends a PRIVMSG to target (either channel, service, or user).
func (c *Client) Message(target, message string) error {
	if !IsValidNick(target) && !IsValidChannel(target) {
		return &ErrInvalidTarget{Target: target}
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(&Event{Command: PRIVMSG, Params: []string{target}, Trailing: message})
}

// Messagef sends a formated PRIVMSG to target (either channel, service, or
// user).
func (c *Client) Messagef(target, format string, a ...interface{}) error {
	return c.Message(target, fmt.Sprintf(format, a...))
}

// Action sends a PRIVMSG ACTION (/me) to target (either channel, service,
// or user).
func (c *Client) Action(target, message string) error {
	if !IsValidNick(target) && !IsValidChannel(target) {
		return &ErrInvalidTarget{Target: target}
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(&Event{
		Command:  PRIVMSG,
		Params:   []string{target},
		Trailing: fmt.Sprintf("\001ACTION %s\001", message),
	})
}

// Actionf sends a formated PRIVMSG ACTION (/me) to target (either channel,
// service, or user).
func (c *Client) Actionf(target, format string, a ...interface{}) error {
	return c.Action(target, fmt.Sprintf(format, a...))
}

// Notice sends a NOTICE to target (either channel, service, or user).
func (c *Client) Notice(target, message string) error {
	if !IsValidNick(target) && !IsValidChannel(target) {
		return &ErrInvalidTarget{Target: target}
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(&Event{Command: NOTICE, Params: []string{target}, Trailing: message})
}

// Noticef sends a formated NOTICE to target (either channel, service, or
// user).
func (c *Client) Noticef(target, format string, a ...interface{}) error {
	return c.Notice(target, fmt.Sprintf(format, a...))
}

// SendRaw sends a raw string back to the server, without carriage returns
// or newlines.
func (c *Client) SendRaw(raw string) error {
	e := ParseEvent(raw)
	if e == nil {
		return errors.New("invalid event: " + raw)
	}

	if !c.IsConnected() {
		return ErrNotConnected
	}

	return c.Send(e)
}

// SendRawf sends a formated string back to the server, without carriage
// returns or newlines.
func (c *Client) SendRawf(format string, a ...interface{}) error {
	return c.SendRaw(fmt.Sprintf(format, a...))
}

// Topic sets the topic of channel to message. Does not verify the length
// of the topic.
func (c *Client) Topic(channel, message string) error {
	return c.Send(&Event{Command: TOPIC, Params: []string{channel}, Trailing: message})
}

// ErrCallbackTimedout is used when we need to wait for temporary callbacks.
type ErrCallbackTimedout struct {
	// ID is the identified of the callback in the callback stack.
	ID string
	// Timeout is the time that past before the callback timed out.
	Timeout time.Duration
}

func (e *ErrCallbackTimedout) Error() string {
	return "callback [" + e.ID + "] timed out while waiting for response from the server: " + e.Timeout.String()
}

// Who runs a WHO query to give details about an active user. Can only be
// used if the calling callback was added as a background callback. Will
// lock otherwise.
func (c *Client) Who(nick string) (*User, error) {
	if !IsValidNick(nick) {
		return nil, &ErrInvalidTarget{Target: nick}
	}

	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	var user *User
	whoDone := make(chan struct{})

	whoHandler := func(c *Client, e Event) {
		var eIdent, eHost, eNick string

		// Assume WHOX related.
		if e.Command == RPL_WHOSPCRPL {
			if len(e.Params) != 6 {
				// Assume there was some form of error or invalid WHOX response.
				return
			}

			if e.Params[1] != "2" {
				// We should always be sending 2, and we should receive 2. If
				// this is anything but, then we didn't send the request and we
				// can ignore it.
				return
			}

			eIdent, eHost, eNick = e.Params[3], e.Params[4], e.Params[5]
		} else {
			eIdent, eHost, eNick = e.Params[2], e.Params[3], e.Params[5]
		}

		if strings.ToLower(eNick) != strings.ToLower(nick) {
			// Not the same user we're querying for.
			return
		}

		user = &User{
			Nick:  eNick,
			Ident: eIdent,
			Host:  eHost,
			Name:  e.Trailing,
		}

		whoDone <- struct{}{}
	}

	// Add a callback for both the standard WHO as well as a WHOX response.
	// Use register rather than sregister, as we're already in a lock.
	whoCb := c.Callbacks.AddBg(RPL_WHOREPLY, whoHandler)
	whoXCb := c.Callbacks.AddBg(RPL_WHOSPCRPL, whoHandler)

	// Initiate the event so we can catch it.
	err := c.Send(&Event{Command: WHO, Params: []string{nick, "%tcuhnr,2"}})
	if err != nil {
		return nil, err
	}

	// Wait for everything to finish. Give the server 2 seconds to respond.
	select {
	case <-whoDone:
		close(whoDone)
	case <-time.After(time.Second * 2):
		// Remove callbacks and return. Took too long.
		c.Callbacks.Remove(whoCb)
		c.Callbacks.Remove(whoXCb)

		return nil, &ErrCallbackTimedout{
			ID:      whoCb + " + " + whoXCb,
			Timeout: time.Second * 2,
		}
	}

	c.Callbacks.Remove(whoCb)
	c.Callbacks.Remove(whoXCb)

	return user, nil
}

// Whowas sends and waits for a response to a WHOWAS query to the server.
// Returns the list of users form the WHOWAS query. Can only be used if the
// calling callback was added as a background callback. Will lock otherwise.
func (c *Client) Whowas(nick string) ([]*User, error) {
	if !IsValidNick(nick) {
		return nil, &ErrInvalidTarget{Target: nick}
	}

	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	var mu sync.Mutex
	var events []*Event
	whoDone := make(chan struct{})

	// One callback needs to be added to collect the WHOWAS lines.
	// <nick> <user> <host> * :<real_name>
	whoCb := c.Callbacks.AddBg(RPL_WHOWASUSER, func(c *Client, e Event) {
		if len(e.Params) != 5 {
			return
		}

		// First check and make sure that this WHOWAS is for us.
		if e.Params[1] != nick {
			return
		}

		mu.Lock()
		events = append(events, &e)
		mu.Unlock()
	})

	// One more callback needs to be added to let us know when WHOWAS has
	// finished.
	// 	<nick> :<info>
	whoDoneCb := c.Callbacks.AddBg(RPL_ENDOFWHOWAS, func(c *Client, e Event) {
		if len(e.Params) != 2 {
			return
		}

		// First check and make sure that this WHOWAS is for us.
		if e.Params[1] != nick {
			return
		}

		mu.Lock()
		whoDone <- struct{}{}
		mu.Unlock()
	})

	// Send the WHOWAS query.
	err := c.Send(&Event{Command: WHOWAS, Params: []string{nick, "10"}})
	if err != nil {
		return nil, err
	}

	// Wait for everything to finish. Give the server 2 seconds to respond.
	select {
	case <-whoDone:
		close(whoDone)
	case <-time.After(time.Second * 2):
		// Remove callbacks and return. Took too long.
		c.Callbacks.Remove(whoCb)
		c.Callbacks.Remove(whoDoneCb)

		return nil, &ErrCallbackTimedout{
			ID:      whoCb + " + " + whoDoneCb,
			Timeout: time.Second * 2,
		}
	}

	// Remove the temporary callbacks to ensure that nothing else is
	// received.
	c.Callbacks.Remove(whoCb)
	c.Callbacks.Remove(whoDoneCb)

	var users []*User

	for i := 0; i < len(events); i++ {
		users = append(users, &User{
			Nick:  events[i].Params[1],
			Ident: events[i].Params[2],
			Host:  events[i].Params[3],
			Name:  events[i].Trailing,
		})
	}

	return users, nil
}
