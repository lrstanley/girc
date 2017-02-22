// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"golang.org/x/net/context"
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

// newConn sets up and returns a new connection to the server. This includes
// setting up things like proxies, ssl/tls, and other misc. things.
func newConn(conf Config, addr string) (*ircConn, error) {
	if err := conf.isValid(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %s", err)
	}

	var conn net.Conn
	var err error

	dialer := &net.Dialer{Timeout: 5 * time.Second}

	if conf.Bind != "" {
		var local *net.TCPAddr
		local, err = net.ResolveTCPAddr("tcp", conf.Bind+":0")
		if err != nil {
			return nil, fmt.Errorf("unable to resolve bind address %s: %s", conf.Bind, err)
		}

		dialer.LocalAddr = local
	}

	if conf.Proxy != "" {
		var proxyURI *url.URL
		var proxyDialer proxy.Dialer

		proxyURI, err = url.Parse(conf.Proxy)
		if err != nil {
			return nil, fmt.Errorf("unable to use proxy %q: %s", conf.Proxy, err)
		}

		proxyDialer, err = proxy.FromURL(proxyURI, dialer)
		if err != nil {
			return nil, fmt.Errorf("unable to use proxy %q: %s", conf.Proxy, err)
		}

		conn, err = proxyDialer.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to proxy %q: %s", conf.Proxy, err)
		}
	} else {
		conn, err = dialer.Dial("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("unable to connect to %q: %s", addr, err)
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

func (c *ircConn) decode() (event *Event, err error) {
	line, err := c.io.ReadString(delim)
	if err != nil {
		return nil, err
	}

	event = ParseEvent(line)
	if event == nil {
		return nil, fmt.Errorf("unable to parse incoming event: %s", event)
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

func (c *Client) startTLSHandshake() {
	if c.Config.Out != nil {
		fmt.Fprintln(c.Config.Out, "[*] attempting to upgrade connection to SSL/TLS")
	}

	c.debug.Print("initiating STARTTLS handshake")
	// Success -- the server supports STARTTLS, and it's awaiting our ACK
	// and/or handshake before proceeding. Upgrade the connection, then
	// let our first read with the new connection make the handshake for us.
	_, done := c.Handlers.AddTmp(RPL_STARTTLS, 3*time.Second, func(client *Client, e Event) bool {
		defer func() {
			// Initiate a new readLoop, as the previous one gets closed upon
			// receiving a RPL_STARTTLS.
			var rctx context.Context
			rctx, c.closeRead = context.WithCancel(context.Background())
			go c.readLoop(rctx)
		}()

		tlsConn, err := tlsHandshake(c.conn.sock, c.Config.TLSConfig, c.Config.Server, false)
		if err != nil {
			c.debug.Printf("unable to complete tls upgrade: %s", err)
			return true
		}

		c.conn.sock = tlsConn
		c.conn.newReadWriter()

		if c.Config.Out != nil {
			fmt.Fprintln(c.Config.Out, "[*] successfully completed TLS/SSL handshake")
		}
		c.debug.Print("completed tls upgrade")
		return true
	})

	// If for some reason the server has an error. This should never occur.
	fCuid, fail := c.Handlers.AddTmp(ERR_STARTTLS, 0, func(client *Client, e Event) bool {
		c.debug.Print("unable to complete tls upgrade: server returned error")
		return true
	})
	defer c.Handlers.Remove(fCuid)

	// Send the request to see if we can handshake. Once a server that
	// supports STARTTLS sees this, it should completely halt any other
	// commands/responses, other than success/fail, and should wait for the
	// connection to be upgraded/the handshake to go through.
	c.conn.encode(&Event{Command: STARTTLS})

	// Wait for either a success, or fail. Whichever is first.
	select {
	case <-done:
		return
	case <-fail:
		return
	}
}

// Close closes the underlying socket.
func (c *ircConn) Close() error {
	return c.sock.Close()
}

// Connect attempts to connect to the given IRC server
func (c *Client) Connect() error {
	// Clean up any old running stuff.
	c.cleanup(false)

	// We want to be the only one handling connects/disconnects right now.
	c.cmux.Lock()

	// Reset the state.
	c.state = newState()

	// Validate info, and actually make the connection.
	c.debug.Printf("connecting to %s...", c.Server())
	conn, err := newConn(c.Config, c.Server())
	if err != nil {
		c.cmux.Unlock()
		return err
	}

	c.conn = conn
	c.cmux.Unlock()

	// Start read loop to process messages from the server.
	var rctx, ectx, sctx, pctx context.Context
	rctx, c.closeRead = context.WithCancel(context.Background())
	ectx, c.closeExec = context.WithCancel(context.Background())
	sctx, c.closeSend = context.WithCancel(context.Background())
	pctx, c.closePing = context.WithCancel(context.Background())
	go c.execLoop(ectx)
	go c.readLoop(rctx)
	go c.pingLoop(pctx)
	go c.sendLoop(sctx)

	// Initiate a STARTTLS based handshake, to optionally upgrade the
	// connection, without user interaction. Even though this has flaws (and
	// one should really explicitly use SSL: true, and TLSConfig), it would
	// be better than plain text.
	if !c.Config.SSL && !c.Config.DisableSTARTTLS {
		c.startTLSHandshake()
	}

	// Send a virtual event allowing hooks for successful socket connection.
	c.RunHandlers(&Event{Command: INITIALIZED, Trailing: c.Server()})

	// Passwords first.
	if c.Config.Password != "" {
		c.write(&Event{Command: PASS, Params: []string{c.Config.Password}})
	}

	// Then nickname.
	c.write(&Event{Command: NICK, Params: []string{c.Config.Nick}})

	// Then username and realname.
	if c.Config.Name == "" {
		c.Config.Name = c.Config.User
	}

	c.write(&Event{Command: USER, Params: []string{c.Config.User, "+iw", "*"}, Trailing: c.Config.Name})

	// List the IRCv3 capabilities, specifically with the max protocol we
	// support.
	c.listCAP()

	// Consider the connection a success at this point.
	c.tries = 0
	c.reconnecting = false

	return nil
}

// reconnect is the internal wrapper for reconnecting to the IRC server (if
// requested.)
func (c *Client) reconnect(remoteInvoked bool) (err error) {
	if c.reconnecting {
		return nil
	}
	c.reconnecting = true
	defer func() {
		c.reconnecting = false
	}()

	c.cleanup(false)

	if c.Config.ReconnectDelay < (5 * time.Second) {
		c.Config.ReconnectDelay = 5 * time.Second
	}

	if c.Config.Retries < 1 && !remoteInvoked {
		return ErrDisconnected
	}

	if !remoteInvoked {
		// Delay so we're not slaughtering the server with a bunch of
		// connections.
		c.debug.Printf("reconnecting to %s in %s", c.Server(), c.Config.ReconnectDelay)
		time.Sleep(c.Config.ReconnectDelay)
	}

	for err = c.Connect(); err != nil && c.tries < c.Config.Retries; c.tries++ {
		c.debug.Printf("reconnecting to %s in %s (%d tries)", c.Server(), c.Config.ReconnectDelay, c.tries)
		time.Sleep(c.Config.ReconnectDelay)
	}

	if err != nil {
		// Too many errors at this point.
		c.cleanup(false)
	}

	return err
}

// Reconnect checks to make sure we want to, and then attempts to reconnect
// to the server. This will ignore the reconnect delay.
func (c *Client) Reconnect() error {
	return c.reconnect(true)
}

func (c *Client) disconnectHandler(err error) {
	if err != nil {
		c.debug.Println("disconnecting due to error: " + err.Error())
	}

	rerr := c.reconnect(false)
	if rerr != nil {
		c.debug.Println("error: " + rerr.Error())
		if c.Config.HandleError != nil {
			if c.Config.Retries < 1 {
				c.Config.HandleError(err)
			}

			c.Config.HandleError(rerr)
		}
	}
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
			// c.conn.sock.SetDeadline(time.Now().Add(300 * time.Second))
			event, err = c.conn.decode()
			if err != nil {
				// Attempt a reconnect (if applicable). If it fails, send
				// the error to c.Config.HandleError to be dealt with, if
				// the handler exists.
				c.disconnectHandler(err)
				return
			}

			c.rx <- event

			// Why do we do this? -- If we let readLoop continue to try and
			// read from the current socket, it may not pause long enough
			// to start reading from the upgraded socket. In short, once we
			// confirmed from the server that we can upgrade via STARTTLS,
			// stop reading, upgrade, then let the STARTTLS process initiate
			// a new readLoop.
			if event.Command == RPL_STARTTLS && !c.Config.DisableSTARTTLS {
				c.debug.Print("exiting readLoop due to STARTTLS")
				return
			}
		}
	}
}

// Send sends an event to the server. Use Client.RunHandlers() if you are
// simply looking to trigger handlers with an event.
func (c *Client) Send(event *Event) {
	if !c.Config.AllowFlood {
		<-time.After(c.conn.rate(event.Len()))
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
	if c.writeDelay += _time - time.Now().Sub(c.lastWrite); c.writeDelay < 0 {
		c.writeDelay = 0
	}

	if c.writeDelay > (8 * time.Second) {
		return _time
	}

	return 0
}

func (c *Client) sendLoop(ctx context.Context) {
	var err error

	for {
		select {
		case <-ctx.Done():
			return
		case event := <-c.tx:
			// Log the event.
			if !event.Sensitive {
				c.debug.Print("> ", StripRaw(event.String()))
			}
			if c.Config.Out != nil {
				if pretty, ok := event.Pretty(); ok {
					fmt.Fprintln(c.Config.Out, StripRaw(pretty))
				}
			}

			c.conn.lastWrite = time.Now()

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
				c.disconnectHandler(err)
			}
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

// ErrTimedOut is returned when we attempt to ping the server, and time out
// before receiving a PONG back.
var ErrTimedOut = errors.New("timed out during ping to server")

func (c *Client) pingLoop(ctx context.Context) {
	c.conn.lastPing = time.Now()
	c.conn.lastPong = time.Now()

	// Delay for 30 seconds during connect to wait for the client to register
	// and what not.
	time.Sleep(20 * time.Second)

	tick := time.NewTicker(c.Config.PingDelay)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if time.Since(c.conn.lastPong) > c.Config.PingDelay+(60*time.Second) {
				// It's 60 seconds over what out ping delay is, connection
				// has probably dropped.
				c.disconnectHandler(ErrTimedOut)
				return
			}

			c.conn.lastPing = time.Now()
			c.Commands.Ping(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
	}
}
