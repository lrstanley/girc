// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bufio"
	"io"
	"sync"
)

// messages are delimited with CR and LF line endings, we're using the last
// one to split the stream. both are removed during parsing of the message.
const delim byte = '\n'

var endline = []byte("\r\n")

// conn represents an IRC network protocol connection, it consists of an
// Encoder and Decoder to manage i/o.
type ircConn struct {
	ircEncoder
	ircDecoder

	c io.ReadWriteCloser
}

// Close closes the underlying ReadWriteCloser.
func (c *ircConn) Close() error {
	return c.c.Close()
}

// ircDecoder reads Event objects from an input stream.
type ircDecoder struct {
	reader *bufio.Reader
	line   string
	mu     sync.Mutex
}

// newDecoder returns a new Decoder that reads from r.
func newDecoder(r io.Reader) *ircDecoder {
	return &ircDecoder{reader: bufio.NewReader(r)}
}

// Decode attempts to read a single Event from the stream, returns non-nil
// error if read failed.
func (dec *ircDecoder) Decode() (e *Event, err error) {
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

// newEncoder returns a new Encoder that writes to w.
func newEncoder(w io.Writer) *ircEncoder {
	return &ircEncoder{writer: w}
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
