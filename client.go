// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Client contains all of the information necessary to run a single IRC
// client.
type Client struct {
	// Config represents the configuration
	Config Config
	// rx is a buffer of events waiting to be processed.
	rx chan *Event
	// tx is a buffer of events waiting to be sent.
	tx chan *Event

	// state represents the throw-away state for the irc session.
	state *state
	// initTime represents the creation time of the client.
	initTime time.Time

	// Handlers is a handler which manages internal and external handlers.
	Handlers *Caller
	// CTCP is a handler which manages internal and external CTCP handlers.
	CTCP *CTCP
	// Commands contains various helper methods to interact with the server.
	Commands *Commands

	// conn is a net.Conn reference to the IRC server.
	conn *ircConn

	// cmux is the mux used for connections/disconnections from the server,
	// so multiple threads aren't trying to connect at the same time, and
	// vice versa.
	cmux sync.Mutex

	// debug is used if a writer is supplied for Client.Config.Debugger.
	debug *log.Logger
}

// Config contains configuration options for an IRC client
type Config struct {
	// Server is a host/ip of the server you want to connect to. This only
	// has an affect during the dial process
	Server string
	// Port is the port that will be used during server connection. This only
	// has an affect during the dial process.
	Port int
	// Password is the server password used to authenticate. This only has an
	// affect during the dial process.
	Password string
	// Nick is an rfc-valid nickname used during connection. This only has an
	// affect during the dial process.
	Nick string
	// User is the username/ident to use on connect. Ignored if an identd
	// server is used. This only has an affect during the dial process.
	User string
	// Name is the "realname" that's used during connection. This only has an
	// affect during the dial process.
	Name string
	// Proxy is a proxy based address, used during the dial process when
	// connecting to the server. This only has an affect during the dial
	// process. Currently, x/net/proxy only supports socks5, however you can
	// add your own proxy functionality using:
	//    proxy.RegisterDialerType
	//
	// Examples of how Proxy may be used:
	//    socks5://localhost:8080
	//    socks5://1.2.3.4:8888
	//    customProxy://example.com:8000
	//
	Proxy string
	// Bind is used to bind to a specific host or ip during the dial process
	// when connecting to the server. This can be a hostname, however it must
	// resolve to an IPv4/IPv6 address bindable on your system. Otherwise,
	// you can simply use a IPv4/IPv6 address directly. This only has an
	// affect during the dial process.
	Bind string
	// SSL allows dialing via TLS. See TLSConfig to set your own TLS
	// configuration (e.g. to not force hostname checking). This only has an
	// affect during the dial process.
	SSL bool
	// TLSConfig is an optional user-supplied tls configuration, used during
	// socket creation to the server. SSL must be enabled for this to be used.
	// This only has an affect during the dial process.
	TLSConfig *tls.Config
	// AllowFlood allows the client to bypass the rate limit of outbound
	// messages.
	AllowFlood bool
	// Debug is an optional, user supplied location to log the raw lines
	// sent from the server, or other useful debug logs. Defaults to
	// ioutil.Discard. For quick debugging, this could be set to os.Stdout.
	Debug io.Writer
	// Out is used to print out a prettified version of certain, important
	// events, ignoring ones that are not important.
	Out io.Writer
	// RecoverFunc is called when a handler throws a panic. If RecoverFunc is
	// set, the panic will be considered recovered, otherwise the client will
	// panic. Set this to DefaultRecoverHandler if you don't want the client
	// to panic, however you don't want to handle the panic yourself.
	// DefaultRecoverHandler will log the panic to Debug or os.Stdout if
	// Debug is unset.
	RecoverFunc func(c *Client, e *HandlerError)
	// SupportedCaps are the IRCv3 capabilities you would like the client to
	// support. Only use this if DisableTracking and DisableCapTracking are
	// not enabled, otherwise you will need to handle CAP negotiation yourself.
	// The keys value gets passed to the server if supported.
	SupportedCaps map[string][]string
	// Version is the application version information that will be used in
	// response to a CTCP VERSION, if default CTCP replies have not been
	// overwritten or a VERSION handler was already supplied.
	Version string
	// PingDelay is the frequency between when the client sends keep-alive
	// ping's to the server, and awaits a response (timing out if the server
	// doesn't respond in time). This must be between 20-600 seconds. See
	// Client.Lag() if you want to determine the delay between the server
	// and the client.
	PingDelay time.Duration

	// disableTracking disables all channel and user-level tracking. Useful
	// for highly embedded scripts with single purposes.
	disableTracking bool
	// HandleNickCollide when set, allows the client to handle nick collisions
	// in a custom way. If unset, the client will attempt to append a
	// underscore to the end of the nickname, in order to bypass using
	// an invalid nickname. For example, if "test" is already in use, or is
	// blocked by the network/a service, the client will try and use "test_",
	// then it will attempt "test__", "test___", and so on.
	HandleNickCollide func(oldNick string) (newNick string)
}

// isValid checks some basic settings to ensure the config is valid.
func (conf Config) isValid() error {
	if conf.Server == "" {
		return errors.New("invalid server specified")
	}

	if conf.Port < 21 || conf.Port > 65535 {
		return errors.New("invalid port (21-65535)")
	}

	if !IsValidNick(conf.Nick) || !IsValidUser(conf.User) {
		return errors.New("invalid nickname or user")
	}

	return nil
}

// ErrNotConnected is returned if a method is used when the client isn't
// connected.
var ErrNotConnected = errors.New("client is not connected to server")

// ErrDisconnected is called when Config.Retries is less than 1, and we
// non-intentionally disconnected from the server.
var ErrDisconnected = errors.New("unexpectedly disconnected")

// ErrInvalidTarget should be returned if the target which you are
// attempting to send an event to is invalid or doesn't match RFC spec.
type ErrInvalidTarget struct {
	Target string
}

func (e *ErrInvalidTarget) Error() string { return "invalid target: " + e.Target }

// New creates a new IRC client with the specified server, name and config.
func New(config Config) *Client {
	c := &Client{
		Config:   config,
		rx:       make(chan *Event, 25),
		tx:       make(chan *Event, 25),
		CTCP:     newCTCP(),
		initTime: time.Now(),
	}

	c.Commands = &Commands{c: c}

	if c.Config.PingDelay < (20 * time.Second) {
		c.Config.PingDelay = 20 * time.Second
	} else if c.Config.PingDelay > (600 * time.Second) {
		c.Config.PingDelay = 600 * time.Second
	}

	if c.Config.Debug == nil {
		c.debug = log.New(ioutil.Discard, "", 0)
	} else {
		c.debug = log.New(c.Config.Debug, "debug:", log.Ltime|log.Lshortfile)
		c.debug.Print("initializing debugging")
	}

	// Setup the caller.
	c.Handlers = newCaller(c.debug)

	// Give ourselves a new state.
	c.state = newState()

	// Register builtin handlers.
	c.registerBuiltins()

	// Register default CTCP responses.
	c.CTCP.addDefaultHandlers()

	return c
}

// String returns a brief description of the current client state.
func (c *Client) String() string {
	var connected bool
	if c.conn != nil {
		connected = c.conn.connected
	}

	return fmt.Sprintf(
		"<Client init:%q handlers:%d connected:%t>", c.initTime.String(), c.Handlers.Len(), connected,
	)
}

// Close exits the clients main loop and any other goroutines created by
// the client itself. This does not include handlers, as they will run for
// any incoming events prior to when Close() or Quit() was called, until the
// event queue is empty and execution has completed for those handlers. This
// means that you are responsible to ensure that your handlers due not
// execute forever. Use Client.Quit() first if you want to disconnect the
// client from the server/connection gracefully.
func (c *Client) Close(sendQuit bool) {
	if sendQuit {
		c.Send(&Event{Command: QUIT, Trailing: "closing"})
	}

	_ = c.conn.Close()
	c.RunHandlers(&Event{Command: STOPPED, Trailing: c.Server()})
}

func (c *Client) execLoop(done chan struct{}) {
	for {
		select {
		case event := <-c.rx:
			c.RunHandlers(event)
		case <-done:
			return
		}
	}
}

// DisableTracking disables all channel and user-level tracking, and clears
// all internal handlers. Useful for highly embedded scripts with single
// purposes. This cannot be un-done.
func (c *Client) DisableTracking() {
	c.debug.Print("disabling tracking")
	c.Config.disableTracking = true
	c.Handlers.clearInternal()

	c.state.mu.Lock()
	c.state.channels = nil
	c.state.mu.Unlock()

	c.registerBuiltins()
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

// Uptime is the time at which the client successfully connected to the
// server.
func (c *Client) Uptime() (up *time.Time, err error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	up = c.conn.connTime

	return up, nil
}

// ConnSince is the duration that has past since the client successfully
// connected to the server.
func (c *Client) ConnSince() (since *time.Duration, err error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	timeSince := time.Since(*c.conn.connTime)

	return &timeSince, nil
}

// IsConnected returns true if the client is connected to the server.
func (c *Client) IsConnected() (connected bool) {
	if c.conn == nil {
		return false
	}
	return c.conn.connected
}

// GetNick returns the current nickname of the active connection. Panics if
// tracking is disabled.
func (c *Client) GetNick() string {
	c.panicIfNotTracking()
	var nick string

	c.state.mu.RLock()
	if c.state.nick == "" {
		nick = c.Config.Nick
	} else {
		nick = c.state.nick
	}
	c.state.mu.RUnlock()

	return nick
}

// GetIdent returns the current ident of the active connection. Panics if
// tracking is disabled. May be empty, as this is obtained from when we join
// a channel, as there is no other more efficient method to return this info.
func (c *Client) GetIdent() string {
	c.panicIfNotTracking()
	var ident string

	c.state.mu.RLock()
	if c.state.ident == "" {
		ident = c.Config.Name
	} else {
		ident = c.state.ident
	}
	c.state.mu.RUnlock()

	return ident
}

// GetHost returns the current host of the active connection. Panics if
// tracking is disabled. May be empty, as this is obtained from when we join
// a channel, as there is no other more efficient method to return this info.
func (c *Client) GetHost() string {
	c.panicIfNotTracking()
	var host string

	c.state.mu.RLock()
	if c.state.host == "" {
		host = c.Config.Name
	} else {
		host = c.state.host
	}
	c.state.mu.RUnlock()

	return host
}

// Channels returns the active list of channels that the client is in.
// Panics if tracking is disabled.
func (c *Client) Channels() []string {
	c.panicIfNotTracking()
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

// Lookup looks up a given channel in state. If the channel doesn't exist,
// channel is nil. Panics if tracking is disabled.
func (c *Client) Lookup(name string) *Channel {
	c.panicIfNotTracking()
	c.state.mu.Lock()
	defer c.state.mu.Unlock()

	channel := c.state.lookupChannel(name)
	if channel == nil {
		return nil
	}

	return channel.Copy()
}

// IsInChannel returns true if the client is in channel. Panics if tracking
// is disabled.
func (c *Client) IsInChannel(channel string) bool {
	c.panicIfNotTracking()

	c.state.mu.RLock()
	_, inChannel := c.state.channels[strings.ToLower(channel)]
	c.state.mu.RUnlock()

	return inChannel
}

// GetServerOption retrieves a server capability setting that was retrieved
// during client connection. This is also known as ISUPPORT (or RPL_PROTOCTL).
// Will panic if used when tracking has been disabled. Examples of usage:
//
//   nickLen, success := GetServerOption("MAXNICKLEN")
//
func (c *Client) GetServerOption(key string) (result string, ok bool) {
	c.panicIfNotTracking()

	c.state.mu.Lock()
	result, ok = c.state.serverOptions[key]
	c.state.mu.Unlock()

	return result, ok
}

// ServerName returns the server host/name that the server itself identifies
// as. May be empty if the server does not support RPL_MYINFO. Will panic if
// used when tracking has been disabled.
func (c *Client) ServerName() (name string) {
	c.panicIfNotTracking()

	name, _ = c.GetServerOption("SERVER")

	return name
}

// NetworkName returns the network identifier. E.g. "EsperNet", "ByteIRC".
// May be empty if the server does not support RPL_ISUPPORT (or RPL_PROTOCTL).
// Will panic if used when tracking has been disabled.
func (c *Client) NetworkName() (name string) {
	c.panicIfNotTracking()

	name, _ = c.GetServerOption("NETWORK")

	return name
}

// ServerVersion returns the server software version, if the server has
// supplied this information during connection. May be empty if the server
// does not support RPL_MYINFO. Will panic if used when tracking has been
// disabled.
func (c *Client) ServerVersion() (version string) {
	c.panicIfNotTracking()

	version, _ = c.GetServerOption("VERSION")

	return version
}

// ServerMOTD returns the servers message of the day, if the server has sent
// it upon connect. Will panic if used when tracking has been disabled.
func (c *Client) ServerMOTD() (motd string) {
	c.panicIfNotTracking()

	c.state.mu.Lock()
	motd = c.state.motd
	c.state.mu.Unlock()

	return motd
}

// Lag is the latency between the server and the client. This is measured by
// determining the difference in time between when we ping the server, and
// when we receive a pong.
func (c *Client) Lag() time.Duration {
	delta := c.conn.lastPong.Sub(c.conn.lastPing)
	if delta < 0 {
		return 0
	}

	return delta
}

// panicIfNotTracking will throw a panic when it's called, and tracking is
// disabled. Adds useful info like what function specifically, and where it
// was called from.
func (c *Client) panicIfNotTracking() {
	if !c.Config.disableTracking {
		return
	}

	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	_, file, line, _ := runtime.Caller(2)

	panic(fmt.Sprintf("%s used when tracking is disabled (caller %s:%d)", fn.Name(), file, line))
}
