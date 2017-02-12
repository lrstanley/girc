// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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
	ircEncoder
	ircDecoder

	lconn net.Conn
}

func newConn(conf Config, addr string) (*ircConn, error) {
	// Sanity check a few options.
	if conf.Server == "" {
		return nil, errors.New("invalid server specified")
	}

	if conf.Port < 21 || conf.Port > 65535 {
		return nil, errors.New("invalid port (21-65535)")
	}

	if !IsValidNick(conf.Nick) || !IsValidUser(conf.User) {
		return nil, errors.New("invalid nickname or user")
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
		var proxyUri *url.URL
		var proxyDialer proxy.Dialer

		proxyUri, err = url.Parse(conf.Proxy)
		if err != nil {
			return nil, fmt.Errorf("unable to use proxy %q: %s", conf.Proxy, err)
		}

		proxyDialer, err = proxy.FromURL(proxyUri, dialer)
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
		var sslConf *tls.Config

		if conf.TLSConfig == nil {
			sslConf = &tls.Config{ServerName: conf.Server}
		} else {
			sslConf = conf.TLSConfig
		}

		tlsConn := tls.Client(conn, sslConf)
		if err = tlsConn.Handshake(); err != nil {
			return nil, fmt.Errorf("failed handshake during tls conn to %q: %s", addr, err)
		}
		conn = tlsConn
	}

	return &ircConn{
		ircEncoder: ircEncoder{writer: conn},
		ircDecoder: ircDecoder{reader: bufio.NewReader(conn)},
		lconn:      conn,
	}, nil
}

// Close closes the underlying ReadWriteCloser.
func (c *ircConn) Close() error {
	return c.lconn.Close()
}

// ircDecoder reads Event objects from an input stream.
type ircDecoder struct {
	reader *bufio.Reader
	line   string
	mu     sync.Mutex
}

// Decode attempts to read a single Event from the stream, returns non-nil
// error if read failed. event may be nil if unparseable.
func (dec *ircDecoder) Decode() (event *Event, err error) {
	dec.mu.Lock()
	dec.line, err = dec.reader.ReadString(delim)
	dec.mu.Unlock()

	if err != nil {
		return nil, err
	}

	return ParseEvent(dec.line), nil
}

// ircEncoder writes Event objects to an output stream.
type ircEncoder struct {
	writer io.Writer
	mu     sync.Mutex
}

// Encode writes the IRC encoding of m to the stream. Goroutine safe.
// returns non-nil error if the write to the underlying stream stopped early.
func (enc *ircEncoder) Encode(e *Event) (err error) {
	_, err = enc.Write(e.Bytes())

	return
}

// Write writes len(p) bytes from p followed by CR+LF. Goroutine safe.
func (enc *ircEncoder) Write(p []byte) (n int, err error) {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	n, err = enc.writer.Write(p)
	if err != nil {
		return
	}

	_, err = enc.writer.Write(endline)

	return
}
