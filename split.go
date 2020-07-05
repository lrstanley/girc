package girc

import (
	"bytes"
	"unicode/utf8"
)

// splitFunc is the type used for functions implementing splitting of
// too long IRC messages. The function is passed an event which must be
// split into multiple and an event length which shall not be exceeded.
type splitFunc func(event *Event, maxLen int) []*Event

var splitFuncs = map[string]splitFunc{
	PRIVMSG: splitPRIVMSG,
}

func splitPRIVMSG(event *Event, maxLen int) (events []*Event) {
	newMsg := func(text []byte) *Event {
		e := event.Copy()
		e.Params[len(event.Params)-1] = string(text)
		return e
	}

	// Event used to calculate base length of command, this does not
	// include the event source and the last parameter.
	rawEvent := event.Copy()
	rawEvent.Params = event.Params[0 : len(event.Params)-1]

	// maxTextLen must not be exceeded by the last parameter of the
	// PRIVMSG. Also include " :" for formatting this last parameter.
	maxTextLen := maxLen - rawEvent.Len() - len(" :")
	if maxTextLen <= 0 {
		return []*Event{event} // TODO: too long without any text
	}

	b := []byte(event.Last())
	for len(b) > maxTextLen {
		idx := bytes.LastIndexByte(b[:maxTextLen], byte(' '))
		if idx > 0 {
			// maxTextLen is inclusive → include seperator
			idx++
		} else {
			// maxTextLen is inclusive → include extra byte
			idx = bytes.LastIndexFunc(b[:maxTextLen+1], utf8.ValidRune)
		}

		events = append(events, newMsg(b[:idx]))
		b = b[idx:]
	}
	events = append(events, newMsg(b))

	return events
}

// getMaxLen returns the maximum possible length of an IRC message.
// Messages exceeding this length (without taking the prefix length into
// account) must be split into multiple separate messages.
func (c *Client) getMaxLen() int {
	// From RFC 2812:
	//   IRC messages are always lines of characters terminated with a CR-LF
	//   (Carriage Return - Line Feed) pair, and these messages SHALL NOT
	//   exceed 512 characters in length, counting all characters including
	//   the trailing CR-LF.
	const maxIRClen int = 512 - len("\r\n")

	// maxPrefixLen is the maximum possible length of a server
	// message prefix as defined by the following ABNF in RFC 2812:
	//
	//   [ ":" ( servername / ( nickname [ [ "!" user ] "@" host ] ) ) SPACE ]
	var maxPrefixLen int

	// Default values taken from https://modern.ircdocs.horse/
	// Most of these are not actually standardized.
	nicklen := c.getServerOptionInt("NICKLEN", 10)
	userlen := c.getServerOptionInt("USERLEN", 18)
	hostlen := c.getServerOptionInt("HOSTLEN", 63)

	// The code here assumes that `servername` is never used in a
	// prefix as this function is only concerned with messages send
	// by ones own client. In accordance with the ABNF from above,
	// the maximum length is therefore calculated as:
	//
	//   ":" <nickname> "!" <user> "@" <host> " "
	//
	maxPrefixLen = 1 + nicklen + 1 + userlen + 1 + hostlen + 1
	if maxPrefixLen >= maxIRClen {
		panic("maximum prefix length exceeds maximum IRC message length")
	}

	return maxIRClen - maxPrefixLen
}

// splitEvent splits a given event into multiple events to satisfy the
// maximum message length requirements imposed upon the given client by
// the associated IRC server.
func (c *Client) splitEvent(event *Event) []*Event {
	// We cannot calculate the length of the source part actually
	// used by the server. Assume the largest possible value and
	// make sure source is unset for the event to ensure it is not
	// take into account by `event.Len()`.
	event.Source = nil // XXX: Operate on a copy instead?

	if event.Len() > c.maxMsgLen {
		fn, ok := splitFuncs[event.Command]
		if ok {
			return fn(event, c.maxMsgLen)
		}
	}

	return []*Event{event}
}
