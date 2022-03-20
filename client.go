// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Client contains all of the information necessary to run a single IRC
// client.
type Client struct {
	// Config represents the configuration. Please take extra caution in that
	// entries in this are not edited while the client is connected, to prevent
	// data races. This is NOT concurrent safe to update.
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
	// Cmd contains various helper methods to interact with the server.
	Cmd *Commands
	// mu is the mux used for connections/disconnections from the server,
	// so multiple threads aren't trying to connect at the same time, and
	// vice versa.
	mu sync.RWMutex

	// IRCd encapsulates IRC Server details.
	IRCd Server

	// stop is used to communicate with Connect(), letting it know that the
	// client wishes to cancel/close.
	stop context.CancelFunc
	// conn is a net.Conn reference to the IRC server. If this is nil, it is
	// safe to assume that we're not connected. If this is not nil, this
	// means we're either connected, connecting, or cleaning up. This should
	// be guarded with Client.mu.
	conn *ircConn
	// debug is used if a writer is supplied for Client.Config.Debugger.
	debug *log.Logger

	atom uint32
}

// Server contains information about the IRC server that the client is connected to.
type Server struct {
	// Network is the name of the IRC network we are connected to as acquired by 001.
	Network atomic.Value
	// Version is the software version of the IRC daemon as acquired by 004.
	Version string
	// Host is the hostname/id/IP of the leaf, as acquired by 002.
	Host string
	// compiled is the reported date the server was compiled on as acquired by 003.
	Compiled time.Time
	// UserCount is the amount of online users currently on this network as acquired by 251.
	UserCount int
	// MaxUserCount is the amount of online users currently on this network as acquired by 251.
	MaxUserCount int
	// LocalUserCount is the amount of online users currently on this leaf as acquired by 265.
	LocalUserCount int
	// LocalMaxUserCount is the maximum amount of users that have been on this leaf as acquired by 265.
	LocalMaxUserCount int
	// OperCount is the amount of opers currently online as acquired by 252.
	OperCount int
	// ChannelCount is the amount of channels formed as acquired by 254.
	ChannelCount int
}

// Config contains configuration options for an IRC client
type Config struct {
	// Server is a host/ip of the server you want to connect to. This only
	// has an affect during the dial process
	Server string
	// ServerPass is the server password used to authenticate. This only has
	// an affect during the dial process.
	ServerPass string
	// Port is the port that will be used during server connection. This only
	// has an affect during the dial process.
	Port int
	// Nick is an rfc-valid nickname used during connection. This only has an
	// affect during the dial process.
	Nick string
	// User is the username/ident to use on connect. Ignored if an identd
	// server is used. This only has an affect during the dial process.
	User string
	// Name is the "realname" that's used during connection. This only has an
	// affect during the dial process.
	Name string
	// SASL contains the necessary authentication data to authenticate
	// with SASL. See the documentation for SASLMech for what is currently
	// supported. Capability tracking must be enabled for this to work, as
	// this requires IRCv3 CAP handling.
	SASL SASLMech
	// WebIRC allows forwarding source user hostname/ip information to the server
	// (if supported by the server) to ensure the source machine doesn't show as
	// the source. See the WebIRC type for more information.
	WebIRC WebIRC
	// Bind is used to bind to a specific host or ip during the dial process
	// when connecting to the server. This can be a hostname, however it must
	// resolve to an IPv4/IPv6 address bindable on your system. Otherwise,
	// you can simply use a IPv4/IPv6 address directly. This only has an
	// affect during the dial process and will not work with DialerConnect().
	Bind string
	// SSL allows dialing via TLS. See TLSConfig to set your own TLS
	// configuration (e.g. to not force hostname checking). This only has an
	// affect during the dial process.
	SSL bool
	// DisableSTS disables the use of automatic STS connection upgrades
	// when the server supports STS. STS can also be disabled using the environment
	// variable "GIRC_DISABLE_STS=true". As many clients may not propagate options
	// like this back to the user, this allows to directly disable such automatic
	// functionality.
	DisableSTS bool
	// DisableSTSFallback disables the "fallback" to a non-tls connection if the
	// strict transport policy expires and the first attempt to reconnect back to
	// the tls version fails.
	DisableSTSFallback bool
	// TLSConfig is an optional user-supplied tls configuration, used during
	// socket creation to the server. SSL must be enabled for this to be used.
	// This only has an affect during the dial process.
	TLSConfig *tls.Config
	// AllowFlood allows the client to bypass the rate limit of outbound
	// messages.
	AllowFlood bool
	// GlobalFormat enables passing through all events which have trailing
	// text through the color Fmt() function, so you don't have to wrap
	// every response in the Fmt() method.
	//
	// Note that this only actually applies to PRIVMSG, NOTICE and TOPIC
	// events, to ensure it doesn't clobber unwanted events.
	GlobalFormat bool
	// Debug is an optional, user supplied location to log the raw lines
	// sent from the server, or other useful debug logs. Defaults to
	// ioutil.Discard. For quick debugging, this could be set to os.Stdout.
	Debug io.Writer
	// Out is used to write out a prettified version of incoming events. For
	// example, channel JOIN/PART, PRIVMSG/NOTICE, KICk, etc. Useful to get
	// a brief output of the activity of the client. If you are looking to
	// log raw messages, look at a handler and girc.ALLEVENTS and the relevant
	// Event.Bytes() or Event.String() methods.
	Out io.Writer
	// RecoverFunc is called when a handler throws a panic. If RecoverFunc is
	// set, the panic will be considered recovered, otherwise the client will
	// panic. Set this to DefaultRecoverHandler if you don't want the client
	// to panic, however you don't want to handle the panic yourself.
	// DefaultRecoverHandler will log the panic to Debug or os.Stdout if
	// Debug is unset.
	RecoverFunc func(c *Client, e *HandlerError)
	// SupportedCaps are the IRCv3 capabilities you would like the client to
	// support on top of the ones which the client already supports (see
	// cap.go for which ones the client enables by default). Only use this
	// if you have not called DisableTracking(). The keys value gets passed
	// to the server if supported.
	SupportedCaps map[string][]string
	// Version is the application version information that will be used in
	// response to a CTCP VERSION. A default message will be sent otherwise.
	Version string
	// ClientInfo is the application ClientInfo code information that will be used in
	// response to a CTCP CLIENTINFO. No response will be sent if this is not set.
	ClientInfo string
	// UserInfo is the user information that will be used in
	// response to a CTCP USERINFO. No response will be sent if this is not set.
	UserInfo string
	// Finger is the client information that will be used in
	// response to a CTCP FINGER. A default message will be sent otherwise.
	Finger string
	// Source is the application source code information that will be used in
	// response to a CTCP SOURCE. A default message will be sent otherwise.
	Source string
	// PingDelay is the frequency between when the client sends a keep-alive
	// PING to the server, and awaits a response (and times out if the server
	// doesn't respond in time). This should be between 20-600 seconds. See
	// Client.Latency() if you want to determine the delay between the server
	// and the client. If this is set to -1, the client will not attempt to
	// send client -> server PING requests.
	PingDelay time.Duration

	// disableTracking disables all channel and user-level tracking. Useful
	// for highly embedded scripts with single purposes. This has an exported
	// method which enables this and ensures proper cleanup, see
	// Client.DisableTracking().
	disableTracking bool
	// HandleNickCollide when set, allows the client to handle nick collisions
	// in a custom way. If unset, the client will attempt to append a
	// underscore to the end of the nickname, in order to bypass using
	// an invalid nickname. For example, if "test" is already in use, or is
	// blocked by the network/a service, the client will try and use "test_",
	// then it will attempt "test__", "test___", and so on.
	//
	// If HandleNickCollide returns an empty string, the client will not
	// attempt to fix nickname collisions, and you must handle this yourself.
	HandleNickCollide func(oldNick string) (newNick string)
}

// WebIRC is useful when a user connects through an indirect method, such web
// clients, the indirect client sends its own IP address instead of sending the
// user's IP address unless WebIRC is implemented by both the client and the
// server.
//
// Client expectations:
//  - Perform any proxy resolution.
//  - Check the reverse DNS and forward DNS match.
//  - Check the IP against suitable access controls (ipaccess, dnsbl, etc).
//
// More information:
//  - https://ircv3.net/specs/extensions/webirc.html
//  - https://kiwiirc.com/docs/webirc
type WebIRC struct {
	// Password that authenticates the WEBIRC command from this client.
	Password string
	// Gateway or client type requesting spoof (cgiirc defaults to cgiirc, as an
	// example).
	Gateway string
	// Hostname of user.
	Hostname string
	// Address either in IPv4 dotted quad notation (e.g. 192.0.0.2) or IPv6
	// notation (e.g. 1234:5678:9abc::def). IPv4-in-IPv6 addresses
	// (e.g. ::ffff:192.0.0.2) should not be sent.
	Address string
}

// Params returns the arguments for the WEBIRC command that can be passed to the
// server.
func (w WebIRC) Params() []string {
	return []string{w.Password, w.Gateway, w.Hostname, w.Address}
}

// ErrInvalidConfig is returned when the configuration passed to the client
// is invalid.
type ErrInvalidConfig struct {
	Conf Config // Conf is the configuration that was not valid.
	err  error
}

func (e ErrInvalidConfig) Error() string { return "invalid configuration: " + e.err.Error() }

// isValid checks some basic settings to ensure the config is valid.
func (conf *Config) isValid() error {
	if conf.Server == "" {
		return &ErrInvalidConfig{Conf: *conf, err: errors.New("empty server")}
	}

	// Default port to 6667 (the standard IRC port).
	if conf.Port == 0 {
		conf.Port = 6667
	}

	if conf.Port < 1 || conf.Port > 65535 {
		return &ErrInvalidConfig{Conf: *conf, err: errors.New("port outside valid range (1-65535)")}
	}

	if !IsValidNick(conf.Nick) {
		return &ErrInvalidConfig{Conf: *conf, err: errors.New("bad nickname specified: " + conf.Nick)}
	}
	if !IsValidUser(conf.User) {
		return &ErrInvalidConfig{Conf: *conf, err: errors.New("bad user/ident specified: " + conf.User)}
	}

	return nil
}

// ErrNotConnected is returned if a method is used when the client isn't
// connected.
var ErrNotConnected = errors.New("client is not connected to server")

// New creates a new IRC client with the specified server, name and config.
func New(config Config) *Client {
	c := &Client{
		Config:   config,
		rx:       make(chan *Event, 25),
		tx:       make(chan *Event, 25),
		CTCP:     newCTCP(),
		initTime: time.Now(),
	}

	c.IRCd = Server{
		Network:      atomic.Value{},
		Version:      "",
		UserCount:    0,
		MaxUserCount: 0,
	}

	c.IRCd.Network.Store("")

	c.Cmd = &Commands{c: c}

	if c.Config.PingDelay >= 0 && c.Config.PingDelay < (20*time.Second) {
		c.Config.PingDelay = 20 * time.Second
	} else if c.Config.PingDelay > (600 * time.Second) {
		c.Config.PingDelay = 600 * time.Second
	}

	envDebug, _ := strconv.ParseBool(os.Getenv("GIRC_DEBUG"))
	if c.Config.Debug == nil {
		if envDebug {
			c.debug = log.New(os.Stderr, "debug:", log.Ltime|log.Lshortfile)
		} else {
			c.debug = log.New(ioutil.Discard, "", 0)
		}
	} else {
		if envDebug {
			if c.Config.Debug != os.Stdout && c.Config.Debug != os.Stderr {
				c.Config.Debug = io.MultiWriter(os.Stderr, c.Config.Debug)
			}
		}
		c.debug = log.New(c.Config.Debug, "debug:", log.Ltime|log.Lshortfile)
		c.debug.Print("initializing debugging")
	}

	envDisableSTS, _ := strconv.ParseBool(os.Getenv("GIRC_DISABLE_STS"))
	if envDisableSTS {
		c.Config.DisableSTS = envDisableSTS
	}

	// Setup the caller.
	c.Handlers = newCaller(c, c.debug)

	// Give ourselves a new state.
	c.state = &state{}
	c.state.RWMutex = &sync.RWMutex{}
	c.state.reset(true)

	c.state.client = c

	// Register builtin handlers.
	c.registerBuiltins()

	// Register default CTCP responses.
	c.CTCP.addDefaultHandlers()

	return c
}

// String returns a brief description of the current client state.
func (c *Client) String() string {
	connected := c.IsConnected()

	return fmt.Sprintf(
		"<Client init:%q handlers:%d connected:%t>", c.initTime.String(), c.Handlers.Len(), connected,
	)
}

// TLSConnectionState returns the TLS connection state from tls.Conn{}, which
// is useful to return needed TLS fingerprint info, certificates, verify cert
// expiration dates, etc. Will only return an error if the underlying
// connection wasn't established using TLS (see ErrConnNotTLS), or if the
// client isn't connected.
func (c *Client) TLSConnectionState() (*tls.ConnectionState, error) {
	if c.conn == nil {
		return nil, ErrNotConnected
	}

	if !c.conn.connected.Load().(bool) {
		return nil, ErrNotConnected
	}

	if tlsConn, ok := c.conn.sock.(*tls.Conn); ok {
		cs := tlsConn.ConnectionState()
		return &cs, nil
	}

	return nil, ErrConnNotTLS
}

// ErrConnNotTLS is returned when Client.TLSConnectionState() is called, and
// the connection to the server wasn't made with TLS.
var ErrConnNotTLS = errors.New("underlying connection is not tls")

// Close closes the network connection to the server, and sends a CLOSED
// event. This should cause Connect() to return with nil. This should be
// safe to call multiple times. See Connect()'s documentation on how
// handlers and goroutines are handled when disconnected from the server.
func (c *Client) Close() {
	if c.stop != nil {
		c.debug.Print("requesting client to stop")
		c.stop()
	}
}

// Quit sends a QUIT message to the server with a given reason to close the
// connection. Underlying this event being sent, Client.Close() is called as well.
// This is different than just calling Client.Close() in that it provides a reason
// as to why the connection was closed (for bots to tell users the bot is restarting,
// or shutting down, etc).
//
// NOTE: servers may delay showing of QUIT reasons, until you've been connected to
// the server for a certain period of time (e.g. 5 minutes). Keep this in mind.
func (c *Client) Quit(reason string) {
	c.Send(&Event{Command: QUIT, Params: []string{reason}})
}

// ErrEvent is an error returned when the server (or library) sends an ERROR
// message response. The string returned contains the trailing text from the
// message.
type ErrEvent struct {
	Event *Event
}

func (e *ErrEvent) Error() string {
	if e.Event == nil {
		return "unknown error occurred"
	}

	return e.Event.Last()
}

func (c *Client) execLoop(ctx context.Context, errs chan error, working *int32) {
	c.debug.Print("starting execLoop")
	defer c.debug.Print("closing execLoop")

	defer atomic.AddInt32(working, -1)

	var event *Event

	for {
		select {
		case <-ctx.Done():
			// We've been told to exit, however we shouldn't bail on the
			// current events in the queue that should be processed, as one
			// may want to handle an ERROR, QUIT, etc.
			c.debug.Printf("received signal to close, flushing %d events and executing", len(c.rx))
			for {
				select {
				case event = <-c.rx:
					c.RunHandlers(event)
				default:
					goto done
				}
			}

		done:
			return
		case event = <-c.rx:
			if event != nil && event.Command == ERROR {
				// Handles incoming ERROR responses. These are only ever sent
				// by the server (with the exception that this library may use
				// them as a lower level way of signalling to disconnect due
				// to some other client-choosen error), and should always be
				// followed up by the server disconnecting the client. If for
				// some reason the server doesn't disconnect the client, or
				// if this library is the source of the error, this should
				// signal back up to the main connect loop, to disconnect.
				errs <- &ErrEvent{Event: event}

				// Make sure to not actually exit, so we can let any handlers
				// actually handle the ERROR event.
			}

			c.RunHandlers(event)
		}
	}
}

// DisableTracking disables all channel/user-level/CAP tracking, and clears
// all internal handlers. Useful for highly embedded scripts with single
// purposes. This cannot be un-done on a client.
func (c *Client) DisableTracking() {
	c.debug.Print("disabling tracking")
	c.Config.disableTracking = true
	c.Handlers.clearInternal()

	c.state.channels.Clear()
	c.state.notify(c, UPDATE_STATE)

	c.registerBuiltins()
}

// Server returns the string representation of host+port pair for the connection.
func (c *Client) Server() string {
	return c.server()
}

// server returns the string representation of host+port pair for net.Conn, and
// takes into consideration STS. Must lock state mu first!
func (c *Client) server() string {
	if c.state.sts.enabled() {
		return net.JoinHostPort(c.Config.Server, strconv.Itoa(c.state.sts.upgradePort))
	}
	return net.JoinHostPort(c.Config.Server, strconv.Itoa(c.Config.Port))
}

// Lifetime returns the amount of time that has passed since the client was
// created.
func (c *Client) Lifetime() time.Duration {
	return time.Since(c.initTime)
}

// Uptime is the time at which the client successfully connected to the
// server.
func (c *Client) Uptime() (up time.Time, err error) {
	if !c.IsConnected() {
		return time.Now(), ErrNotConnected
	}

	up = c.conn.connTime.Load().(time.Time)

	return up, nil
}

// ConnSince is the duration that has past since the client successfully
// connected to the server.
func (c *Client) ConnSince() (since *time.Duration, err error) {
	if !c.IsConnected() {
		return nil, ErrNotConnected
	}

	timeSince := time.Since(c.conn.connTime.Load().(time.Time))

	return &timeSince, nil
}

// IsConnected returns true if the client is connected to the server.
func (c *Client) IsConnected() bool {
	if c == nil {
		return false
	}
	if c.conn == nil {
		return false
	}
	if c.conn.connected.Load() == nil {
		c.conn.connected.Store(false)
	}

	return c.conn.connected.Load().(bool)
}

// GetNick returns the current nickname of the active connection. Panics if
// tracking is disabled.
func (c *Client) GetNick() string {
	if c == nil {
		return ""
	}
	c.panicIfNotTracking()
	n := c.state.nick.Load().(string)

	if len(n) < 1 {
		return c.Config.Nick
	}

	return n
}

// GetID returns an RFC1459 compliant version of the current nickname. Panics
// if tracking is disabled.
func (c *Client) GetID() string {
	return ToRFC1459(c.GetNick())
}

// GetIdent returns the current ident of the active connection. Panics if
// tracking is disabled. May be empty, as this is obtained from when we join
// a channel, as there is no other more efficient method to return this info.
func (c *Client) GetIdent() string {
	c.panicIfNotTracking()

	if c.state.ident.Load().(string) == "" {
		return c.Config.User
	}
	return c.state.ident.Load().(string)
}

// GetHost returns the current host of the active connection. Panics if
// tracking is disabled. May be empty, as this is obtained from when we join
// a channel, as there is no other more efficient method to return this info.
func (c *Client) GetHost() (host string) {
	c.panicIfNotTracking()

	host = c.state.host.Load().(string)

	return host
}

// ChannelList returns the (sorted) active list of channel names that the client
// is in. Panics if tracking is disabled.
func (c *Client) ChannelList() []string {
	c.panicIfNotTracking()

	channels := make([]string, 0, len(c.state.channels.Keys()))
	for channel := range c.state.channels.IterBuffered() {
		chn := channel.Val.(*Channel)
		if !chn.UserIn(c.GetNick()) {
			continue
		}
		channels = append(channels, chn.Name)
	}

	sort.Strings(channels)
	return channels
}

// Channels returns the (sorted) active channels that the client is in. Panics
// if tracking is disabled.
func (c *Client) Channels() []*Channel {
	c.panicIfNotTracking()

	channels := make([]*Channel, 0, len(c.state.channels))
	for channel := range c.state.channels.IterBuffered() {
		chn := channel.Val.(*Channel)
		channels = append(channels, chn.Copy())
	}

	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})
	return channels
}

// UserList returns the (sorted) active list of nicknames that the client is
// tracking across all channels. Panics if tracking is disabled.
func (c *Client) UserList() []string {
	c.panicIfNotTracking()

	users := make([]string, 0, len(c.state.users))
	for user := range c.state.users.IterBuffered() {
		usr := user.Val.(*User)
		if usr.Stale {
			continue
		}
		users = append(users, usr.Nick)
	}

	sort.Strings(users)
	return users
}

// Users returns the (sorted) active users that the client is tracking across
// all channels. Panics if tracking is disabled.
func (c *Client) Users() []*User {
	c.panicIfNotTracking()

	users := make([]*User, 0, len(c.state.users))
	for user := range c.state.users.IterBuffered() {
		usr := user.Val.(*User)
		users = append(users, usr.Copy())
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].Nick < users[j].Nick
	})
	return users
}

// LookupChannel looks up a given channel in state. If the channel doesn't
// exist, nil is returned. Panics if tracking is disabled.
func (c *Client) LookupChannel(name string) (channel *Channel) {
	c.panicIfNotTracking()
	if name == "" {
		return nil
	}

	channel = c.state.lookupChannel(name).Copy()

	return channel
}

// LookupUser looks up a given user in state. If the user doesn't exist, nil
// is returned. Panics if tracking is disabled.
func (c *Client) LookupUser(nick string) (user *User) {
	c.panicIfNotTracking()
	if nick == "" {
		return nil
	}

	user = c.state.lookupUser(nick).Copy()

	return user
}

// IsInChannel returns true if the client is in channel. Panics if tracking
// is disabled.
// TODO: make sure this still works.
func (c *Client) IsInChannel(channel string) (in bool) {
	c.panicIfNotTracking()
	_, in = c.state.channels.Get(ToRFC1459(channel))
	return in
}

// GetServerOpt retrieves a server capability setting that was retrieved
// during client connection. This is also known as ISUPPORT (or RPL_PROTOCTL).
// Will panic if used when tracking has been disabled. Examples of usage:
//
//   nickLen, success := GetServerOpt("MAXNICKLEN")
//
func (c *Client) GetServerOpt(key string) (result string, ok bool) {
	c.panicIfNotTracking()

	oi, ok := c.state.serverOptions.Get(key)
	if !ok {
		return "", ok
	}

	result = oi.(string)

	if len(result) > 0 {
		ok = true
	}

	return result, ok
}

// GetServerOptions retrieves all of a server's capability settings that were retrieved
// during client connection. This is also known as ISUPPORT (or RPL_PROTOCTL).
func (c *Client) GetServerOptions() []byte {
	o := make(map[string]string)
	for opt := range c.state.serverOptions.IterBuffered() {
		o[opt.Key] = opt.Val.(string)
	}
	jcytes, _ := json.Marshal(o)
	return jcytes
}

// NetworkName returns the network identifier. E.g. "EsperNet", "ByteIRC".
// May be empty if the server does not support RPL_ISUPPORT (or RPL_PROTOCTL).
// Will panic if used when tracking has been disabled.
func (c *Client) NetworkName() (name string) {
	c.panicIfNotTracking()
	var ok bool

	if len(c.state.network.Load().(string)) > 0 {
		return c.state.network.Load().(string)
	}

	name, ok = c.GetServerOpt("NETWORK")
	if !ok {
		return c.IRCd.Network.Load().(string)
	}

	if len(name) < 1 && len(c.IRCd.Network.Load().(string)) > 1 {
		name = c.IRCd.Network.Load().(string)
	}

	return name
}

// ServerVersion returns the server software version, if the server has
// supplied this information during connection. May be empty if the server
// does not support RPL_MYINFO. Will panic if used when tracking has been
// disabled.
func (c *Client) ServerVersion() (version string) {
	c.panicIfNotTracking()

	version, _ = c.GetServerOpt("VERSION")
	return version
}

// ServerMOTD returns the servers message of the day, if the server has sent
// it upon connect. Will panic if used when tracking has been disabled.
func (c *Client) ServerMOTD() (motd string) {
	c.panicIfNotTracking()

	return c.state.motd
}

// Latency is the latency between the server and the client. This is measured
// by determining the difference in time between when we ping the server, and
// when we receive a pong.
func (c *Client) Latency() (delta time.Duration) {
	delta = c.conn.lastPong.Load().(time.Time).Sub(c.conn.lastPing.Load().(time.Time))

	if delta < 0 {
		return 0
	}

	return delta
}

// HasCapability checks if the client connection has the given capability. If
// you want the full list of capabilities, listen for the girc.CAP_ACK event.
// Will panic if used when tracking has been disabled.
func (c *Client) HasCapability(name string) (has bool) {
	c.panicIfNotTracking()

	if !c.IsConnected() {
		return false
	}

	name = strings.ToLower(name)

	c.state.RLock()
	for key := range c.state.enabledCap {
		key = strings.ToLower(key)
		if key == name {
			has = true
			break
		}
	}
	c.state.RUnlock()

	return has
}

// panicIfNotTracking will throw a panic when it's called, and tracking is
// disabled. Adds useful info like what function specifically, and where it
// was called from.
func (c *Client) panicIfNotTracking() {
	if c == nil {
		return
	}
	if !c.Config.disableTracking {
		return
	}

	pc, _, _, _ := runtime.Caller(1)
	fn := runtime.FuncForPC(pc)
	_, file, line, _ := runtime.Caller(2)

	panic(fmt.Sprintf("%s used when tracking is disabled (caller %s:%d)", fn.Name(), file, line))
}

func (c *Client) debugLogEvent(e *Event, dropped bool) {
	var prefix string

	if dropped {
		prefix = "dropping event (disconnected):"
	} else {
		prefix = ">"
	}

	if e.Sensitive {
		c.debug.Printf(prefix, " %s ***redacted***", e.Command)
	} else {
		c.debug.Print(prefix, " ", StripRaw(e.String()))
	}

	if c.Config.Out != nil {
		if pretty, ok := e.Pretty(); ok {
			fmt.Fprintln(c.Config.Out, StripRaw(pretty))
		}
	}
}
