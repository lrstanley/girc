// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"time"
)

// Client contains all of the information necessary to run a single IRC
// client.
type Client struct {
	// config represents the configuration
	config Config
	// Events is a buffer of events waiting to be processed.
	Events chan *Event

	// state represents the throw-away state for the irc session.
	state *state
	// initTime represents the creation time of the client.
	initTime time.Time

	// Callbacks is a handler which manages internal and external callbacks.
	Callbacks *Caller
	// CTCP is a handler which manages internal and external CTCP handlers.
	CTCP *CTCP

	// tries represents the internal reconnect count to the IRC server.
	tries int
	// stopped determins if Client.Stop() has been called.
	stopped bool
	// limiter is a configurable EventLimiter by the end user.
	limiter *EventLimiter
	// debug is used if a writer is supplied for Client.Config.Debugger.
	debug *log.Logger

	// closeRead is the function which sends a close to the readLoop function
	// context.
	closeRead context.CancelFunc
	// closeExec is the function which sends a close to the execLoop function
	// context.
	closeExec context.CancelFunc
	// closeLoop is the function which sends a close to the Loop function
	// context.
	closeLoop context.CancelFunc
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
	// RateLimit is the delay in seconds between events sent to the server,
	// with a burst of 4 messages. Set to -1 to disable.
	RateLimit int
	// Debugger is an optional, user supplied location to log the raw lines
	// sent from the server, or other useful debug logs. Defaults to
	// ioutil.Discard.
	Debugger io.Writer
	// SupportedCaps are the IRCv3 capabilities you would like the client to
	// support. Only use this if DisableTracking and DisableCapTracking are
	// not enabled, otherwise you will need to handle CAP negotiation yourself.
	// The keys value gets passed to the server if supported.
	SupportedCaps map[string][]string
	// Version is the application version information that will be used in
	// response to a CTCP VERSION, if default CTCP replies have not been
	// overwritten or a VERSION handler was already supplied.
	Version string
	// ReconnectDelay is the a duration of time to delay before attempting a
	// reconnection. Defaults to 10s (minimum of 10s).
	ReconnectDelay time.Duration
	// DisableTracking disables all channel and user-level tracking. Useful
	// for highly embedded scripts with single purposes.
	DisableTracking bool
	// DisableDefaultCTCP disables all default CTCP responses. Though, any
	// set CTCP's will override any pre-set ones, by default.
	DisableDefaultCTCP bool
	// DisableCapTracking disables all network/server capability tracking.
	// This includes determining what feature the IRC server supports, what
	// the "NETWORK=" variables are, and other useful stuff. DisableTracking
	// cannot be enabled if you want to also tracking capabilities.
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

// ErrCalledAfterStop is used when one uses Client.Stop(), and subsequently
// attempts to use the client again.
var ErrCalledAfterStop = errors.New("attempted use after stop has been called")

// ErrInvalidTarget should be returned if the target which you are
// attempting to send an event to is invalid or doesn't match RFC spec.
type ErrInvalidTarget struct {
	Target string
}

func (e *ErrInvalidTarget) Error() string { return "invalid target: " + e.Target }

// New creates a new IRC client with the specified server, name and config.
func New(config Config) *Client {
	client := &Client{
		config:   config,
		Events:   make(chan *Event, 100), // buffer 100 events max.
		CTCP:     newCTCP(),
		initTime: time.Now(),
	}

	if client.config.Debugger == nil {
		client.config.Debugger = ioutil.Discard
	}
	client.debug = log.New(client.config.Debugger, "debug:", log.Ltime|log.Lshortfile)
	client.debug.Print("initializing debugging")

	// Setup the caller.
	client.Callbacks = newCaller(client.debug)

	// Setup a rate limiter if they requested one.
	if client.config.RateLimit == 0 {
		client.limiter = NewEventLimiter(4, 1*time.Second, client.write)
	} else if client.config.RateLimit > 0 {
		client.limiter = NewEventLimiter(4, time.Duration(client.config.RateLimit)*time.Second, client.write)
	}

	// Give ourselves a new state.
	client.state = newState()

	// Register builtin handlers.
	client.registerHandlers()

	// Register default CTCP responses.
	client.CTCP.disableDefault = client.config.DisableDefaultCTCP
	client.CTCP.addDefaultHandlers()

	return client
}

func (c *Client) cleanup(all bool) {
	if c.closeRead != nil {
		c.closeRead()
	}
	if c.closeExec != nil {
		c.closeExec()
	}

	if all {
		if c.closeLoop != nil {
			c.closeLoop()
		}

		// Close and limiters they have, otherwise the client could be easily
		// held in memory.
		if c.limiter != nil {
			c.limiter.Stop()
		}
	}
}

// quit is the underlying wrapper to quit from the network and cleanup.
func (c *Client) quit(sendMessage bool) {
	if sendMessage {
		c.Send(&Event{Command: QUIT, Trailing: "disconnecting..."})
	}

	c.state.connected = false
	if c.state.conn != nil {
		c.state.conn.Close()
	}

	// Close out the read and exec loops.
	c.cleanup(false)
}

// Quit disconnects from the server.
func (c *Client) Quit() {
	c.quit(true)
}

// Quit disconnects from the server with a given message.
func (c *Client) QuitWithMessage(message string) {
	c.Send(&Event{Command: QUIT, Trailing: message})

	c.quit(false)
}

// Stop exits the clients main loop and any other goroutines created by
// the client itself. This does not include callbacks, as they will run for
// any incoming events prior to when Stop() or Quit() was called, until the
// event queue is empty and execution has completed for those callbacks. This
// means that you are responsible to ensure that your callbacks due not
// execute forever. Use Client.Quit() first if you want to disconnect the
// client from the server/connection gracefully. Once Stop is called, the
// client is no longer useable, and will panic if used again.
func (c *Client) Stop() {
	if c.stopped {
		panic(ErrCalledAfterStop)
	}

	// Close out any other running loops.
	c.cleanup(true)
	c.stopped = true
}

// Connect attempts to connect to the given IRC server
func (c *Client) Connect() error {
	if c.stopped {
		panic(ErrCalledAfterStop)
	}

	var conn net.Conn
	var err error

	// Sanity check a few options.
	if c.config.Server == "" {
		return errors.New("invalid server specified")
	}

	if c.config.Port < 21 || c.config.Port > 65535 {
		return errors.New("invalid port (21-65535)")
	}

	if !IsValidNick(c.config.Nick) || !IsValidUser(c.config.User) {
		return errors.New("invalid nickname or user")
	}

	// Reset the state.
	c.state = newState()

	c.debug.Printf("connecting to %s...", c.Server())

	// Allow the user to specify their own net.Conn.
	if c.config.Conn == nil {
		if c.config.TLSConfig == nil {
			conn, err = net.Dial("tcp", c.Server())
		} else {
			conn, err = tls.Dial("tcp", c.Server(), c.config.TLSConfig)
		}
		if err != nil {
			return err
		}

		c.state.conn = conn
	} else {
		c.state.conn = *c.config.Conn
	}

	c.state.reader = newDecoder(c.state.conn)
	c.state.writer = newEncoder(c.state.conn)

	// Send a virtual event allowing hooks for successful socket connection.
	c.Events <- &Event{Command: INITIALIZED, Trailing: c.Server()}

	for _, event := range c.connectMessages() {
		if err := c.write(event); err != nil {
			return err
		}
	}

	// List the IRCv3 capabilities, specifically with the max protocol we
	// support.
	if err := c.listCAP(); err != nil {
		return err
	}

	// Start read loop to process messages from the server.
	var rctx, ectx context.Context
	rctx, c.closeRead = context.WithCancel(context.Background())
	ectx, c.closeRead = context.WithCancel(context.Background())
	go c.readLoop(rctx)
	go c.execLoop(ectx)

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
	if c.config.Password != "" {
		events = append(events, &Event{Command: PASS, Params: []string{c.config.Password}})
	}

	// Then nickname.
	events = append(events, &Event{Command: NICK, Params: []string{c.config.Nick}})

	// Then username and realname.
	if c.config.Name == "" {
		c.config.Name = c.config.User
	}

	events = append(events, &Event{Command: USER, Params: []string{c.config.User, "+iw", "*"}, Trailing: c.config.Name})

	return events
}

// reconnect checks to make sure we want to, and then attempts to reconnect
// to the server.
func (c *Client) reconnect(remoteInvoked bool) (err error) {
	if c.stopped {
		panic(ErrCalledAfterStop)
	}

	if c.state.reconnecting {
		return ErrAlreadyConnecting
	}

	c.state.reconnecting = true
	// A successful connect should reset the state, however if Connect() fails,
	// this will ensure it gets unset so reconnect() can be used again.
	defer func() {
		c.state.reconnecting = false
	}()

	if c.state.quitting {
		return nil
	}

	if c.config.ReconnectDelay < (10 * time.Second) {
		c.config.ReconnectDelay = 25 * time.Second
	}

	// Make sure we're not connected.
	c.Quit()

	if c.config.MaxRetries < 1 && !remoteInvoked {
		return errors.New("unexpectedly disconnected")
	}

	// Delay so we're not slaughtering the server with a bunch of
	// connections.
	c.debug.Printf("reconnecting to %s in %s", c.Server(), c.config.ReconnectDelay)
	time.Sleep(c.config.ReconnectDelay)

	for err = c.Connect(); err != nil && c.tries < c.config.MaxRetries; c.tries++ {
		c.state.reconnecting = true
		c.debug.Printf("reconnecting to %s in %s (%d tries)", c.Server(), c.config.ReconnectDelay, c.tries)
		time.Sleep(c.config.ReconnectDelay)
	}

	if err != nil {
		// Too many errors. Stop the client.
		c.Stop()
	}

	return err
}

// reconnect checks to make sure we want to, and then attempts to reconnect
// to the server.
func (c *Client) Reconnect() error {
	return c.reconnect(true)
}

// readLoop sets a timeout of 300 seconds, and then attempts to read from the
// IRC server. If there is an error, it calls Reconnect.
func (c *Client) readLoop(ctx context.Context) {
	var event *Event
	var err error

	for {
		select {
		case <-ctx.Done():
			return
		default:
			c.state.conn.SetDeadline(time.Now().Add(10 * time.Second))
			event, err = c.state.reader.Decode()
			if err != nil {
				// And attempt a reconnect (if applicable).
				c.reconnect(false)
				return
			}

			if event == nil {
				continue
			}

			c.Events <- event
		}
	}
}

func (c *Client) execLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-c.Events:
			c.RunCallbacks(event)
		}
	}
}

// Loop reads from the events channel and sends the events to be handled for
// every message it receives.
func (c *Client) Loop() {
	var ctx context.Context
	ctx, c.closeLoop = context.WithCancel(context.Background())

	<-ctx.Done()
}

// Server returns the string representation of host+port pair for net.Conn.
func (c *Client) Server() string {
	return fmt.Sprintf("%s:%d", c.config.Server, c.config.Port)
}

// Lifetime returns the amount of time that has passed since the client was
// created.
func (c *Client) Lifetime() time.Duration {
	return time.Since(c.initTime)
}

// Send sends an event to the server. Use Client.RunCallback() if you are
// simply looking to trigger callbacks with an event.
func (c *Client) Send(event *Event) error {
	// if the client wants us to rate limit incoming events, do so, otherwise
	// simply use the underlying send functionality.
	if c.limiter != nil {
		return c.limiter.Send(event)
	}

	return c.write(event)
}

// write is the lower level function to write an event.
func (c *Client) write(event *Event) error {
	// log the event
	if !event.Sensitive {
		c.debug.Print("> ", StripRaw(event.String()))
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
	if c.config.DisableTracking {
		panic("GetNick() used when tracking is disabled")
	}

	c.state.mu.RLock()
	if c.state.nick == "" {
		nick = c.config.Nick
	} else {
		nick = c.state.nick
	}
	c.state.mu.RUnlock()

	return nick
}

// Nick changes the client nickname.
func (c *Client) Nick(name string) error {
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
// Panics if tracking is disabled.
func (c *Client) Channels() []string {
	if c.config.DisableTracking {
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

// IsInChannel returns true if the client is in channel. Panics if tracking
// is disabled.
func (c *Client) IsInChannel(channel string) bool {
	if c.config.DisableTracking {
		panic("Channels() used when tracking is disabled")
	}

	c.state.mu.RLock()
	_, inChannel := c.state.channels[strings.ToLower(channel)]
	c.state.mu.RUnlock()

	return inChannel
}

// Join attempts to enter a list of IRC channels, at bulk if possible to
// prevent sending extensive JOIN commands.
func (c *Client) Join(channels ...string) error {
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

	return c.Send(&Event{Command: JOIN, Params: []string{channel, password}})
}

// Part leaves an IRC channel.
func (c *Client) Part(channel, message string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	return c.Send(&Event{Command: JOIN, Params: []string{channel}})
}

// PartMessage leaves an IRC channel with a specified leave message.
func (c *Client) PartMessage(channel, message string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	return c.Send(&Event{Command: JOIN, Params: []string{channel}, Trailing: message})
}

// SendCTCP sends a CTCP request to target. Note that this method uses
// PRIVMSG specifically.
func (c *Client) SendCTCP(target, ctcpType, message string) error {
	out := encodeCTCPRaw(ctcpType, message)
	if out == "" {
		return errors.New("invalid CTCP")
	}

	return c.Message(target, out)
}

// SendCTCPf sends a CTCP request to target using a specific format. Note that
// this method uses PRIVMSG specifically.
func (c *Client) SendCTCPf(target, ctcpType, format string, a ...interface{}) error {
	return c.SendCTCP(target, ctcpType, fmt.Sprintf(format, a...))
}

// SendCTCPReplyf sends a CTCP response to target using a specific format.
// Note that this method uses NOTICE specifically.
func (c *Client) SendCTCPReplyf(target, ctcpType, format string, a ...interface{}) error {
	return c.SendCTCPReply(target, ctcpType, fmt.Sprintf(format, a...))
}

// SendCTCPReply sends a CTCP response to target. Note that this method uses
// NOTICE specifically.
func (c *Client) SendCTCPReply(target, ctcpType, message string) error {
	out := encodeCTCPRaw(ctcpType, message)
	if out == "" {
		return errors.New("invalid CTCP")
	}

	return c.Notice(target, out)
}

// Message sends a PRIVMSG to target (either channel, service, or user).
func (c *Client) Message(target, message string) error {
	if !IsValidNick(target) && !IsValidChannel(target) {
		return &ErrInvalidTarget{Target: target}
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

// Who sends a WHO query to the server, which will attempt WHOX by default.
// See http://faerion.sourceforge.net/doc/irc/whox.var for more details. This
// sends "%tcuhnr,2" per default. Do not use "1" as this will conflict with
// girc's builtin tracking functionality.
func (c *Client) Who(target string) error {
	if !IsValidNick(target) && !IsValidChannel(target) && !IsValidUser(target) {
		return &ErrInvalidTarget{Target: target}
	}

	return c.Send(&Event{Command: WHO, Params: []string{target, "%tcuhnr,2"}})
}

// Whois sends a WHOIS query to the server, targetted at a specific user.
// as WHOIS is a bit slower, you may want to use WHO for brief user info.
func (c *Client) Whois(nick string) error {
	if !IsValidNick(nick) {
		return &ErrInvalidTarget{Target: nick}
	}

	return c.Send(&Event{Command: WHOIS, Params: []string{nick}})
}

// Ping sends a PING query to the server, with a specific identifier that
// the server should respond with.
func (c *Client) Ping(id string) error {
	return c.Send(&Event{Command: PING, Params: []string{id}})
}

// Pong sends a PONG query to the server, with an identifier which was
// received from a previous PING query received by the client.
func (c *Client) Pong(id string) error {
	return c.Send(&Event{Command: PONG, Params: []string{id}})
}

// Oper sends a OPER authentication query to the server, with a username
// and password.
func (c *Client) Oper(user, pass string) error {
	return c.Send(&Event{Command: OPER, Params: []string{user, pass}, Sensitive: true})
}

// Kick sends a KICK query to the server, attempting to kick nick from
// channel, with reason. If reason is blank, one will not be sent to the
// server.
func (c *Client) Kick(channel, nick, reason string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	if !IsValidNick(nick) {
		return &ErrInvalidTarget{Target: nick}
	}

	if reason != "" {
		return c.Send(&Event{Command: KICK, Params: []string{channel, nick}, Trailing: reason})
	}

	return c.Send(&Event{Command: KICK, Params: []string{channel, nick}})
}

// Invite sends a INVITE query to the server, to invite nick to channel.
func (c *Client) Invite(channel, nick string) error {
	if !IsValidChannel(channel) {
		return &ErrInvalidTarget{Target: channel}
	}

	if !IsValidNick(nick) {
		return &ErrInvalidTarget{Target: nick}
	}

	return c.Send(&Event{Command: INVITE, Params: []string{nick, channel}})
}

// Away sends a AWAY query to the server, suggesting that the client is no
// longer active. If reason is blank, Client.Back() is called. Also see
// Client.Back().
func (c *Client) Away(reason string) error {
	if reason == "" {
		return c.Back()
	}

	return c.Send(&Event{Command: AWAY, Params: []string{reason}})
}

// Back sends a AWAY query to the server, however the query is blank,
// suggesting that the client is active once again. Also see Client.Away().
func (c *Client) Back() error {
	return c.Send(&Event{Command: AWAY})
}

// LIST sends a LIST query to the server, which will list channels and topics.
// Supports multiple channels at once, in hopes it will reduce extensive
// LIST queries to the server. Supply no channels to run a list against the
// entire server (warning, that may mean LOTS of channels!)
func (c *Client) List(channels ...string) error {
	if len(channels) == 0 {
		return c.Send(&Event{Command: LIST})
	}

	// We can LIST multiple channels at once, however we need to ensure that
	// we are not exceeding the line length. (see maxLength)
	max := maxLength - len(JOIN) - 1

	var buffer string
	var err error

	for i := 0; i < len(channels); i++ {
		if !IsValidChannel(channels[i]) {
			return &ErrInvalidTarget{Target: channels[i]}
		}

		if len(buffer+","+channels[i]) > max {
			err = c.Send(&Event{Command: LIST, Params: []string{buffer}})
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
			return c.Send(&Event{Command: LIST, Params: []string{buffer}})
		}
	}

	return nil
}

// Whowas sends a WHOWAS query to the server. amount is the amount of results
// you want back.
func (c *Client) Whowas(nick string, amount int) error {
	if !IsValidNick(nick) {
		return &ErrInvalidTarget{Target: nick}
	}

	return c.Send(&Event{Command: WHOWAS, Params: []string{nick, string(amount)}})
}

// GetServerOption retrieves a server capability setting that was retrieved
// during client connection. This is also known as ISUPPORT (or RPL_PROTOCTL).
// Will panic if used when tracking has been disabled. Examples of usage:
//
//   nickLen, success := GetServerOption("MAXNICKLEN")
//
func (c *Client) GetServerOption(key string) (result string, ok bool) {
	if c.config.DisableTracking {
		panic("GetServerOption() used when tracking is disabled")
	}

	c.state.mu.Lock()
	result, ok = c.state.serverOptions[key]
	c.state.mu.Unlock()

	return result, ok
}

// ServerName returns the server host/name that the server itself identifies
// as. May be empty if the server does not support RPL_MYINFO. Will panic if
// used when tracking has been disabled.
func (c *Client) ServerName() (name string) {
	if c.config.DisableTracking {
		panic("ServerName() used when tracking is disabled")
	}

	name, _ = c.GetServerOption("SERVER")

	return name
}

// NetworkName returns the network identifier. E.g. "EsperNet", "ByteIRC".
// May be empty if the server does not support RPL_ISUPPORT (or RPL_PROTOCTL).
// Will panic if used when tracking has been disabled.
func (c *Client) NetworkName() (name string) {
	if c.config.DisableTracking {
		panic("NetworkName() used when tracking is disabled")
	}

	name, _ = c.GetServerOption("NETWORK")

	return name
}

// ServerVersion returns the server software version, if the server has
// supplied this information during connection. May be empty if the server
// does not support RPL_MYINFO. Will panic if used when tracking has been
// disabled.
func (c *Client) ServerVersion() (version string) {
	if c.config.DisableTracking {
		panic("ServerVersion() used when tracking is disabled")
	}

	version, _ = c.GetServerOption("VERSION")

	return version
}

// ServerMOTD returns the servers message of the day, if the server has sent
// it upon connect. Will panic if used when tracking has been disabled.
func (c *Client) ServerMOTD() (motd string) {
	if c.config.DisableTracking {
		panic("ServerMOTD() used when tracking is disabled")
	}

	c.state.mu.Lock()
	motd = c.state.motd
	c.state.mu.Unlock()

	return motd
}
