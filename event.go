// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

const (
	eventSpace byte = 0x20 // Separator.
	maxLength       = 510  // Maximum length is 510 (2 for line endings).
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
	Source        *Source  // The source of the event.
	Tags          Tags     // IRCv3 style message tags. Only use if network supported.
	Command       string   // the IRC command, e.g. JOIN, PRIVMSG, KILL.
	Params        []string // parameters to the command. Commonly nickname, channel, etc.
	Trailing      string   // any trailing data. e.g. with a PRIVMSG, this is the message text.
	EmptyTrailing bool     // if true, trailing prefix (:) will be added even if Event.Trailing is empty.
	Sensitive     bool     // if the message is sensitive (e.g. and should not be logged).
}

// ParseEvent takes a string and attempts to create a Event struct.
//
// Returns nil if the Event is invalid.
func ParseEvent(raw string) (e *Event) {
	// Ignore empty events.
	if raw = strings.TrimFunc(raw, cutCRFunc); len(raw) < 2 {
		return nil
	}

	i, j := 0, 0
	e = &Event{}

	if raw[0] == prefixTag {
		// Tags end with a space.
		i = strings.IndexByte(raw, eventSpace)

		if i < 2 {
			return nil
		}

		e.Tags = ParseTags(raw[1:i])
		raw = raw[i+1:]
	}

	if raw[0] == messagePrefix {
		// Prefix ends with a space.
		i = strings.IndexByte(raw, eventSpace)

		// Prefix string must not be empty if the indicator is present.
		if i < 2 {
			return nil
		}

		e.Source = ParseSource(raw[1:i])

		// Skip space at the end of the prefix.
		i++
	}

	// Find end of command.
	j = i + strings.IndexByte(raw[i:], eventSpace)

	// Extract command.
	if j < i {
		e.Command = strings.ToUpper(raw[i:])
		return e
	}

	e.Command = strings.ToUpper(raw[i:j])
	// Skip space after command.
	j++

	// Find prefix for trailer.
	i = bytes.Index([]byte(raw[j:]), []byte{eventSpace, messagePrefix})
	if i != -1 {
		i += 1
	}

	if i < 0 || raw[j+i-1] != eventSpace {
		// No trailing argument.
		e.Params = strings.Split(raw[j:], string(eventSpace))
		return e
	}

	// Compensate for index on substring.
	i = i + j

	// Check if we need to parse arguments.
	if i > j {
		e.Params = strings.Split(raw[j:i-1], string(eventSpace))
	}

	e.Trailing = raw[i+1:]

	// We need to re-encode the trailing argument even if it was empty.
	if len(e.Trailing) <= 0 {
		e.EmptyTrailing = true
	}

	return e
}

// Len calculates the length of the string representation of event.
func (e *Event) Len() (length int) {
	if e.Tags != nil {
		// Include tags and trailing space.
		length = e.Tags.Len() + 1
	}
	if e.Source != nil {
		// Include prefix and trailing space.
		length += e.Source.Len() + 2
	}

	length += len(e.Command)

	if len(e.Params) > 0 {
		length += len(e.Params)

		for i := 0; i < len(e.Params); i++ {
			length += len(e.Params[i])
		}
	}

	if len(e.Trailing) > 0 || e.EmptyTrailing {
		// Include prefix and space.
		length += len(e.Trailing) + 2
	}

	return
}

// Bytes returns a []byte representation of event. Strips all newlines and
// carriage returns.
//
// Per RFC2812 section 2.3, messages should not exceed 512 characters in
// length. This method forces that limit by discarding any characters
// exceeding the length limit.
func (e *Event) Bytes() []byte {
	buffer := new(bytes.Buffer)

	// Tags.
	if e.Tags != nil {
		e.Tags.writeTo(buffer)
	}

	// Event prefix.
	if e.Source != nil {
		buffer.WriteByte(messagePrefix)
		e.Source.writeTo(buffer)
		buffer.WriteByte(eventSpace)
	}

	// Command is required.
	buffer.WriteString(e.Command)

	// Space separated list of arguments.
	if len(e.Params) > 0 {
		buffer.WriteByte(eventSpace)
		buffer.WriteString(strings.Join(e.Params, string(eventSpace)))
	}

	if len(e.Trailing) > 0 || e.EmptyTrailing {
		buffer.WriteByte(eventSpace)
		buffer.WriteByte(messagePrefix)
		buffer.WriteString(e.Trailing)
	}

	// We need the limit the buffer length.
	if buffer.Len() > (maxLength) {
		if e.Tags != nil {
			// regular message, max tag length, and the splitting space.
			buffer.Truncate(maxLength + maxTagLength + 1)
		} else {
			buffer.Truncate(maxLength)
		}
	}

	out := buffer.Bytes()

	// Strip newlines and carriage returns.
	for i := 0; i < len(out); i++ {
		if out[i] == 0x0A || out[i] == 0x0D {
			out = append(out[:i], out[i+1:]...)
			i-- // Decrease the index so we can pick up where we left off.
		}
	}

	return out
}

// String returns a string representation of this event. Strips all newlines
// and carriage returns.
func (e *Event) String() string {
	return string(e.Bytes())
}

// Pretty returns a prettified string of the event. If the event doesn't
// support prettification, ok is false. Pretty is not just useful to make
// an event prettier, but also to filter out events that most don't visually
// see in normal IRC clients. e.g. most clients don't show WHO queries.
func (e *Event) Pretty() (out string, ok bool) {
	if e.Command == INITIALIZED {
		return fmt.Sprintf("[*] connection to %s initialized", e.Trailing), true
	}

	if e.Command == CONNECTED {
		return fmt.Sprintf("[*] successfully connected to %s", e.Trailing), true
	}

	if (e.Command == PRIVMSG || e.Command == NOTICE) && len(e.Params) > 0 {
		return fmt.Sprintf("[%s] (%s) %s", strings.Join(e.Params, ","), e.Source.Name, e.Trailing), true
	}

	if e.Command == RPL_MOTD || e.Command == RPL_MOTDSTART ||
		e.Command == RPL_WELCOME || e.Command == RPL_YOURHOST ||
		e.Command == RPL_CREATED || e.Command == RPL_LUSERCLIENT {
		return fmt.Sprintf("[*] " + e.Trailing), true
	}

	if e.Command == JOIN {
		return fmt.Sprintf("[*] %s has joined %s", e.Source.Name, strings.Join(e.Params, ", ")), true
	}

	if e.Command == PART {
		return fmt.Sprintf("[*] %s has left %s (%s)", e.Source.Name, strings.Join(e.Params, ", "), e.Trailing), true
	}

	if e.Command == ERROR {
		return fmt.Sprintf("[*] an error occurred: %s", e.Trailing), true
	}

	if e.Command == QUIT {
		return fmt.Sprintf("[*] %s has quit (%s)", e.Source.Name, e.Trailing), true
	}

	if e.Command == KICK && len(e.Params) == 2 {
		return fmt.Sprintf("[%s] *** %s has kicked %s: %s", e.Params[0], e.Source.Name, e.Params[1], e.Trailing), true
	}

	if e.Command == NICK && len(e.Params) == 1 {
		return fmt.Sprintf("[*] %s is now known as %s", e.Source.Name, e.Params[0]), true
	}

	if e.Command == TOPIC && len(e.Params) > 0 {
		return fmt.Sprintf("[%s] *** %s has set the topic to: %s", e.Params[len(e.Params)-1], e.Source.Name, e.Trailing), true
	}

	if e.Command == MODE && len(e.Params) > 2 {
		return fmt.Sprintf("[%s] %s set modes: %s", e.Params[0], e.Source.Name, strings.Join(e.Params[1:], " ")), true
	}

	return "", false
}

// IsAction checks to see if the event is a PRIVMSG, and is an ACTION (/me).
func (e *Event) IsAction() bool {
	if len(e.Trailing) <= 0 || e.Command != PRIVMSG {
		return false
	}

	if !strings.HasPrefix(e.Trailing, "\001ACTION") || e.Trailing[len(e.Trailing)-1] != ctcpDelim {
		return false
	}

	return true
}

// IsFromChannel checks to see if a message was from a channel (rather than
// a private message).
func (e *Event) IsFromChannel() bool {
	if len(e.Params) != 1 {
		return false
	}

	if e.Command != "PRIVMSG" || !IsValidChannel(e.Params[0]) {
		return false
	}

	return true
}

// IsFromUser checks to see if a message was from a user (rather than a
// channel).
func (e *Event) IsFromUser() bool {
	if len(e.Params) != 1 {
		return false
	}

	if e.Command != "PRIVMSG" || !IsValidNick(e.Params[0]) {
		return false
	}

	return true
}

// StripAction returns the stripped version of the action encoding from a
// PRIVMSG ACTION (/me).
func (e *Event) StripAction() string {
	if !e.IsAction() || len(e.Trailing) < 9 {
		return e.Trailing
	}

	return e.Trailing[8 : len(e.Trailing)-1]
}

// EventLimiter is a custom ticker which lets you rate limit sending events
// to a function (e.g. Client.Send()), with optional burst support. See
// NewEventLimiter() for more information.
type EventLimiter struct {
	tick     *time.Ticker
	throttle chan time.Time
	fn       func(*Event) error
}

// loop is used to read events from the internal time.Ticker.
func (el *EventLimiter) loop() {
	// This should exit itself once el.Stop() is called.
	for t := range el.tick.C {
		el.throttle <- t
	}
}

// Stop closes the ticker, and prevents re-use of the EventLimiter. Use this
// to prevent EventLimiter from keeping unnecessary pointers in memory.
func (el *EventLimiter) Stop() {
	el.tick.Stop()
	el.fn = nil
}

// Send is the subtitute function used to send the event the the previously
// specified send function.
//
// This WILL panic if Stop() was already called on the EventLimiter.
func (el *EventLimiter) Send(event *Event) error {
	// Ensure nobody is sending to it once it's closed.
	if el.fn == nil {
		panic("attempted send on closed EventLimiter")
	}

	<-el.throttle
	return el.fn(event)
}

// SendAll sends a list of events to Send(). SendAll will return the first
// error it gets when attempting to Send() to the predefined Send function.
// It will not attempt to continue processing the list of events.
func (el *EventLimiter) SendAll(events ...*Event) error {
	for i := 0; i < len(events); i++ {
		if err := el.Send(events[i]); err != nil {
			return err
		}
	}

	return nil
}

// NewEventLimiter returns a NewEventLimiter which can be used to rate limit
// events being sent to a Send function. This does support bursting a
// certain amount of messages if there are less than burstCount.
//
// Ensure that Stop() is called on the returned EventLimiter, otherwise
// the limiter may keep unwanted pointers to data in memory.
func NewEventLimiter(burstCount int, rate time.Duration, eventFunc func(event *Event) error) *EventLimiter {
	limiter := &EventLimiter{
		tick:     time.NewTicker(rate),
		throttle: make(chan time.Time, burstCount),
		fn:       eventFunc,
	}

	// Push the ticket into the background. If you want to stop this, simply
	// use EventLimiter.Stop().
	go limiter.loop()

	return limiter
}
