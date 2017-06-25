// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"
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
	// lastWrite is used ot keep track of when we last wrote to the server.
	lastWrite time.Time
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
	lastPong  time.Time
	pingDelay time.Duration
}

// ErrInvalidConfig is returned when the configuration passed to the client
// is invalid.
type ErrInvalidConfig struct {
	Conf Config // Conf is the configuration that was not valid.
	err  error
}

func (e ErrInvalidConfig) Error() string { return "invalid configuration: " + e.err.Error() }

// ErrProxy is returned when an attempt to use the supplied proxy resulted
// in error, with implementation or connection.
type ErrProxy struct {
	Bind string // Bind is the query string address that was supplied.
	err  error
}

func (e ErrProxy) Error() string { return fmt.Sprintf("proxy error: %q: %s", e.Bind, e.err) }

// newConn sets up and returns a new connection to the server. This includes
// setting up things like proxies, ssl/tls, and other misc. things.
func newConn(conf Config, addr string) (*ircConn, error) {
	if err := conf.isValid(); err != nil {
		return nil, ErrInvalidConfig{conf, err}
	}

	var conn net.Conn
	var err error

	dialer := &net.Dialer{Timeout: 5 * time.Second}

	if conf.Bind != "" {
		var local *net.TCPAddr
		local, err = net.ResolveTCPAddr("tcp", conf.Bind+":0")
		if err != nil {
			return nil, err
		}

		dialer.LocalAddr = local
	}

	if conf.Proxy != "" {
		var proxyURI *url.URL
		var proxyDialer proxy.Dialer

		if proxyURI, err = url.Parse(conf.Proxy); err != nil {
			return nil, ErrProxy{conf.Proxy, err}
		}

		if proxyDialer, err = proxy.FromURL(proxyURI, dialer); err != nil {
			return nil, ErrProxy{conf.Proxy, err}
		}

		if conn, err = proxyDialer.Dial("tcp", addr); err != nil {
			return nil, ErrProxy{conf.Proxy, err}
		}
	} else {
		if conn, err = dialer.Dial("tcp", addr); err != nil {
			return nil, err
		}
	}

	if conf.SSL {
		var tlsConn net.Conn
		tlsConn, err = tlsHandshake(conn, conf.TLSConfig, conf.Server, true)
		if err != nil {
			return nil, err
		}

		conn = tlsConn
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

// ErrParseEvent is returned when an event cannot be parsed with ParseEvent().
type ErrParseEvent struct {
	Line string
}

func (e ErrParseEvent) Error() string { return "unable to parse event: " + e.Line }

func (c *ircConn) decode() (event *Event, err error) {
	line, err := c.io.ReadString(delim)
	if err != nil {
		return nil, err
	}

	if event = ParseEvent(line); event == nil {
		return nil, ErrParseEvent{line}
	}

	return event, nil
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

func tlsHandshake(conn net.Conn, conf *tls.Config, server string, validate bool) (net.Conn, error) {
	if conf == nil {
		conf = &tls.Config{ServerName: server, InsecureSkipVerify: !validate}
	}

	tlsConn := tls.Client(conn, conf)
	return net.Conn(tlsConn), nil
}

// Close closes the underlying socket.
func (c *ircConn) Close() error {
	return c.sock.Close()
}

// Connect attempts to connect to the given IRC server. Returns only when
// an error has occurred, or a disconnect was requested with Close(). Connect
// will only return once all goroutines have been closed to ensure there are
// no long-running routines becoming backed up. This also means that this
// will wait for all non-background handlers to complete.
//
// If this returns nil, this means that the client requested to be closed
// (e.g. Client.Close()). Connect will panic if called when the last call has
// not completed.
func (c *Client) Connect() error {
	return c.internalConnect(nil)
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
	return c.internalConnect(conn)
}

func (c *Client) internalConnect(mock net.Conn) error {
	// We want to be the only one handling connects/disconnects right now.
	c.mu.Lock()

	if c.conn != nil {
		panic("use of connect more than once")
	}

	// Reset the state.
	c.state.clean()

	if mock == nil {
		// Validate info, and actually make the connection.
		c.debug.Printf("connecting to %s...", c.Server())
		conn, err := newConn(c.Config, c.Server())
		if err != nil {
			c.mu.Unlock()
			return err
		}

		c.conn = conn
	} else {
		c.conn = newMockConn(mock)
	}

	var ctx context.Context
	ctx, c.stop = context.WithCancel(context.Background())

	c.mu.Unlock()

	// Start read loop to process messages from the server.
	errs := make(chan error, 3)
	done := make(chan struct{}, 4)
	var wg sync.WaitGroup
	// 4 being the number of goroutines we need to finish when this function
	// returns.
	wg.Add(4)

	go c.execLoop(done, &wg)
	go c.readLoop(errs, done, &wg)
	go c.sendLoop(errs, done, &wg)

	if mock == nil {
		go c.pingLoop(errs, done, &wg, 10*time.Second)
	} else {
		go c.pingLoop(errs, done, &wg, 1*time.Second)
	}

	// Send a virtual event allowing hooks for successful socket connection.
	c.RunHandlers(&Event{Command: INITIALIZED, Trailing: c.Server()})

	// Passwords first.
	if c.Config.ServerPass != "" {
		c.write(&Event{Command: PASS, Params: []string{c.Config.ServerPass}, Sensitive: true})
	}

	// Then nickname.
	c.write(&Event{Command: NICK, Params: []string{c.Config.Nick}})

	// Then username and realname.
	if c.Config.Name == "" {
		c.Config.Name = c.Config.User
	}

	c.write(&Event{Command: USER, Params: []string{c.Config.User, "*", "*"}, Trailing: c.Config.Name})

	// List the IRCv3 capabilities, specifically with the max protocol we
	// support.
	c.listCAP()

	// Wait for the first error.
	var result error
	select {
	case err := <-errs:
		c.debug.Print("received error, beginning clean up")
		result = err
	case <-ctx.Done():
		c.debug.Print("received request to close, beginning clean up")
	}

	c.mu.Lock()
	c.conn.mu.Lock()
	c.conn.connected = false
	c.conn.mu.Unlock()
	c.mu.Unlock()

	// Once we have our error/result, let all other functions know we're done.
	c.debug.Print("waiting for all routines to finish")
	close(done)

	// Wait for all goroutines to finish.
	wg.Wait()
	close(errs)

	// Make sure that the connection is closed if not already.
	c.mu.Lock()
	_ = c.conn.Close()

	// This helps ensure that the end user isn't improperly using the client
	// more than once. If they want to do this, they should be using multiple
	// clients, not multiple instances of Connect().
	c.conn = nil
	c.mu.Unlock()

	return result
}

// readLoop sets a timeout of 300 seconds, and then attempts to read from the
// IRC server. If there is an error, it calls Reconnect.
func (c *Client) readLoop(errs chan error, done chan struct{}, wg *sync.WaitGroup) {
	c.debug.Print("starting readLoop")
	defer c.debug.Print("closing readLoop")

	var event *Event
	var err error

	for {
		select {
		case <-done:
			wg.Done()
			return
		default:
			// c.conn.sock.SetDeadline(time.Now().Add(300 * time.Second))
			event, err = c.conn.decode()
			if err != nil {
				errs <- err
				wg.Done()
				return
			}

			c.rx <- event
		}
	}
}

// Send sends an event to the server. Use Client.RunHandlers() if you are
// simply looking to trigger handlers with an event.
func (c *Client) Send(event *Event) {
	if !c.Config.AllowFlood {
		<-time.After(c.conn.rate(event.Len()))
	}

	if c.Config.GlobalFormat && event.Trailing != "" &&
		(event.Command == PRIVMSG || event.Command == TOPIC || event.Command == NOTICE) {
		event.Trailing = Fmt(event.Trailing)
	}

	c.write(event)
}

// write is the lower level function to write an event. It does not have a
// write-delay when sending events.
func (c *Client) write(event *Event) {
	c.tx <- event
}

// rate allows limiting events based on how frequent the event is being sent,
// as well as how many characters each event has.
func (c *ircConn) rate(chars int) time.Duration {
	_time := time.Second + ((time.Duration(chars) * time.Second) / 100)

	c.mu.Lock()
	if c.writeDelay += _time - time.Now().Sub(c.lastWrite); c.writeDelay < 0 {
		c.writeDelay = 0
	}
	c.mu.Unlock()

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.writeDelay > (8 * time.Second) {
		return _time
	}

	return 0
}

func (c *Client) sendLoop(errs chan error, done chan struct{}, wg *sync.WaitGroup) {
	c.debug.Print("starting sendLoop")
	defer c.debug.Print("closing sendLoop")

	var err error

	for {
		select {
		case event := <-c.tx:
			// Check if tags exist on the event. If they do, and message-tags
			// isn't a supported capability, remove them from the event.
			if event.Tags != nil {
				c.state.mu.Lock()
				var in bool
				for i := 0; i < len(c.state.enabledCap); i++ {
					if c.state.enabledCap[i] == "message-tags" {
						in = true
						break
					}
				}
				c.state.mu.Unlock()

				if !in {
					event.Tags = Tags{}
				}
			}

			// Log the event.
			if event.Sensitive {
				c.debug.Printf("> %s ***redacted***", event.Command)
			} else {
				c.debug.Print("> ", StripRaw(event.String()))
			}
			if c.Config.Out != nil {
				if pretty, ok := event.Pretty(); ok {
					fmt.Fprintln(c.Config.Out, StripRaw(pretty))
				}
			}

			c.conn.mu.Lock()
			c.conn.lastWrite = time.Now()
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

			if err != nil {
				errs <- err
				wg.Done()
				return
			}
		case <-done:
			wg.Done()
			return
		}
	}
}

// flushTx empties c.tx.
func (c *Client) flushTx() {
	for {
		select {
		case <-c.tx:
		default:
			return
		}
	}
}

// ErrTimedOut is returned when we attempt to ping the server, and timed out
// before receiving a PONG back.
type ErrTimedOut struct {
	// TimeSinceSuccess is how long ago we received a successful pong.
	TimeSinceSuccess time.Duration
	// LastPong is the time we received our last successful pong.
	LastPong time.Time
	// LastPong is the last time we sent a pong request.
	LastPing time.Time
	// Delay is the configured delay between how often we send a ping request.
	Delay time.Duration
}

func (ErrTimedOut) Error() string { return "timed out during ping to server" }

func (c *Client) pingLoop(errs chan error, done chan struct{}, wg *sync.WaitGroup, initDelay time.Duration) {
	c.debug.Print("starting pingLoop")
	defer c.debug.Print("closing pingLoop")

	c.conn.mu.Lock()
	c.conn.lastPing = time.Now()
	c.conn.lastPong = time.Now()
	c.conn.mu.Unlock()

	// Delay during connect to wait for the client to register and what not.
	time.Sleep(initDelay)

	tick := time.NewTicker(c.Config.PingDelay)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			c.conn.mu.RLock()
			if time.Since(c.conn.lastPong) > c.Config.PingDelay+(60*time.Second) {
				// It's 60 seconds over what out ping delay is, connection
				// has probably dropped.
				errs <- ErrTimedOut{
					TimeSinceSuccess: time.Since(c.conn.lastPong),
					LastPong:         c.conn.lastPong,
					LastPing:         c.conn.lastPing,
					Delay:            c.Config.PingDelay,
				}

				wg.Done()
				c.conn.mu.RUnlock()
				return
			}
			c.conn.mu.RUnlock()

			c.conn.mu.Lock()
			c.conn.lastPing = time.Now()
			c.conn.mu.Unlock()

			c.Cmd.Ping(fmt.Sprintf("%d", time.Now().UnixNano()))
		case <-done:
			wg.Done()
			return
		}
	}
}
