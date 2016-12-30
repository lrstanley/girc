// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import "time"

// registerHelpers sets up built-in callbacks/helpers, based on client
// configuration.
func (c *Client) registerHelpers() {
	c.Callbacks.mu.Lock()

	// Built-in things that should always be supported.
	c.Callbacks.register(true, SUCCESS, CallbackFunc(func(c *Client, e Event) {
		go handleConnect(c, e)
	}))
	c.Callbacks.register(true, PING, CallbackFunc(handlePING))

	if !c.Config.DisableTracking {
		// Joins/parts/anything that may add/remove/rename users.
		c.Callbacks.register(true, JOIN, CallbackFunc(handleJOIN))
		c.Callbacks.register(true, PART, CallbackFunc(handlePART))
		c.Callbacks.register(true, KICK, CallbackFunc(handleKICK))
		c.Callbacks.register(true, QUIT, CallbackFunc(handleQUIT))
		c.Callbacks.register(true, NICK, CallbackFunc(handleNICK))

		// WHO/WHOX responses.
		c.Callbacks.register(true, RPL_WHOREPLY, CallbackFunc(handleWHO))
		c.Callbacks.register(true, RPL_WHOSPCRPL, CallbackFunc(handleWHO))

		// Other misc. useful stuff.
		c.Callbacks.register(true, TOPIC, CallbackFunc(handleTOPIC))
		c.Callbacks.register(true, RPL_TOPIC, CallbackFunc(handleTOPIC))
	}

	// Nickname collisions.
	if !c.Config.DisableNickCollision {
		c.Callbacks.register(true, ERR_NICKNAMEINUSE, CallbackFunc(nickCollisionHandler))
		c.Callbacks.register(true, ERR_NICKCOLLISION, CallbackFunc(nickCollisionHandler))
		c.Callbacks.register(true, ERR_UNAVAILRESOURCE, CallbackFunc(nickCollisionHandler))
	}

	// CAP IRCv3-specific tracking and functionality.
	if !c.Config.DisableTracking && !c.Config.DisableCapTracking {
		c.Callbacks.register(true, CAP_CHGHOST, CallbackFunc(handleCHGHOST))
		c.Callbacks.register(true, CAP_AWAY, CallbackFunc(handleAWAY))
	}

	c.Callbacks.mu.Unlock()
}

// handleConnect is a helper function which lets the client know that enough
// time has passed and now they can send commands.
//
// Should always run in separate thread due to blocking delay.
func handleConnect(c *Client, e Event) {
	// This should be the nick that the server gives us. 99% of the time, it's
	// the one we supplied during connection, but some networks will rename
	// users on connect.
	if len(e.Params) > 0 {
		c.state.nick = e.Params[0]
	}

	time.Sleep(1 * time.Second)

	c.Events <- &Event{Command: CONNECTED}
}

// nickCollisionHandler helps prevent the client from having conflicting
// nicknames with another bot, user, etc.
func nickCollisionHandler(c *Client, e Event) {
	c.Nick(c.GetNick() + "_")
}

// handlePING helps respond to ping requests from the server.
func handlePING(c *Client, e Event) {
	c.Send(&Event{Command: PONG, Params: e.Params, Trailing: e.Trailing})
}

// handleJOIN ensures that the state has updated users and channels.
func handleJOIN(c *Client, e Event) {
	if len(e.Params) < 1 {
		return
	}

	// Create the user in state. 2This will also verify the channel.
	c.state.mu.Lock()
	user := c.state.createUserIfNotExists(e.Params[0], e.Source.Name)
	c.state.mu.Unlock()
	if user == nil {
		return
	}

	if e.Source.Name == c.GetNick() {
		// If it's us, don't just add our user to the list. Run a WHO which
		// will tell us who exactly is in the entire channel.
		c.Send(&Event{Command: WHO, Params: []string{e.Params[0], "%tacuhnr,1"}})
		return
	}

	// Only WHO the user, which is more efficient.
	c.Send(&Event{Command: WHO, Params: []string{e.Source.Name, "%tacuhnr,1"}})
}

// handlePART ensures that the state is clean of old user and channel entries.
func handlePART(c *Client, e Event) {
	if len(e.Params) == 0 {
		return
	}

	if e.Source.Name == c.GetNick() {
		c.state.mu.Lock()
		c.state.deleteChannel(e.Params[0])
		c.state.mu.Unlock()
		return
	}

	c.state.mu.Lock()
	c.state.deleteUser(e.Source.Name)
	c.state.mu.Unlock()
}

func handleTOPIC(c *Client, e Event) {
	var name string
	switch len(e.Params) {
	case 0:
		return
	case 1:
		name = e.Params[0]
	default:
		name = e.Params[len(e.Params)-1]
	}

	c.state.mu.Lock()
	channel := c.state.createChanIfNotExists(name)
	if channel == nil {
		c.state.mu.Unlock()
		return
	}

	channel.Topic = e.Trailing
	c.state.mu.Unlock()
}

// handlWHO updates our internal tracking of users/channels with WHO/WHOX
// information.
func handleWHO(c *Client, e Event) {
	var channel, ident, host, nick, account string

	// Assume WHOX related.
	if e.Command == RPL_WHOSPCRPL {
		if len(e.Params) != 7 {
			// Assume there was some form of error or invalid WHOX response.
			return
		}

		if e.Params[1] != "1" {
			// We should always be sending 1, and we should receive 1. If this
			// is anything but, then we didn't send the request and we can
			// ignore it.
			return
		}

		channel, ident, host, nick, account = e.Params[2], e.Params[3], e.Params[4], e.Params[5], e.Params[6]
	} else {
		channel, ident, host, nick = e.Params[1], e.Params[2], e.Params[3], e.Params[5]
	}

	c.state.mu.Lock()
	user := c.state.createUserIfNotExists(channel, nick)
	if user == nil {
		c.state.mu.Unlock()
		return
	}

	user.Host = host
	user.Ident = ident
	user.Extras.Name = e.Trailing

	if account != "0" {
		user.Extras.Account = account
	}

	c.state.mu.Unlock()
}

// handleKICK ensures that users are cleaned up after being kicked from the
// channel
func handleKICK(c *Client, e Event) {
	if len(e.Params) < 2 {
		// Needs at least channel and user.
		return
	}

	if e.Params[1] == c.GetNick() {
		c.state.mu.Lock()
		c.state.deleteChannel(e.Params[0])
		c.state.mu.Unlock()
		return
	}

	// Assume it's just another user.
	c.state.mu.Lock()
	c.state.deleteUser(e.Params[1])
	c.state.mu.Unlock()
}

// handleNICK ensures that users are renamed in state, or the client name is
// up to date.
func handleNICK(c *Client, e Event) {
	if len(e.Params) != 1 {
		// Something erronous was sent to us.
		return
	}

	c.state.mu.Lock()
	c.state.renameUser(e.Source.Name, e.Params[0])
	c.state.mu.Unlock()
}

func handleQUIT(c *Client, e Event) {
	c.state.mu.Lock()
	c.state.deleteUser(e.Source.Name)
	c.state.mu.Unlock()
}

func handleCHGHOST(c *Client, e Event) {
	if len(e.Params) != 2 {
		return
	}

	c.state.mu.Lock()
	for chanName := range c.state.channels {
		if _, ok := c.state.channels[chanName].users[e.Source.Name]; !ok {
			continue
		}

		c.state.channels[chanName].users[e.Source.Name].Ident = e.Params[0]
		c.state.channels[chanName].users[e.Source.Name].Host = e.Params[1]
	}
	c.state.mu.Unlock()
}

func handleAWAY(c *Client, e Event) {
	c.state.mu.Lock()
	for chanName := range c.state.channels {
		if _, ok := c.state.channels[chanName].users[e.Source.Name]; !ok {
			continue
		}

		c.state.channels[chanName].users[e.Source.Name].Extras.Away = e.Trailing
	}
	c.state.mu.Unlock()
}
