// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	space     byte = 0x20 // separator
	maxLength      = 510  // maximum length is 510 (2 for line endings)
)

// cutCRFunc is used to trim CR characters from prefixes/messages.
func cutCRFunc(r rune) bool {
	return r == '\r' || r == '\n'
}

// Event represents an IRC protocol message, see RFC1459 section 2.3.1
//
//    <message>  :: [':' <prefix> <SPACE>] <command> <params> <crlf>
//    <prefix>   :: <servername> | <nick> ['!' <user>] ['@' <host>]
//    <command>  :: <letter>{<letter>} | <number> <number> <number>
//    <SPACE>    :: ' '{' '}
//    <params>   :: <SPACE> [':' <trailing> | <middle> <params>]
//    <middle>   :: <Any *non-empty* sequence of octets not including SPACE or NUL
//                   or CR or LF, the first of which may not be ':'>
//    <trailing> :: <Any, possibly empty, sequence of octets not including NUL or
//                   CR or LF>
//    <crlf>     :: CR LF
type Event struct {
	*Source                // The source of the event
	Command       string   // the IRC command, e.g. JOIN, PRIVMSG, KILL
	Params        []string // parameters to the command. Commonly nickname, channel, etc
	Trailing      string   // any trailing data. e.g. with a PRIVMSG, this is the message text
	EmptyTrailing bool     // if true, trailing prefix (:) will be added even if Event.Trailing is empty
	Sensitive     bool     // if the message is sensitive (e.g. and should not be logged)
}

// ParseEvent takes a string and attempts to create a Event struct.
// Returns nil if the Event is invalid.
func ParseEvent(raw string) (e *Event) {
	// ignore empty events
	if raw = strings.TrimFunc(raw, cutCRFunc); len(raw) < 2 {
		return nil
	}

	i, j := 0, 0
	e = new(Event)

	if raw[0] == prefix {
		// prefix ends with a space
		i = strings.IndexByte(raw, space)

		// prefix string must not be empty if the indicator is present
		if i < 2 {
			return nil
		}

		e.Source = ParseSource(raw[1:i])

		i++ // skip space at the end of the prefix
	}

	// find end of command
	j = i + strings.IndexByte(raw[i:], space)

	// extract command
	if j < i {
		e.Command = strings.ToUpper(raw[i:])
		return e
	}

	e.Command = strings.ToUpper(raw[i:j])
	j++ // skip space after command

	// find prefix for trailer
	i = strings.IndexByte(raw[j:], prefix)

	if i < 0 || raw[j+i-1] != space {
		// no trailing argument
		e.Params = strings.Split(raw[j:], string(space))
		return e
	}

	// compensate for index on substring
	i = i + j

	// check if we need to parse arguments
	if i > j {
		e.Params = strings.Split(raw[j:i-1], string(space))
	}

	e.Trailing = raw[i+1:]

	// we need to re-encode the trailing argument even if it was empty
	if len(e.Trailing) <= 0 {
		e.EmptyTrailing = true
	}

	return e

}

// Len calculates the length of the string representation of event
func (e *Event) Len() (length int) {
	if e.Source != nil {
		length = e.Source.Len() + 2 // include prefix and trailing space
	}

	length = length + len(e.Command)

	if len(e.Params) > 0 {
		length = length + len(e.Params)

		for i := 0; i < len(e.Params); i++ {
			length = length + len(e.Params[i])
		}
	}

	if len(e.Trailing) > 0 || e.EmptyTrailing {
		length = length + len(e.Trailing) + 2 // include prefix and space
	}

	return
}

// Bytes returns a []byte representation of event
//
// per RFC2812 section 2.3, messages should not exceed 512 characters
// in length. this method forces that limit by discarding any characters
// exceeding the length limit.
func (e *Event) Bytes() []byte {
	buffer := new(bytes.Buffer)

	// event prefix
	if e.Source != nil {
		buffer.WriteByte(prefix)
		e.Source.writeTo(buffer)
		buffer.WriteByte(space)
	}

	// command is required
	buffer.WriteString(e.Command)

	// space separated list of arguments
	if len(e.Params) > 0 {
		buffer.WriteByte(space)
		buffer.WriteString(strings.Join(e.Params, string(space)))
	}

	if len(e.Trailing) > 0 || e.EmptyTrailing {
		buffer.WriteByte(space)
		buffer.WriteByte(prefix)
		buffer.WriteString(e.Trailing)
	}

	// we need the limit the buffer length
	if buffer.Len() > (maxLength) {
		buffer.Truncate(maxLength)
	}

	return buffer.Bytes()
}

// Raw returns a string representation of this event.
func (e *Event) Raw() string {
	return string(e.Bytes())
}

// String returns a prettified string representation of this event.
//
// per RFC2812 section 2.3, messages should not exceed 512 characters
// in length. this method forces that limit by discarding any characters
// exceeding the length limit.
func (e *Event) String() (out string) {
	// event prefix
	if e.Source != nil {
		if e.Source.Name != "" {
			out += fmt.Sprintf("[%s] ", e.Source.Name)
		} else {
			out += fmt.Sprintf("[%s] ", e.Source)
		}
	}

	// command is required
	out += e.Command

	// space separated list of arguments
	if len(e.Params) > 0 {
		out += " " + strings.Join(e.Params, string(space))
	}

	if len(e.Trailing) > 0 || e.EmptyTrailing {
		out += " :" + e.Trailing
	}

	// we need the limit the buffer length
	if len(out) > (maxLength) {
		out = out[0 : maxLength-1]
	}

	return out
}

// IsAction checks to see if the event is a PRIVMSG, and is an ACTION (/me)
func (e *Event) IsAction() bool {
	if len(e.Trailing) <= 0 || e.Command != PRIVMSG {
		return false
	}

	if !strings.HasPrefix(e.Trailing, "\001ACTION") || !strings.HasSuffix(e.Trailing, "\001") {
		return false
	}

	return true
}

// StripAction strips the action encoding from a PRIVMSG ACTION (/me)
func (e *Event) StripAction() string {
	if !e.IsAction() || len(e.Trailing) < 9 {
		return e.Trailing
	}

	return e.Trailing[8 : len(e.Trailing)-1]
}
