// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bufio"
	"io"
	"net"
	"sync"
)

// messages are delimited with CR and LF line endings, we're using the last
// one to split the stream. both are removed during parsing of the message.
const delim byte = '\n'

var endline = []byte("\r\n")

// Conn represents an IRC network protocol connection, it consists of an
// Encoder and Decoder to manage i/o
type Conn struct {
	Encoder
	Decoder

	conn io.ReadWriteCloser
}

// NewConn returns a new Conn using rwc for i/o
func NewConn(rwc io.ReadWriteCloser) *Conn {
	return &Conn{
		Encoder: Encoder{writer: rwc},
		Decoder: Decoder{reader: bufio.NewReader(rwc)},
		conn:    rwc,
	}
}

// Dial connects to the given address using net.Dial and then returns a
// new Conn for the connection
func Dial(addr string) (*Conn, error) {
	c, err := net.Dial("tcp", addr)

	if err != nil {
		return nil, err
	}

	return NewConn(c), nil
}

// Close closes the underlying ReadWriteCloser
func (c *Conn) Close() error {
	return c.conn.Close()
}

// A Decoder reads Event objects from an input stream
type Decoder struct {
	reader *bufio.Reader
	line   string
	mu     sync.Mutex
}

// NewDecoder returns a new Decoder that reads from r
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{reader: bufio.NewReader(r)}
}

// Decode attempts to read a single Event from the stream, returns non-nil
// error if read failed
func (dec *Decoder) Decode() (e *Event, err error) {
	dec.mu.Lock()
	dec.line, err = dec.reader.ReadString(delim)
	dec.mu.Unlock()

	if err != nil {
		return nil, err
	}

	return ParseEvent(dec.line), nil
}

// Encoder writes Event objects to an output stream
type Encoder struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewEncoder returns a new Encoder that writes to w
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{writer: w}
}

// Encode writes the IRC encoding of m to the stream. goroutine safe.
// returns non-nil error if the write to the underlying stream stopped early.
func (enc *Encoder) Encode(e *Event) (err error) {
	_, err = enc.Write(e.Bytes())

	return
}

// Write writes len(p) bytes from p followed by CR+LF. goroutine safe.
func (enc *Encoder) Write(p []byte) (n int, err error) {
	enc.mu.Lock()
	defer enc.mu.Unlock()

	n, err = enc.writer.Write(p)
	if err != nil {
		return
	}

	_, err = enc.writer.Write(endline)

	return
}
