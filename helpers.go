// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import "time"

// registerHelpers sets up built-in callbacks/helpers, based on client
// configuration.
func (c *Client) registerHelpers() {
	// Built-in things that should always be supported.
	c.Callbacks.register(true, "routine", SUCCESS, CallbackFunc(handleConnect))
	c.Callbacks.register(true, "std", PING, CallbackFunc(handlePING))

	if !c.Config.DisableTracking {
		// Joins/parts/anything that may add/remove/rename users.
		c.Callbacks.register(true, "std", JOIN, CallbackFunc(handleJOIN))
		c.Callbacks.register(true, "std", PART, CallbackFunc(handlePART))
		c.Callbacks.register(true, "std", KICK, CallbackFunc(handleKICK))
		c.Callbacks.register(true, "std", QUIT, CallbackFunc(handleQUIT))
		c.Callbacks.register(true, "std", NICK, CallbackFunc(handleNICK))

		// WHO/WHOX responses.
		c.Callbacks.register(true, "std", RPL_WHOREPLY, CallbackFunc(handleWHO))
		c.Callbacks.register(true, "std", RPL_WHOSPCRPL, CallbackFunc(handleWHO))
	}

	// Nickname collisions.
	if !c.Config.DisableNickCollision {
		c.Callbacks.register(true, "std", ERR_NICKNAMEINUSE, CallbackFunc(nickCollisionHandler))
		c.Callbacks.register(true, "std", ERR_NICKCOLLISION, CallbackFunc(nickCollisionHandler))
		c.Callbacks.register(true, "std", ERR_UNAVAILRESOURCE, CallbackFunc(nickCollisionHandler))
	}
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
	c.SetNick(c.GetNick() + "_")
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

	c.state.createChanIfNotExists(e.Params[0])

	if e.Source.Name == c.GetNick() {
		// If it's us, don't just add our user to the list. Run a WHO which
		// will tell us who exactly is in the channel.
		c.Send(&Event{Command: WHO, Params: []string{e.Params[0], "%tcuhn,1"}})
		return
	}

	// Create the user in state. Only WHO the user, which is more efficient.
	c.Send(&Event{Command: WHO, Params: []string{e.Source.Name, "%tcuhn,1"}})
}

// handlePART ensures that the state is clean of old user and channel entries.
func handlePART(c *Client, e Event) {
	if len(e.Params) == 0 {
		return
	}

	if e.Source.Name == c.GetNick() {
		c.state.deleteChannel(e.Params[0])
		return
	}

	c.state.deleteUser(e.Source.Name)
}

// handlWHO updates our internal tracking of users/channels with WHO/WHOX
// information.
func handleWHO(c *Client, e Event) {
	var channel, user, host, nick string

	// Assume WHOX related.
	if e.Command == RPL_WHOSPCRPL {
		if len(e.Params) != 6 {
			// Assume there was some form of error or invalid WHOX response.
			return
		}

		if e.Params[1] != "1" {
			// We should always be sending 1, and we should receive 1. If this
			// is anything but, then we didn't send the request and we can
			// ignore it.
			return
		}

		channel, user, host, nick = e.Params[2], e.Params[3], e.Params[4], e.Params[5]
	} else {
		channel, user, host, nick = e.Params[1], e.Params[2], e.Params[3], e.Params[5]
	}

	c.state.createUserIfNotExists(channel, nick, user, host)
}

// handleKICK ensures that users are cleaned up after being kicked from the
// channel
func handleKICK(c *Client, e Event) {
	if len(e.Params) < 2 {
		// Needs at least channel and user.
		return
	}

	if e.Params[1] == c.GetNick() {
		c.state.deleteChannel(e.Params[0])
		return
	}

	// Assume it's just another user.
	c.state.deleteUser(e.Params[1])
}

// handleNICK ensures that users are renamed in state, or the client name is
// up to date.
func handleNICK(c *Client, e Event) {
	if len(e.Params) != 1 {
		// Something erronous was sent to us.
		return
	}

	c.state.renameUser(e.Source.Name, e.Params[0])
}

func handleQUIT(c *Client, e Event) {
	c.state.deleteUser(e.Source.Name)
}
