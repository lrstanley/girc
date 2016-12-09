// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bytes"
	"strings"
)

const (
	prefix     byte = 0x3A // ":" -- prefix or last argument
	prefixUser byte = 0x21 // "!" -- username
	prefixHost byte = 0x40 // "@" -- hostname
)

// Source represents the sender of an IRC event, see RFC1459 section 2.3.1.
// <servername> | <nick> [ '!' <user> ] [ '@' <host> ]
type Source struct {
	// Name is the nickname, server name, or service name.
	Name string
	// Ident is commonly known as the "user".
	Ident string
	// Host is the hostname or IP address of the user/service. Is not accurate
	// due to how IRC servers can spoof hostnames.
	Host string
}

// ParseSource takes a string and attempts to create a Source struct.
func ParseSource(raw string) (src *Source) {
	src = new(Source)

	user := strings.IndexByte(raw, prefixUser)
	host := strings.IndexByte(raw, prefixHost)

	switch {
	case user > 0 && host > user:
		src.Name = raw[:user]
		src.Ident = raw[user+1 : host]
		src.Host = raw[host+1:]
	case user > 0:
		src.Name = raw[:user]
		src.Ident = raw[user+1:]
	case host > 0:
		src.Name = raw[:host]
		src.Host = raw[host+1:]
	default:
		src.Name = raw
	}

	return src
}

// Len calculates the length of the string representation of prefix
func (s *Source) Len() (length int) {
	length = len(s.Name)
	if len(s.Ident) > 0 {
		length = 1 + length + len(s.Ident)
	}
	if len(s.Host) > 0 {
		length = 1 + length + len(s.Host)
	}

	return
}

// Bytes returns a []byte representation of source.
func (s *Source) Bytes() []byte {
	buffer := new(bytes.Buffer)
	s.writeTo(buffer)

	return buffer.Bytes()
}

// String returns a string representation of source.
func (s *Source) String() (out string) {
	out = s.Name
	if len(s.Ident) > 0 {
		out = out + string(prefixUser) + s.Ident
	}
	if len(s.Host) > 0 {
		out = out + string(prefixHost) + s.Host
	}

	return
}

// IsHostmask returns true if source looks like a user hostmask.
func (s *Source) IsHostmask() bool {
	return len(s.Ident) > 0 && len(s.Host) > 0
}

// IsServer returns true if this source looks like a server name.
func (s *Source) IsServer() bool {
	return len(s.Ident) <= 0 && len(s.Host) <= 0
}

// writeTo is an utility function to write the source to the bytes.Buffer
// in Event.String().
func (s *Source) writeTo(buffer *bytes.Buffer) {
	buffer.WriteString(s.Name)
	if len(s.Ident) > 0 {
		buffer.WriteByte(prefixUser)
		buffer.WriteString(s.Ident)
	}
	if len(s.Host) > 0 {
		buffer.WriteByte(prefixHost)
		buffer.WriteString(s.Host)
	}

	return
}
