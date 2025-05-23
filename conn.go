// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/lrstanley/girc/internal/ctxgroup"
)

// Messages are delimited with CR and LF line endings, we're using the last
// one to split the stream. Both are removed during parsing of the message.
const delim byte = '\n'

var endline = []byte("\r\n")

// ircConn represents an IRC network protocol connection, it consists of an
// Encoder and Decoder to manage i/o.
type ircConn struct {
	io   *bufio.ReadWriter
	sock net.Conn

	mu sync.RWMutex
	// lastWrite is used to keep track of when we last wrote to the server.
	lastWrite time.Time
	// lastActive is the last time the client was interacting with the server,
	// excluding a few background commands (PING, PONG, WHO, etc).
	lastActive time.Time
	// writeDelay is used to keep track of rate limiting of events sent to
	// the server.
	writeDelay time.Duration
	// connected is true if we're actively connected to a server.
	connected bool
	// connTime is the time at which the client has connected to a server.
	connTime *time.Time
	// lastPing is the last time that we pinged the server.
	lastPing time.Time
	// lastPong is the last successful time that we pinged the server and
	// received a successful pong back.
	lastPong time.Time
}

// Dialer is an interface implementation of net.Dialer. Use this if you would
// like to implement your own dialer which the client will use when connecting.
type Dialer interface {
	// Dial takes two arguments. Network, which should be similar to "tcp",
	// "tdp6", "udp", etc -- as well as address, which is the hostname or ip
	// of the network. Note that network can be ignored if your transport
	// doesn't take advantage of network types.
	Dial(network, address string) (net.Conn, error)
}

// newConn sets up and returns a new connection to the server.
func newConn(conf Config, dialer Dialer, addr string, sts *strictTransport) (*ircConn, error) {
	if err := conf.isValid(); err != nil {
		return nil, err
	}

	var conn net.Conn
	var err error

	if dialer == nil {
		netDialer := &net.Dialer{Timeout: 5 * time.Second}

		if conf.Bind != "" {
			var local *net.TCPAddr
			local, err = net.ResolveTCPAddr("tcp", conf.Bind+":0")
			if err != nil {
				return nil, err
			}

			netDialer.LocalAddr = local
		}

		dialer = netDialer
	}

	if conn, err = dialer.Dial("tcp", addr); err != nil {
		if sts.enabled() {
			err = &ErrSTSUpgradeFailed{Err: err}
		}

		if sts.expired() && !conf.DisableSTSFallback {
			sts.lastFailed = time.Now()
			sts.reset()
		}
		return nil, err
	}

	if conf.SSL || sts.enabled() {
		conn = tlsHandshake(conn, conf.TLSConfig, conf.Server, true)
	}

	ctime := time.Now()

	c := &ircConn{
		sock:      conn,
		connTime:  &ctime,
		connected: true,
	}
	c.newReadWriter()

	return c, nil
}

func newMockConn(conn net.Conn) *ircConn {
	ctime := time.Now()
	c := &ircConn{
		sock:      conn,
		connTime:  &ctime,
		connected: true,
	}
	c.newReadWriter()

	return c
}

// ErrParseEvent is aliased to ParseEventError.
//
// Deprecated: use ParseEventError instead.
type ErrParseEvent = ParseEventError //nolint:errname

// ParseEventError is returned when an event cannot be parsed with ParseEvent().
type ParseEventError struct {
	Line string
}

func (e ParseEventError) Error() string { return "unable to parse event: " + e.Line }

type decodedEvent struct {
	event *Event
	err   error
}

func (c *ircConn) decode() <-chan decodedEvent {
	ch := make(chan decodedEvent, 1)

	go func() {
		defer close(ch)

		line, err := c.io.ReadString(delim)
		if err != nil {
			ch <- decodedEvent{err: err}
			return
		}

		event := ParseEvent(line)
		if event == nil {
			ch <- decodedEvent{err: ErrParseEvent{Line: line}}
			return
		}

		ch <- decodedEvent{event: event}
	}()

	return ch
}

func (c *ircConn) encode(event *Event) error {
	if _, err := c.io.Write(event.Bytes()); err != nil {
		return err
	}
	if _, err := c.io.Write(endline); err != nil {
		return err
	}

	return c.io.Flush()
}

func (c *ircConn) newReadWriter() {
	c.io = bufio.NewReadWriter(bufio.NewReader(c.sock), bufio.NewWriter(c.sock))
}

func tlsHandshake(conn net.Conn, conf *tls.Config, server string, validate bool) net.Conn {
	if conf == nil {
		conf = &tls.Config{ServerName: server, InsecureSkipVerify: !validate} //nolint:gosec
	}

	tlsConn := tls.Client(conn, conf)
	return net.Conn(tlsConn)
}

// Close closes the underlying socket.
func (c *ircConn) Close() error {
	return c.sock.Close()
}

// Connect attempts to connect to the given IRC server. Returns only when
// an error has occurred, or a disconnect was requested with Close(). Connect
// will only return once all client-based goroutines have been closed to
// ensure there are no long-running routines becoming backed up.
//
// Connect will wait for all non-goroutine handlers to complete on error/quit,
// however it will not wait for goroutine-based handlers.
//
// If this returns nil, this means that the client requested to be closed
// (e.g. Client.Close()). Connect will panic if called when the last call has
// not completed.
func (c *Client) Connect() error {
	return c.internalConnect(nil, nil)
}

// DialerConnect allows you to specify your own custom dialer which implements
// the Dialer interface.
//
// An example of using this library would be to take advantage of the
// golang.org/x/net/proxy library:
//
//	proxyUrl, _ := proxyURI, err = url.Parse("socks5://1.2.3.4:8888")
//	dialer, _ := proxy.FromURL(proxyURI, &net.Dialer{Timeout: 5 * time.Second})
//	_ := girc.DialerConnect(dialer)
func (c *Client) DialerConnect(dialer Dialer) error {
	return c.internalConnect(nil, dialer)
}

// MockConnect is used to implement mocking with an IRC server. Supply a net.Conn
// that will be used to spoof the server. A useful way to do this is to so
// net.Pipe(), pass one end into MockConnect(), and the other end into
// bufio.NewReader().
//
// For example:
//
//	client := girc.New(girc.Config{
//		Server: "dummy.int",
//		Port:   6667,
//		Nick:   "test",
//		User:   "test",
//		Name:   "Testing123",
//	})
//
//	in, out := net.Pipe()
//	defer in.Close()
//	defer out.Close()
//	b := bufio.NewReader(in)
//
//	go func() {
//		if err := client.MockConnect(out); err != nil {
//			panic(err)
//		}
//	}()
//
//	defer client.Close(false)
//
//	for {
//		in.SetReadDeadline(time.Now().Add(300 * time.Second))
//		line, err := b.ReadString(byte('\n'))
//		if err != nil {
//			panic(err)
//		}
//
//		event := girc.ParseEvent(line)
//
//		if event == nil {
//	 		continue
//	 	}
//
//	 	// Do stuff with event here.
//	 }
func (c *Client) MockConnect(conn net.Conn) error {
	return c.internalConnect(conn, nil)
}

func (c *Client) internalConnect(mock net.Conn, dialer Dialer) error {
startConn:
	// We want to be the only one handling connects/disconnects right now.
	c.mu.Lock()

	if c.conn != nil {
		panic("use of connect more than once")
	}

	// Reset the state.
	c.state.reset(false)

	addr := c.server()

	if mock == nil {
		// Validate info, and actually make the connection.
		c.debug.Printf("connecting to %s... (sts: %v, config-ssl: %v)", addr, c.state.sts.enabled(), c.Config.SSL)
		conn, err := newConn(c.Config, dialer, addr, &c.state.sts)
		if err != nil {
			errSTSUpgradeFailed := &ErrSTSUpgradeFailed{}
			if errors.As(err, &errSTSUpgradeFailed) {
				if !c.state.sts.enabled() {
					c.RunHandlers(&Event{Command: STS_ERR_FALLBACK})
				}
			}
			c.mu.Unlock()
			return err
		}

		c.conn = conn
	} else {
		c.conn = newMockConn(mock)
	}
	c.mu.Unlock()

	var ctx context.Context
	ctx, c.stop = context.WithCancel(context.Background())

	group := ctxgroup.New(ctx)

	group.Go(c.execLoop)
	group.Go(c.readLoop)
	group.Go(c.sendLoop)
	group.Go(c.pingLoop)

	// Passwords first.

	if c.Config.WebIRC.Password != "" {
		c.write(&Event{Command: WEBIRC, Params: c.Config.WebIRC.Params(), Sensitive: true})
	}

	if c.Config.ServerPass != "" {
		c.write(&Event{Command: PASS, Params: []string{c.Config.ServerPass}, Sensitive: true})
	}

	// List the IRCv3 capabilities, specifically with the max protocol we
	// support. The IRCv3 specification doesn't directly state if this should
	// be called directly before registration, or if it should be called
	// after NICK/USER requests. It looks like non-supporting networks
	// should ignore this, and some IRCv3 capable networks require this to
	// occur before NICK/USER registration.
	c.listCAP()

	// Then nickname.
	c.write(&Event{Command: NICK, Params: []string{c.Config.Nick}})

	// Then username and realname.
	if c.Config.Name == "" {
		c.Config.Name = c.Config.User
	}

	c.write(&Event{Command: USER, Params: []string{c.Config.User, "*", "*", c.Config.Name}})

	// Send a virtual event allowing hooks for successful socket connection.
	c.RunHandlers(&Event{Command: INITIALIZED, Params: []string{addr}})

	// Wait for the first error.
	err := group.Wait()
	if err != nil {
		c.debug.Printf("received error, beginning cleanup: %v", err)
	} else {
		if !c.state.sts.beginUpgrade {
			c.debug.Print("received request to close, beginning clean up")
		}

		c.RunHandlers(&Event{Command: CLOSED, Params: []string{addr}})
	}

	// Make sure that the connection is closed if not already.
	c.mu.RLock()
	if c.stop != nil {
		c.stop()
	}
	c.conn.mu.Lock()
	c.conn.connected = false
	_ = c.conn.Close()
	c.conn.mu.Unlock()
	c.mu.RUnlock()

	c.RunHandlers(&Event{Command: DISCONNECTED, Params: []string{addr}})

	// This helps ensure that the end user isn't improperly using the client
	// more than once. If they want to do this, they should be using multiple
	// clients, not multiple instances of Connect().
	c.mu.Lock()
	c.conn = nil

	if err == nil {
		if c.state.sts.beginUpgrade {
			c.state.sts.beginUpgrade = false
			c.mu.Unlock()
			goto startConn
		}

		if c.state.sts.enabled() {
			c.state.sts.persistenceReceived = time.Now()
		}
	}
	c.mu.Unlock()

	return err
}

// readLoop sets a timeout of 300 seconds, and then attempts to read from the
// IRC server. If there is an error, it calls Reconnect.
func (c *Client) readLoop(ctx context.Context) error {
	c.debug.Print("starting readLoop")
	defer c.debug.Print("closing readLoop")

	var de decodedEvent

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_ = c.conn.sock.SetReadDeadline(time.Now().Add(300 * time.Second))

			select {
			case <-ctx.Done():
				return nil
			case de = <-c.conn.decode():
			}

			if de.err != nil {
				return de.err
			}

			// Check if it's an echo-message.
			if !c.Config.disableTracking {
				de.event.Echo = (de.event.Command == PRIVMSG || de.event.Command == NOTICE) &&
					de.event.Source != nil && de.event.Source.ID() == c.GetID()
			}

			c.receive(de.event)
		}
	}
}

// Send sends an event to the server. Send will split events if the event is longer
// than what the server supports, and is an event that supports splitting. Use
// Client.RunHandlers() if you are simply looking to trigger handlers with an event.
func (c *Client) Send(event *Event) {
	var delay time.Duration

	if c.Config.GlobalFormat && len(event.Params) > 0 && event.Params[len(event.Params)-1] != "" &&
		(event.Command == PRIVMSG || event.Command == TOPIC || event.Command == NOTICE) {
		event.Params[len(event.Params)-1] = Fmt(event.Params[len(event.Params)-1])
	}

	events := event.split(c.MaxEventLength())

	for _, e := range events {
		if !c.Config.AllowFlood {
			c.mu.RLock()

			// Drop the event early as we're disconnected, this way we don't have to wait
			// the (potentially long) rate limit delay before dropping.
			if c.conn == nil {
				c.debugLogEvent(e, true)
				c.mu.RUnlock()
				return
			}

			c.conn.mu.Lock()
			delay = c.conn.rate(e.Len())
			c.conn.mu.Unlock()
			c.mu.RUnlock()
		}

		<-time.After(delay)
		c.write(e)
	}
}

// write is the lower level function to write an event. It does not have a
// write-delay when sending events. write will timeout after 30s if the event
// can't be sent.
func (c *Client) write(event *Event) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		// Drop the event if disconnected.
		c.debugLogEvent(event, true)
		return
	}

	t := time.NewTimer(30 * time.Second)
	defer t.Stop()

	select {
	case c.tx <- event:
	case <-t.C:
		c.debugLogEvent(event, true)
	}
}

// rate allows limiting events based on how frequent the event is being sent,
// as well as how many characters each event has.
func (c *ircConn) rate(chars int) time.Duration {
	_time := time.Second + ((time.Duration(chars) * time.Second) / 100)

	if c.writeDelay += _time - time.Since(c.lastWrite); c.writeDelay < 0 {
		c.writeDelay = 0
	}

	if c.writeDelay > (8 * time.Second) {
		return _time
	}

	return 0
}

func (c *Client) sendLoop(ctx context.Context) error {
	c.debug.Print("starting sendLoop")
	defer c.debug.Print("closing sendLoop")

	var err error

	for {
		select {
		case event := <-c.tx:
			// Check if tags exist on the event. If they do, and message-tags
			// isn't a supported capability, remove them from the event.
			if event.Tags != nil {
				c.state.RLock()
				var in bool
				for i := 0; i < len(c.state.enabledCap); i++ {
					if _, ok := c.state.enabledCap["message-tags"]; ok {
						in = true
						break
					}
				}
				c.state.RUnlock()

				if !in {
					event.Tags = Tags{}
				}
			}

			c.debugLogEvent(event, false)

			c.conn.mu.Lock()
			c.conn.lastWrite = time.Now()

			if event.Command != PING && event.Command != PONG && event.Command != WHO {
				c.conn.lastActive = c.conn.lastWrite
			}
			c.conn.mu.Unlock()

			// Write the raw line.
			_, err = c.conn.io.Write(event.Bytes())
			if err == nil {
				// And the \r\n.
				_, err = c.conn.io.Write(endline)
				if err == nil {
					// Lastly, flush everything to the socket.
					err = c.conn.io.Flush()
				}
			}

			if event.Command == QUIT {
				c.Close()
				return nil
			}

			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// ErrTimedOut is aliased to TimedOutError.
//
// Deprecated: use TimedOutError instead.
type ErrTimedOut = TimedOutError //nolint:errname

// TimedOutError is returned when we attempt to ping the server, and timed out
// before receiving a PONG back.
type TimedOutError struct {
	// TimeSinceSuccess is how long ago we received a successful pong.
	TimeSinceSuccess time.Duration
	// LastPong is the time we received our last successful pong.
	LastPong time.Time
	// LastPong is the last time we sent a pong request.
	LastPing time.Time
	// Delay is the configured delay between how often we send a ping request.
	Delay time.Duration
}

func (TimedOutError) Error() string { return "timed out waiting for a requested PING response" }

func (c *Client) pingLoop(ctx context.Context) error {
	// Don't run the pingLoop if they want to disable it.
	if c.Config.PingDelay <= 0 {
		return nil
	}

	c.debug.Print("starting pingLoop")
	defer c.debug.Print("closing pingLoop")

	c.conn.mu.Lock()
	c.conn.lastPing = time.Now()
	c.conn.lastPong = time.Now()
	c.conn.mu.Unlock()

	tick := time.NewTicker(c.Config.PingDelay)
	defer tick.Stop()

	started := time.Now()
	past := false
	pingSent := false

	for {
		select {
		case <-tick.C:
			// Delay during connect to wait for the client to register, otherwise
			// some ircd's will not respond (e.g. during SASL negotiation).
			if !past {
				if time.Since(started) < 30*time.Second {
					continue
				}

				past = true
			}

			c.conn.mu.RLock()
			if pingSent && time.Since(c.conn.lastPong) > c.Config.PingDelay+c.Config.PingTimeout {
				// PingTimeout exceeded, connection has probably dropped.
				err := ErrTimedOut{
					TimeSinceSuccess: time.Since(c.conn.lastPong),
					LastPong:         c.conn.lastPong,
					LastPing:         c.conn.lastPing,
					Delay:            c.Config.PingDelay,
				}

				c.conn.mu.RUnlock()
				return err
			}
			c.conn.mu.RUnlock()

			c.conn.mu.Lock()
			c.conn.lastPing = time.Now()
			c.conn.mu.Unlock()

			c.Cmd.Ping(strconv.FormatInt(time.Now().UnixNano(), 10))
			pingSent = true
		case <-ctx.Done():
			return nil
		}
	}
}
