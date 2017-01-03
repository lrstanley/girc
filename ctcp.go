// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"runtime"
	"strings"
	"sync"
	"time"
)

// ctcpDelim if the delimiter used for CTCP formatted events/messages.
const ctcpDelim byte = 0x01 // Prefix and suffix for CTCP messages.

// CTCPEvent is the necessary information from an IRC message.
type CTCPEvent struct {
	// Source is the author of the CTCP event.
	Source *Source
	// Command is the type of CTCP event. E.g. PING, TIME, VERSION.
	Command string
	// Text is the raw arguments following the command.
	Text string
	// Reply is true if the CTCP event is intended to be a reply to a
	// previous CTCP (e.g, if we sent one).
	Reply bool
}

// decodeCTCP decodes an incoming CTCP event, if it is CTCP. nil is returned
// if the incoming event does not match a valid CTCP.
func decodeCTCP(e *Event) *CTCPEvent {
	// http://www.irchelp.org/protocol/ctcpspec.html

	// Must be targeting a user/channel, AND trailing must have
	// DELIM+TAG+DELIM minimum (at least 3 chars).
	if len(e.Params) != 1 || len(e.Trailing) < 3 {
		return nil
	}

	if (e.Command != "PRIVMSG" && e.Command != "NOTICE") || !IsValidNick(e.Params[0]) {
		return nil
	}

	if e.Trailing[0] != ctcpDelim || e.Trailing[len(e.Trailing)-1] != ctcpDelim {
		return nil
	}

	// Strip delimiters.
	text := e.Trailing[1 : len(e.Trailing)-1]

	s := strings.IndexByte(text, eventSpace)

	// Check to see if it only contains a tag.
	if s < 0 {
		for i := 0; i < len(text); i++ {
			// Check for A-Z, 0-9.
			if (text[i] < 0x41 || text[i] > 0x5A) && (text[i] < 0x30 || text[i] > 0x39) {
				return nil
			}
		}

		return &CTCPEvent{
			Source:  e.Source,
			Command: text,
			Reply:   e.Command == "NOTICE",
		}
	}

	// Loop through checking the tag first.
	for i := 0; i < s; i++ {
		// Check for A-Z, 0-9.
		if (text[i] < 0x41 || text[i] > 0x5A) && (text[i] < 0x30 || text[i] > 0x39) {
			return nil
		}
	}

	return &CTCPEvent{
		Source:  e.Source,
		Command: text[0:s],
		Text:    text[s+1:],
		Reply:   e.Command == "NOTICE",
	}
}

// encodeCTCP encodes a CTCP event into a string, including delimiters.
func encodeCTCP(ctcp *CTCPEvent) (out string) {
	if ctcp == nil {
		return ""
	}

	return encodeCTCPRaw(ctcp.Command, ctcp.Text)
}

// encodeCTCPRaw is much like encodeCTCP, however accepts a raw command and
// string as input.
func encodeCTCPRaw(cmd, text string) (out string) {
	if len(cmd) <= 0 {
		return ""
	}

	out = string(ctcpDelim) + cmd

	if len(text) > 0 {
		out += string(eventSpace) + text
	}

	return out + string(ctcpDelim)
}

// CTCP handles the storage and execution of CTCP handlers against incoming
// CTCP events.
type CTCP struct {
	disableDefault bool
	// mu is the mutex that should be used when accessing callbacks.
	mu sync.RWMutex
	// handlers is a map of CTCP message -> functions.
	handlers map[string]CTCPHandler
}

// newCTCP returns a new clean CTCP handler.
func newCTCP() *CTCP {
	return &CTCP{handlers: map[string]CTCPHandler{}}
}

// call executes the necessary CTCP handler for the incoming event/CTCP
// command.
func (c *CTCP) call(event *CTCPEvent, client *Client) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Support wildcard CTCP event handling. Gets executed first before
	// regular event handlers.
	if _, ok := c.handlers["*"]; ok {
		c.handlers["*"](client, *event)
	}

	if _, ok := c.handlers[event.Command]; !ok {
		// Send a ERRMSG reply, if we know who sent it.
		if event.Source != nil && IsValidNick(event.Source.Name) {
			client.SendCTCPReply(event.Source.Name, CTCP_ERRMSG, "that is an unknown CTCP query")
		}
		return
	}

	c.handlers[event.Command](client, *event)
}

// parseCMD parses a CTCP command/tag, ensuring it's valid. If not, an empty
// string is returned.
func (c *CTCP) parseCMD(cmd string) string {
	// TODO: Needs proper testing.
	// Check if wildcard.
	if cmd == "*" {
		return "*"
	}

	cmd = strings.ToUpper(cmd)

	for i := 0; i < len(cmd); i++ {
		// Check for A-Z, 0-9.
		if (cmd[i] < 0x41 || cmd[i] > 0x5A) && (cmd[i] < 0x30 || cmd[i] > 0x39) {
			return ""
		}
	}

	return cmd
}

// Set saves handler for execution upon a matching incoming CTCP event.
// Use SetBg if the handler may take an extended period of time to execute.
// If you would like to have a handler which will catch ALL CTCP requests,
// simply use "*" in place of the command.
func (c *CTCP) Set(cmd string, handler func(client *Client, ctcp CTCPEvent)) {
	if cmd = c.parseCMD(cmd); cmd == "" {
		return
	}

	c.mu.Lock()
	c.handlers[cmd] = CTCPHandler(handler)
	c.mu.Unlock()
}

// SetBg is much like Set, however the handler is executed in the background,
// ensuring that event handling isn't hung during long running tasks. See Set
// for more information.
func (c *CTCP) SetBg(cmd string, handler func(client *Client, ctcp CTCPEvent)) {
	c.Set(cmd, func(client *Client, ctcp CTCPEvent) {
		go handler(client, ctcp)
	})
}

// Clear removes currently setup handler for cmd, if one is set. This will
// also disable default handlers for a specific cmd.
func (c *CTCP) Clear(cmd string) {
	if cmd = c.parseCMD(cmd); cmd == "" {
		return
	}

	c.mu.Lock()
	delete(c.handlers, cmd)
	c.mu.Unlock()
}

// ClearAll removes all currently setup and re-sets the default handlers,
// unless configured not to. See Client.Config.DisableDefaultCTCP.
func (c *CTCP) ClearAll() {
	c.mu.Lock()
	c.handlers = map[string]CTCPHandler{}
	c.mu.Unlock()

	// Register necessary handlers.
	c.addDefaultHandlers()
}

// CTCPHandler is a type that represents the function necessary to
// implement a CTCP handler.
type CTCPHandler func(client *Client, ctcp CTCPEvent)

// addDefaultHandlers adds some useful default CTCP response handlers, unless
// requested by the client not to.
func (c *CTCP) addDefaultHandlers() {
	if c.disableDefault {
		return
	}

	c.SetBg(CTCP_PING, handleCTCPPing)
	c.SetBg(CTCP_PONG, handleCTCPPong)
	c.SetBg(CTCP_VERSION, handleCTCPVersion)
	c.SetBg(CTCP_SOURCE, handleCTCPSource)
	c.SetBg(CTCP_TIME, handleCTCPTime)
}

// handleCTCPPing replies with a ping and whatever was originally requested.
func handleCTCPPing(client *Client, ctcp CTCPEvent) {
	if ctcp.Reply {
		return
	}
	client.SendCTCPReply(ctcp.Source.Name, CTCP_PING, ctcp.Text)
}

// handleCTCPPong replies with a pong.
func handleCTCPPong(client *Client, ctcp CTCPEvent) {
	if ctcp.Reply {
		return
	}
	client.SendCTCPReply(ctcp.Source.Name, CTCP_PONG, "")
}

// handleCTCPVersion replies with the name of the client, Go version, as well
// as the os type (darwin, linux, windows, etc) and architecture type (x86,
// arm, etc).
func handleCTCPVersion(client *Client, ctcp CTCPEvent) {
	client.SendCTCPReplyf(
		ctcp.Source.Name, CTCP_VERSION,
		"girc (github.com/lrstanley/girc) using %s (%s, %s)",
		runtime.Version(), runtime.GOOS, runtime.GOARCH,
	)
}

// handleCTCPSource replies with the public git location of this library.
func handleCTCPSource(client *Client, ctcp CTCPEvent) {
	client.SendCTCPReply(ctcp.Source.Name, CTCP_SOURCE, "https://github.com/lrstanley/girc")
}

// handleCTCPTime replies with a RFC 1123 (Z) formatted version of Go's
// local time.
func handleCTCPTime(client *Client, ctcp CTCPEvent) {
	client.SendCTCPReply(ctcp.Source.Name, CTCP_TIME, ":"+time.Now().Format(time.RFC1123Z))
}
