package girc

import (
	"bytes"
	"strings"
)

const (
	prefix     byte = 0x3A // prefix or last argument
	prefixUser byte = 0x21 // username
	prefixHost byte = 0x40 // hostname
	space      byte = 0x20 // separator

	maxLength = 510 // maximum length is 510 (2 for line endings)
)

func cutsetFunc(r rune) bool {
	// Characters to trim from prefixes/messages.
	return r == '\r' || r == '\n'
}

// Prefix represents the sender of an IRC event, see RFC1459 section 2.3.1
// <servername> | <nick> [ '!' <user> ] [ '@' <host> ]
type Prefix struct {
	Name string // Nick or servername
	User string // Username
	Host string // Hostname
}

// ParsePrefix takes a string and attempts to create a Prefix struct.
func ParsePrefix(raw string) (p *Prefix) {
	p = new(Prefix)

	user := indexByte(raw, prefixUser)
	host := indexByte(raw, prefixHost)

	switch {
	case user > 0 && host > user:
		p.Name = raw[:user]
		p.User = raw[user+1 : host]
		p.Host = raw[host+1:]
	case user > 0:
		p.Name = raw[:user]
		p.User = raw[user+1:]
	case host > 0:
		p.Name = raw[:host]
		p.Host = raw[host+1:]
	default:
		p.Name = raw

	}

	return p
}

// Len calculates the length of the string representation of prefix
func (p *Prefix) Len() (length int) {
	length = len(p.Name)
	if len(p.User) > 0 {
		length = 1 + length + len(p.User)
	}
	if len(p.Host) > 0 {
		length = 1 + length + len(p.Host)
	}

	return
}

// Bytes returns a []byte representation of prefix
func (p *Prefix) Bytes() []byte {
	buffer := new(bytes.Buffer)
	p.writeTo(buffer)

	return buffer.Bytes()
}

// String returns a string representation of prefix
func (p *Prefix) String() (s string) {
	s = p.Name
	if len(p.User) > 0 {
		s = s + string(prefixUser) + p.User
	}
	if len(p.Host) > 0 {
		s = s + string(prefixHost) + p.Host
	}

	return
}

// IsHostmask returns true if prefix looks like a user hostmask
func (p *Prefix) IsHostmask() bool {
	return len(p.User) > 0 && len(p.Host) > 0
}

// IsServer returns true if this prefix looks like a server name.
func (p *Prefix) IsServer() bool {
	return len(p.User) <= 0 && len(p.Host) <= 0 // && indexByte(p.Name, '.') > 0
}

// writeTo is an utility function to write the prefix to the bytes.Buffer in Event.String()
func (p *Prefix) writeTo(buffer *bytes.Buffer) {
	buffer.WriteString(p.Name)
	if len(p.User) > 0 {
		buffer.WriteByte(prefixUser)
		buffer.WriteString(p.User)
	}
	if len(p.Host) > 0 {
		buffer.WriteByte(prefixHost)
		buffer.WriteString(p.Host)
	}

	return
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
	*Prefix
	Command  string
	Params   []string
	Trailing string

	// When set to true, the trailing prefix (:) will be added even if the trailing message is empty.
	EmptyTrailing bool

	Sensitive bool // if the message is sensitive (e.g. and should not be logged)
}

// ParseEvent takes a string and attempts to create a Event struct.
// Returns nil if the Event is invalid.
func ParseEvent(raw string) (e *Event) {
	// ignore empty events
	if raw = strings.TrimFunc(raw, cutsetFunc); len(raw) < 2 {
		return nil
	}

	i, j := 0, 0

	e = new(Event)

	if raw[0] == prefix {
		// prefix ends with a space
		i = indexByte(raw, space)

		// prefix string must not be empty if the indicator is present
		if i < 2 {
			return nil
		}

		e.Prefix = ParsePrefix(raw[1:i])

		// skip space at the end of the prefix
		i++
	}

	// find end of command
	j = i + indexByte(raw[i:], space)

	// extract command
	if j > i {
		e.Command = strings.ToUpper(raw[i:j])
	} else {
		e.Command = strings.ToUpper(raw[i:])

		return e
	}

	// skip space after command
	j++

	// find prefix for trailer
	i = indexByte(raw[j:], prefix)

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

// Len calculates the length of the string representation of this event
func (e *Event) Len() (length int) {
	if e.Prefix != nil {
		length = e.Prefix.Len() + 2 // include prefix and trailing space
	}

	length = length + len(e.Command)

	if len(e.Params) > 0 {
		length = length + len(e.Params)

		for _, param := range e.Params {
			length = length + len(param)
		}
	}

	if len(e.Trailing) > 0 || e.EmptyTrailing {
		length = length + len(e.Trailing) + 2 // include prefix and space
	}

	return
}

// Bytes returns a []byte representation of this event
//
// as noted in RFC2812 section 2.3, messages should not exceed 512 characters
// in length. this method forces that limit by discarding any characters
// exceeding the length limit.
func (e *Event) Bytes() []byte {
	buffer := new(bytes.Buffer)

	// event prefix
	if e.Prefix != nil {
		buffer.WriteByte(prefix)
		e.Prefix.writeTo(buffer)
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

// String returns a string representation of this event
func (e *Event) String() string {
	return string(e.Bytes())
}

func indexByte(s string, c byte) int {
	return strings.IndexByte(s, c)
}

// contains '*', even though this isn't RFC compliant, it's commonly used
var validChannelPrefixes = [...]string{"&", "#", "+", "!", "*"}

// IsValidChannel checks if channel is an RFC complaint channel or not
func IsValidChannel(channel string) bool {
	if len(channel) < 1 || len(channel) > 50 {
		return false
	}

	var validprefix bool
	for i := 0; i < len(validChannelPrefixes); i++ {
		if string(channel[0]) == validChannelPrefixes[i] {
			validprefix = true
			break
		}
	}

	if !validprefix {
		return false
	}

	if strings.Contains(channel, " ") || strings.Contains(channel, ",") {
		return false
	}

	return true
}
