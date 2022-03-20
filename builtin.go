// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
)

// registerBuiltin sets up built-in handlers, based on client
// configuration.
func (c *Client) registerBuiltins() {
	c.debug.Print("registering built-in handlers")

	c.Handlers.mu.Lock()
	defer c.Handlers.mu.Unlock()

	// Built-in things that should always be supported.
	c.Handlers.register(true, true, RPL_WELCOME, HandlerFunc(handleConnect))
	c.Handlers.register(true, false, PING, HandlerFunc(handlePING))
	c.Handlers.register(true, false, PONG, HandlerFunc(handlePONG))

	// Nickname collisions.
	c.Handlers.register(true, false, ERR_NICKNAMEINUSE, HandlerFunc(nickCollisionHandler))
	c.Handlers.register(true, false, ERR_NICKCOLLISION, HandlerFunc(nickCollisionHandler))
	c.Handlers.register(true, false, ERR_UNAVAILRESOURCE, HandlerFunc(nickCollisionHandler))

	if c.Config.disableTracking {
		return
	}

	// Joins/parts/anything that may add/remove/rename users.
	c.Handlers.register(true, false, JOIN, HandlerFunc(handleJOIN))
	c.Handlers.register(true, false, PART, HandlerFunc(handlePART))
	c.Handlers.register(true, false, KICK, HandlerFunc(handleKICK))
	c.Handlers.register(true, false, QUIT, HandlerFunc(handleQUIT))
	c.Handlers.register(true, false, NICK, HandlerFunc(handleNICK))
	c.Handlers.register(true, false, RPL_NAMREPLY, HandlerFunc(handleNAMES))

	// Modes.
	c.Handlers.register(true, false, MODE, HandlerFunc(handleMODE))
	c.Handlers.register(true, false, RPL_CHANNELMODEIS, HandlerFunc(handleMODE))

	// Channel creation time.
	c.Handlers.register(true, false, RPL_CREATIONTIME, HandlerFunc(handleCREATIONTIME))

	// WHO/WHOX responses.
	c.Handlers.register(true, false, RPL_WHOREPLY, HandlerFunc(handleWHO))
	c.Handlers.register(true, false, RPL_WHOSPCRPL, HandlerFunc(handleWHO))

	// Other misc. useful stuff.
	c.Handlers.register(true, false, TOPIC, HandlerFunc(handleTOPIC))
	c.Handlers.register(true, false, RPL_TOPIC, HandlerFunc(handleTOPIC))
	c.Handlers.register(true, false, RPL_YOURHOST, HandlerFunc(handleYOURHOST))
	c.Handlers.register(true, false, RPL_CREATED, HandlerFunc(handleCREATED))
	c.Handlers.register(true, false, RPL_ISUPPORT, HandlerFunc(handleISUPPORT))
	c.Handlers.register(true, false, RPL_LUSERCHANNELS, HandlerFunc(handleLUSERCHANNELS)) // 254
	c.Handlers.register(true, false, RPL_GLOBALUSERS, HandlerFunc(handleGLOBALUSERS))     // 266
	c.Handlers.register(true, false, RPL_LOCALUSERS, HandlerFunc(handleLOCALUSERS))       // 265
	c.Handlers.register(true, false, RPL_LUSEROP, HandlerFunc(handleLUSEROP))             // 252
	c.Handlers.register(true, false, RPL_MOTDSTART, HandlerFunc(handleMOTD))
	c.Handlers.register(true, false, RPL_MOTD, HandlerFunc(handleMOTD))
	// c.Handlers.register(true, false, RPL_MYINFO, HandlerFunc(handleMYINFO))

	// Keep users lastactive times up to date.
	c.Handlers.register(true, false, PRIVMSG, HandlerFunc(updateLastActive))
	c.Handlers.register(true, false, NOTICE, HandlerFunc(updateLastActive))
	c.Handlers.register(true, false, TOPIC, HandlerFunc(updateLastActive))
	c.Handlers.register(true, false, KICK, HandlerFunc(updateLastActive))

	// CAP IRCv3-specific tracking and functionality.
	c.Handlers.register(true, false, CAP, HandlerFunc(handleCAP))
	c.Handlers.register(true, false, CAP_CHGHOST, HandlerFunc(handleCHGHOST))
	c.Handlers.register(true, false, CAP_AWAY, HandlerFunc(handleAWAY))
	c.Handlers.register(true, false, CAP_ACCOUNT, HandlerFunc(handleACCOUNT))
	c.Handlers.register(true, false, ALL_EVENTS, HandlerFunc(handleTags))

	// SASL IRCv3 support.
	c.Handlers.register(true, false, AUTHENTICATE, HandlerFunc(handleSASL))
	c.Handlers.register(true, false, RPL_SASLSUCCESS, HandlerFunc(handleSASL))
	c.Handlers.register(true, false, RPL_NICKLOCKED, HandlerFunc(handleSASLError))
	c.Handlers.register(true, false, ERR_SASLFAIL, HandlerFunc(handleSASLError))
	c.Handlers.register(true, false, ERR_SASLTOOLONG, HandlerFunc(handleSASLError))
	c.Handlers.register(true, false, ERR_SASLABORTED, HandlerFunc(handleSASLError))
	c.Handlers.register(true, false, RPL_SASLMECHS, HandlerFunc(handleSASLError))
	return
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
		c.state.nick.Store(e.Params[0])
		c.state.notify(c, UPDATE_GENERAL)
		split := strings.Split(e.Params[1], " ")
	search:
		for i, artifact := range split {
			switch strings.ToLower(artifact) {
			case "welcome", "to":
				continue
			case "the":
				if len(split) < i {
					break search
				}
				c.IRCd.Network.Store(split[i+1])
				break search
			default:
				break search
			}
		}
	}

	time.Sleep(2 * time.Second)

	server := c.server()
	c.RunHandlers(&Event{Command: CONNECTED, Params: []string{server}})
}

// nickCollisionHandler helps prevent the client from having conflicting
// nicknames with another bot, user, etc.
func nickCollisionHandler(c *Client, e Event) {
	if c.Config.HandleNickCollide == nil {
		c.Cmd.Nick(c.GetNick() + "_")
		return
	}

	newNick := c.Config.HandleNickCollide(c.GetNick())
	if newNick != "" {
		c.Cmd.Nick(newNick)
	}
}

// handlePING helps respond to ping requests from the server.
func handlePING(c *Client, e Event) {
	c.Cmd.Pong(e.Last())
}

func handlePONG(c *Client, e Event) {
	c.conn.lastPong.Store(time.Now())
}

// handleJOIN ensures that the state has updated users and channels.
func handleJOIN(c *Client, e Event) {
	if e.Source == nil || len(e.Params) == 0 {
		return
	}

	channelName := e.Params[0]

	channel := c.state.lookupChannel(channelName)
	if channel == nil {
		if ok := c.state.createChannel(channelName); !ok {
			return
		}

		channel = c.state.lookupChannel(channelName)
	}

	user := c.state.lookupUser(e.Source.Name)
	if user == nil {
		if _, ok := c.state.createUser(e.Source); !ok {
			return
		}
		user = c.state.lookupUser(e.Source.Name)
	}

	defer c.state.notify(c, UPDATE_STATE)

	channel.addUser(user.Nick, user)
	user.addChannel(channel.Name, channel)

	// Assume extended-join (ircv3).
	if len(e.Params) >= 2 {
		if e.Params[1] != "*" {
			user.Extras.Account = e.Params[1]
		}

		if len(e.Params) > 2 {
			user.Extras.Name = e.Params[2]
		}
	}

	if e.Source.ID() == c.GetID() {
		// If it's us, don't just add our user to the list. Run a WHO which
		// will tell us who exactly is in the entire channel.
		c.Send(&Event{Command: WHO, Params: []string{channelName, "%tacuhnr,1"}})

		// Also send a MODE to obtain the list of channel modes.
		c.Send(&Event{Command: MODE, Params: []string{channelName}})

		// Update our ident and host too, in state -- since there is no
		// cleaner method to do this.
		c.state.ident.Store(e.Source.Ident)
		c.state.host.Store(e.Source.Host)
		return
	}

	// Only WHO the user, which is more efficient.
	c.Send(&Event{Command: WHO, Params: []string{e.Source.Name, "%tacuhnr,1"}})
}

// handlePART ensures that the state is clean of old user and channel entries.
func handlePART(c *Client, e Event) {
	if e.Source == nil || len(e.Params) < 1 {
		return
	}

	c.state.Lock()
	defer c.state.Unlock()

	c.debug.Println("handlePart")
	defer c.debug.Println("handlePart done for " + e.Params[0])

	// TODO: does this work if it's not the bot?
	// er yes, but needs a test case

	channel := e.Params[0]

	if channel == "" {
		return
	}

	defer c.state.notify(c, UPDATE_STATE)

	if chn := c.LookupChannel(channel); chn != nil {
		chn.UserList.Remove(e.Source.ID())
		c.state.Unlock()
		c.debug.Println(fmt.Sprintf("removed: %s, new count: %d", e.Source.ID(), chn.Len()))
		c.state.Lock()
	} else {
		c.debug.Println("failed to lookup channel: " + channel)
	}

	if e.Source.ID() == c.GetID() {
		c.state.deleteChannel(channel)
		return
	}

	c.state.deleteUser(channel, e.Source.ID())

}

// handleCREATIONTIME handles incoming TOPIC events and keeps channel tracking info
// updated with the latest channel topic.
func handleCREATIONTIME(c *Client, e Event) {
	var created string
	var name string
	switch len(e.Params) {
	case 0, 1, 2:
		return
	default:
		name = e.Params[1]
		created = e.Params[2]
		break
	}

	channel := c.state.lookupChannel(name)
	if channel == nil {
		return
	}

	channel.Created = created
	c.state.notify(c, UPDATE_STATE)
}

// handleTOPIC handles incoming TOPIC events and keeps channel tracking info
// updated with the latest channel topic.
func handleTOPIC(c *Client, e Event) {
	var name string
	switch len(e.Params) {
	case 0:
		return
	case 1:
		name = e.Params[0]
	default:
		name = e.Params[1]
	}

	channel := c.state.lookupChannel(name)
	if channel == nil {

		return
	}

	channel.Topic = e.Last()

	c.state.notify(c, UPDATE_STATE)
}

// handlWHO updates our internal tracking of users/channels with WHO/WHOX
// information.
func handleWHO(c *Client, e Event) {
	var ident, host, nick, account, realname string

	// Assume WHOX related.
	if e.Command == RPL_WHOSPCRPL {
		if len(e.Params) != 8 {
			// Assume there was some form of error or invalid WHOX response.
			return
		}

		if e.Params[1] != "1" {
			// We should always be sending 1, and we should receive 1. If this
			// is anything but, then we didn't send the request and we can
			// ignore it.
			return
		}

		ident, host, nick, account = e.Params[3], e.Params[4], e.Params[5], e.Params[6]
		realname = e.Last()
	} else {
		// Assume RPL_WHOREPLY.
		// format: "<client> <channel> <user> <host> <server> <nick> <H|G>[*][@|+] :<hopcount> <real_name>"
		ident, host, nick, realname = e.Params[2], e.Params[3], e.Params[5], e.Last()

		// Strip the numbers from "<hopcount> <realname>"
		for i := 0; i < len(realname); i++ {
			// Check if it's not 0-9.
			if realname[i] < 0x30 || i > 0x39 {
				realname = strings.TrimLeft(realname[i+1:], " ")
				break
			}

			if i == len(realname)-1 {
				// Assume it's only numbers?
				realname = ""
			}
		}
	}

	user := c.state.lookupUser(nick)
	if user == nil {
		usr, _ := c.state.createUser(&Source{nick, ident, host})
		usr.Extras.Name = realname
		if account != "0" {
			usr.Extras.Account = account
		}
		c.state.notify(c, UPDATE_STATE)
		return
	}

	user.Host = host
	user.Ident = ident
	user.Mask = user.Nick + "!" + user.Ident + "@" + user.Host
	user.Extras.Name = realname

	if account != "0" {
		user.Extras.Account = account
	}

	c.state.notify(c, UPDATE_STATE)
}

// handleKICK ensures that users are cleaned up after being kicked from the
// channel
func handleKICK(c *Client, e Event) {
	if len(e.Params) < 2 {
		// Needs at least channel and user.
		return
	}

	defer c.state.notify(c, UPDATE_STATE)

	if e.Params[1] == c.GetNick() {

		c.state.deleteChannel(e.Params[0])

		return
	}

	// Assume it's just another user.

	c.state.deleteUser(e.Params[0], e.Params[1])

}

// handleNICK ensures that users are renamed in state, or the client name is
// up to date.
func handleNICK(c *Client, e Event) {
	if e.Source == nil {
		return
	}

	// renameUser updates the LastActive time automatically.
	if len(e.Params) >= 1 {
		c.state.renameUser(e.Source.ID(), e.Last())
	}
	c.state.notify(c, UPDATE_STATE)
}

// handleQUIT handles users that are quitting from the network.
func handleQUIT(c *Client, e Event) {
	if e.Source == nil {
		return
	}

	if e.Source.ID() == c.GetID() {
		return
	}

	c.state.deleteUser("", e.Source.ID())

	c.state.notify(c, UPDATE_STATE)
}

func handleGLOBALUSERS(c *Client, e Event) {
	cusers, err := strconv.Atoi(e.Params[0])
	if err != nil {
		return
	}
	musers, err := strconv.Atoi(e.Params[1])
	if err != nil {
		return
	}
	c.IRCd.UserCount = cusers
	c.IRCd.MaxUserCount = musers
}

func handleLOCALUSERS(c *Client, e Event) {
	cusers, err := strconv.Atoi(e.Params[1])
	if err != nil {
		return
	}
	musers, err := strconv.Atoi(e.Params[2])
	if err != nil {
		return
	}
	c.IRCd.LocalUserCount = cusers
	c.IRCd.LocalMaxUserCount = musers
}

func handleLUSERCHANNELS(c *Client, e Event) {
	ccount, err := strconv.Atoi(e.Params[1])
	if err != nil {
		return
	}
	c.IRCd.ChannelCount = ccount
}

func handleLUSEROP(c *Client, e Event) {
	ocount, err := strconv.Atoi(e.Params[1])
	if err != nil {
		return
	}
	c.IRCd.OperCount = ocount
}

// handleCREATED handles incoming CREATED events.
// This is commonly used to tell us when the IRC daemon was compiled.
func handleCREATED(c *Client, e Event) {
	split := strings.Split(e.Params[1], " ")
	days := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	found := -1
	for i, word := range split {
		for _, day := range days {
			if word == day+"," {
				found = i
				break
			}
		}
	}
	if found == -1 {
		return
	}
	compiled, err := dateparse.ParseAny(strings.Join(split[found:], " "))
	if err != nil {
		return
	}
	c.IRCd.Compiled = compiled
	c.state.notify(c, UPDATE_GENERAL)
}

// handleYOURHOST handles incoming YOURHOST events.
// This is commonly used to tell us details on the currently connected leaf.
func handleYOURHOST(c *Client, e Event) {
	var host = ""
	var ver = ""
	const prefix = "Your host is "
	const suffix = " running version "
	if strings.Contains(e.Params[1], prefix) && strings.Contains(e.Params[1], ",") {
		s := strings.TrimPrefix(e.Params[1], prefix)
		split := strings.Split(s, ",")
		host = split[0]
		ver = strings.Replace(split[1], suffix, "", 1)
	}
	if len(host)+len(ver) == 0 {
		return
	}
	c.IRCd.Host = host
	c.IRCd.Version = ver
	c.state.notify(c, UPDATE_GENERAL)
}

// handleISUPPORT handles incoming RPL_ISUPPORT (also known as RPL_PROTOCTL)
// events. This commonly contains the date of the daemon's compilation.
func handleISUPPORT(c *Client, e Event) {
	// Must be a ISUPPORT-based message.

	// Also known as RPL_PROTOCTL.
	if !strings.HasSuffix(e.Last(), "this server") {
		return
	}

	// Must have at least one configuration.
	if len(e.Params) < 2 {
		return
	}

	// Skip the first parameter, as it's our nickname, and the last, as it's the doc.
	for i := range e.Params {
		split := strings.Split(e.Params[i], "=")

		if len(split) != 2 {
			c.state.serverOptions.Set(e.Params[i], "")
			continue
		}

		if len(split[0]) < 1 || len(split[1]) < 1 {
			c.state.serverOptions.Set(e.Params[i], "")
			continue
		}

		if split[0] == "NETWORK" {
			c.state.network.Store(split[1])
		}

		c.state.serverOptions.Set(split[0], split[1])
	}

	c.state.notify(c, UPDATE_GENERAL)
}

// handleMOTD handles incoming MOTD messages and buffers them up for use with
// Client.ServerMOTD().
func handleMOTD(c *Client, e Event) {
	defer c.state.notify(c, UPDATE_GENERAL)

	// Beginning of the MOTD.
	if e.Command == RPL_MOTDSTART {
		c.state.motd = ""
		return
	}

	// Otherwise, assume we're getting sent the MOTD line-by-line.
	if len(c.state.motd) != 0 {
		c.state.motd += "\n"
	}
	c.state.motd += e.Last()
}

// handleNAMES handles incoming NAMES queries, of which lists all users in
// a given channel. Optionally also obtains ident/host values, as well as
// permissions for each user, depending on what capabilities are enabled.
func handleNAMES(c *Client, e Event) {
	if len(e.Params) < 1 {
		return
	}

	channel := c.state.lookupChannel(e.Params[2])
	if channel == nil {
		return
	}

	parts := strings.Split(e.Last(), " ")

	var modes, nick string
	var ok bool

	for i := 0; i < len(parts); i++ {
		modes, nick, ok = parseUserPrefix(parts[i])
		if !ok {
			continue
		}

		var s = new(Source)

		// If userhost-in-names.
		if strings.Contains(nick, "@") {
			s = ParseSource(nick)
			if s == nil {
				continue
			}

		} else {
			s = &Source{
				Name: nick,
			}

			if !IsValidNick(s.Name) {
				continue
			}
		}

		c.state.createUser(s)
		user := c.state.lookupUser(s.Name)
		if user == nil {
			continue
		}

		user.addChannel(channel.Name, channel)
		channel.addUser(s.ID(), user)

		// Don't append modes, overwrite them.
		perms, _ := user.Perms.Lookup(channel.Name)
		perms.set(modes, false)
		user.Perms.set(channel.Name, perms)
	}
	c.state.notify(c, UPDATE_STATE)
}

// updateLastActive is a wrapper for any event which the source author
// should have it's LastActive time updated. This is useful for things like
// a KICK where we know they are active, as they just kicked another user,
// even though they may not be talking.
func updateLastActive(c *Client, e Event) {
	if e.Source == nil {
		return
	}

	// Update the users last active time, if they exist.
	user := c.state.lookupUser(e.Source.Name)
	if user == nil {
		return
	}

	user.LastActive = time.Now()
}
