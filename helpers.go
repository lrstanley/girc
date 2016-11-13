package girc

import (
	"time"

	"github.com/y0ssar1an/q"
)

func (c *Client) registerHelpers() {
	c.AddBgCallback(SUCCESS, handleWelcome)
	c.AddCallback(PING, handlePING)

	// joins/parts/anything that may add/remove users
	c.AddCallback(JOIN, handleJOIN)
	c.AddCallback(PART, handlePART)
	c.AddCallback(KICK, handleKICK)

	// WHO/WHOX responses
	c.AddCallback(RPL_WHOREPLY, handleWHO)
	c.AddCallback(RPL_WHOSPCRPL, handleWHO)

	// nickname collisions
	c.AddCallback(ERR_NICKNAMEINUSE, nickCollisionHandler)
	c.AddCallback(ERR_NICKCOLLISION, nickCollisionHandler)
	c.AddCallback(ERR_UNAVAILRESOURCE, nickCollisionHandler)
}

// handleWelcome is a helper function which lets the client know
// that enough time has passed and now they can send commands
//
// should always run in separate thread
func handleWelcome(c *Client, e *Event) {
	// this should be the nick that the server gives us. 99% of the time, it's the
	// one we supplied during connection, but some networks will insta-rename users.
	if len(e.Params) > 0 {
		c.State.nick = e.Params[0]
	}

	time.Sleep(2 * time.Second)

	c.Events <- &Event{Command: CONNECTED}
}

// nickCollisionHandler helps prevent the client from having conflicting
// nicknames with another bot, user, etc
func nickCollisionHandler(c *Client, e *Event) {
	c.SetNick(c.GetNick() + "_")
}

// handlePING helps respond to ping requests from the server
func handlePING(c *Client, e *Event) {
	// TODO: we should be sending pings too.
	c.Send(&Event{Command: PONG, Params: e.Params, Trailing: e.Trailing})
}

// handleJOIN ensures that the state has updated users and channels
func handleJOIN(c *Client, e *Event) {
	if len(e.Params) == 0 {
		return
	}

	// create it in state
	c.State.createChanIfNotExists(e.Params[0])

	c.Who(e.Params[0])
}

// handlePART ensures that the state is clean of old user and channel entries
func handlePART(c *Client, e *Event) {
	if len(e.Params) == 0 {
		return
	}

	if e.Prefix.Name == c.GetNick() {
		c.State.deleteChannel(e.Params[0])
		return
	}

	c.State.deleteUser(e.Params[0], e.Prefix.Name)
}

func handleWHO(c *Client, e *Event) {
	var channel, user, host, nick string

	// assume WHOX related
	if e.Command == RPL_WHOSPCRPL {
		if len(e.Params) != 6 {
			// assume there was some form of error or invalid WHOX response
			return
		}

		if e.Params[1] != "1" {
			// we should always be sending 1, and we should receive 1. if this
			// is anything but, then we didn't send the request and we can
			// ignore it.
			return
		}

		channel, user, host, nick = e.Params[2], e.Params[3], e.Params[4], e.Params[5]
	} else {
		channel, user, host, nick = e.Params[1], e.Params[2], e.Params[3], e.Params[5]
	}

	c.State.createUserIfNotExists(channel, nick, user, host)
}

func handleKICK(c *Client, e *Event) {
	if len(e.Params) < 2 {
		// needs at least channel and user
		return
	}

	q.Q(e)
}
