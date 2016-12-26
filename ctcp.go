// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"strings"
	"sync"
)

const ctcpDelim byte = 0x01 // Prefix and suffix for CTCP messages.

type CTCPEvent struct {
	Source  *Source
	Command string
	Text    string
}

func decodeCTCP(e *Event) *CTCPEvent {
	// Must be targetting a user/channel, AND trailing must have
	// DELIM+TAG+DELIM minimum (at least 3 chars).
	if len(e.Params) != 1 || len(e.Trailing) < 3 {
		return nil
	}

	if e.Command != "PRIVMSG" || !IsValidNick(e.Params[0]) {
		return nil
	}

	if e.Trailing[0] != ctcpDelim || e.Trailing[len(e.Trailing)-1] != ctcpDelim {
		return nil
	}

	// Strip delimiters.
	text := e.Trailing[1 : len(e.Trailing)-1]

	s := strings.IndexByte(text, space)

	// Check to see if it only contains a tag.
	if s < 0 {
		for i := 0; i < len(text); i++ {
			// Check for A-Z, 0-9.
			if (text[i] < 0x41 || text[i] > 0x5A) && (text[i] < 0x30 || text[i] > 0x39) {
				return nil
			}
		}

		return &CTCPEvent{Source: e.Source, Command: text}
	}

	// Loop through checking the tag first.
	for i := 0; i < s; i++ {
		// Check for A-Z, 0-9.
		if (text[i] < 0x41 || text[i] > 0x5A) && (text[i] < 0x30 || text[i] > 0x39) {
			return nil
		}
	}

	return &CTCPEvent{Source: e.Source, Command: text[0:s], Text: text[s+1 : len(text)]}
}

func encodeCTCP(ctcp *CTCPEvent) (out string) {
	if ctcp == nil {
		return ""
	}

	return encodeCTCPRaw(ctcp.Command, ctcp.Text)
}

func encodeCTCPRaw(cmd, text string) (out string) {
	if len(cmd) <= 0 {
		return ""
	}

	out = string(ctcpDelim) + cmd

	if len(text) > 0 {
		out += string(space) + text
	}

	return out + string(ctcpDelim)
}

type CTCP struct {
	// mu is the mutex that should be used when accessing callbacks.
	mu sync.RWMutex
	// handlers is a map of CTCP message -> functions.
	handlers map[string]CTCPHandler
}

func newCTCP() *CTCP {
	return &CTCP{handlers: map[string]CTCPHandler{}}
}

func (c *CTCP) call(event *CTCPEvent, client *Client) {
	c.mu.RLock()
	if _, ok := c.handlers[event.Command]; !ok {
		c.mu.RUnlock()
		return
	}

	go c.handlers[event.Command](client, event)
	c.mu.RUnlock()
}

func (c *CTCP) parseCMD(cmd string) string {
	cmd = strings.ToUpper(cmd)

	for i := 0; i < len(cmd); i++ {
		// Check for A-Z, 0-9.
		if (cmd[i] < 0x41 || cmd[i] > 0x5A) && (cmd[i] < 0x30 || cmd[i] > 0x39) {
			return ""
		}
	}

	return cmd
}

func (c *CTCP) Set(cmd string, handler func(client *Client, ctcp *CTCPEvent)) {
	if cmd = c.parseCMD(cmd); cmd == "" {
		return
	}

	c.mu.Lock()
	c.handlers[cmd] = CTCPHandler(handler)
	c.mu.Unlock()
}

func (c *CTCP) SetBg(cmd string, handler func(client *Client, ctcp *CTCPEvent)) {
	c.Set(cmd, func(client *Client, ctcp *CTCPEvent) {
		go handler(client, ctcp)
	})
}

func (c *CTCP) Clear(cmd string) {
	if cmd = c.parseCMD(cmd); cmd == "" {
		return
	}

	c.mu.Lock()
	delete(c.handlers, cmd)
	c.mu.Unlock()
}

func (c *CTCP) ClearAll() {
	c.mu.Lock()
	c.handlers = map[string]CTCPHandler{}
	c.mu.Unlock()
}

// CTCPHandler is a type that represents the function necessary to
// implement a CTCP handler.
type CTCPHandler func(client *Client, ctcp *CTCPEvent)

func (c *CTCP) addDefaultHandlers() {
	c.SetBg(CTCP_PING, handleCTCPPing)
}

func handleCTCPPing(client *Client, ctcp *CTCPEvent) {
	client.SendCTCP(ctcp.Source.Name, CTCP_PING, "")
}
