// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bytes"
	"strings"
)

const (
	prefix     byte = 0x3A // prefix or last argument
	prefixUser byte = 0x21 // username
	prefixHost byte = 0x40 // hostname
)

// Source represents the sender of an IRC event, see RFC1459 section 2.3.1
// <servername> | <nick> [ '!' <user> ] [ '@' <host> ]
type Source struct {
	Name string // Nick or servername
	User string // Username
	Host string // Hostname
}

// ParseSource takes a string and attempts to create a Source struct.
func ParseSource(raw string) (src *Source) {
	src = new(Source)

	user := strings.IndexByte(raw, prefixUser)
	host := strings.IndexByte(raw, prefixHost)

	switch {
	case user > 0 && host > user:
		src.Name = raw[:user]
		src.User = raw[user+1 : host]
		src.Host = raw[host+1:]
	case user > 0:
		src.Name = raw[:user]
		src.User = raw[user+1:]
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
	if len(s.User) > 0 {
		length = 1 + length + len(s.User)
	}
	if len(s.Host) > 0 {
		length = 1 + length + len(s.Host)
	}

	return
}

// Bytes returns a []byte representation of prefix
func (s *Source) Bytes() []byte {
	buffer := new(bytes.Buffer)
	s.writeTo(buffer)

	return buffer.Bytes()
}

// String returns a string representation of prefix
func (s *Source) String() (out string) {
	out = s.Name
	if len(s.User) > 0 {
		out = out + string(prefixUser) + s.User
	}
	if len(s.Host) > 0 {
		out = out + string(prefixHost) + s.Host
	}

	return
}

// IsHostmask returns true if prefix looks like a user hostmask
func (s *Source) IsHostmask() bool {
	return len(s.User) > 0 && len(s.Host) > 0
}

// IsServer returns true if this prefix looks like a server name.
func (s *Source) IsServer() bool {
	return len(s.User) <= 0 && len(s.Host) <= 0
}

// writeTo is an utility function to write the prefix to the bytes.Buffer in Event.String()
func (s *Source) writeTo(buffer *bytes.Buffer) {
	buffer.WriteString(s.Name)
	if len(s.User) > 0 {
		buffer.WriteByte(prefixUser)
		buffer.WriteString(s.User)
	}
	if len(s.Host) > 0 {
		buffer.WriteByte(prefixHost)
		buffer.WriteString(s.Host)
	}

	return
}

// IsValidChannel checks if channel is an RFC complaint channel or not
//
// channel      =  ( "#" / "+" / ( "!" channelid ) / "&" ) chanstring
//                 [ ":" chanstring ]
//   chanstring =  0x01-0x07 / 0x08-0x09 / 0x0B-0x0C / 0x0E-0x1F / 0x21-0x2B
//   chanstring =  / 0x2D-0x39 / 0x3B-0xFF
//                   ; any octet except NUL, BELL, CR, LF, " ", "," and ":"
//   channelid  = 5( 0x41-0x5A / digit )   ; 5( A-Z / 0-9 )
func IsValidChannel(channel string) bool {
	if len(channel) <= 1 || len(channel) > 50 {
		return false
	}

	// #, +, !<channelid>, or &
	// Including "*" in the prefix list, as this is commonly used (e.g. ZNC)
	if bytes.IndexByte([]byte{0x21, 0x23, 0x26, 0x2A, 0x2B}, channel[0]) == -1 {
		return false
	}

	// !<channelid> -- not very commonly supported, but we'll check it anyway.
	// The ID must be 5 chars. This means min-channel size should be:
	//   1 (prefix) + 5 (id) + 1 (+, channel name)
	if channel[0] == 0x21 {
		if len(channel) < 7 {
			return false
		}

		// check for valid ID
		for i := 1; i < 6; i++ {
			if (channel[i] < 0x30 || channel[i] > 0x39) && (channel[i] < 0x41 || channel[i] > 0x5A) {
				return false
			}
		}
	}

	// Check for invalid octets here.
	bad := []byte{0x00, 0x07, 0x0D, 0x0A, 0x20, 0x2C, 0x3A}
	for i := 1; i < len(channel); i++ {
		if bytes.IndexByte(bad, channel[i]) != -1 {
			return false
		}
	}

	return true
}

// IsValidNick valids an IRC nickame. Note that this does not valid IRC
// nickname length.
//
// nickname   =  ( letter / special ) *8( letter / digit / special / "-" )
//   letter   =  0x41-0x5A / 0x61-0x7A
//   digit    =  0x30-0x39
//   special  =  0x5B-0x60 / 0x7B-0x7D
func IsValidNick(nick string) bool {
	if len(nick) <= 0 {
		return false
	}

	// Check the first index. Some characters aren't allowed for the first
	// index of an IRC nickname.
	if nick[0] < 0x41 || nick[0] > 0x7D {
		// a-z, A-Z, and _\[]{}^|
		return false
	}

	for i := 1; i < len(nick); i++ {
		if (nick[i] < 0x41 || nick[i] > 0x7D) && (nick[i] < 0x30 || nick[i] > 0x39) && nick[i] != 0x2D {
			// a-z, A-Z, 0-9, -, and _\[]{}^|
			return false
		}
	}

	return true
}
