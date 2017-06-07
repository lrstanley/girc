// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"strings"
	"sync"
	"time"
)

// state represents the actively-changing variables within the client
// runtime.
type state struct {
	// m is a RW mutex lock, used to guard the state from goroutines causing
	// corruption.
	mu sync.RWMutex
	// nick, ident, and host are the internal trackers for our user.
	nick, ident, host string
	// channels represents all channels we're active in.
	channels map[string]*Channel
	// enabledCap are the capabilities which are enabled for this connection.
	enabledCap []string
	// tmpCap are the capabilties which we share with the server during the
	// last capability check. These will get sent once we have received the
	// last capability list command from the server.
	tmpCap []string
	// serverOptions are the standard capabilities and configurations
	// supported by the server at connection time. This also includes
	// RPL_ISUPPORT entries.
	serverOptions map[string]string
	// motd is the servers message of the day.
	motd string
}

func (s *state) clean() {
	s.mu.Lock()
	s.nick = ""
	s.ident = ""
	s.host = ""
	s.channels = make(map[string]*Channel)
	s.serverOptions = make(map[string]string)
	s.enabledCap = []string{}
	s.motd = ""
	s.mu.Unlock()
}

// User represents an IRC user and the state attached to them.
type User struct {
	// Nick is the users current nickname.
	Nick string
	// Ident is the users username/ident. Ident is commonly prefixed with a
	// "~", which indicates that they do not have a identd server setup for
	// authentication.
	Ident string
	// Host is the visible host of the users connection that the server has
	// provided to us for their connection. May not always be accurate due to
	// many networks spoofing/hiding parts of the hostname for privacy
	// reasons.
	Host string

	// FirstSeen represents the first time that the user was seen by the
	// client for the given channel. Only usable if from state, not in past.
	FirstSeen time.Time
	// LastActive represents the last time that we saw the user active,
	// which could be during nickname change, message, channel join, etc.
	// Only usable if from state, not in past.
	LastActive time.Time

	// Perms are the user permissions applied to this user that affect the given
	// channel. This supports non-rfc style modes like Admin, Owner, and HalfOp.
	// If you want to easily check if a user has permissions equal or greater
	// than OP, use Perms.IsAdmin().
	Perms UserPerms

	// Extras are things added on by additional tracking methods, which may
	// or may not work on the IRC server in mention.
	Extras struct {
		// Name is the users "realname" or full name. Commonly contains links
		// to the IRC client being used, or something of non-importance. May
		// also be empty if unsupported by the server/tracking is disabled.
		Name string
		// Account refers to the account which the user is authenticated as.
		// This differs between each network (e.g. usually Nickserv, but
		// could also be something like Undernet). May also be empty if
		// unsupported by the server/tracking is disabled.
		Account string
		// Away refers to the away status of the user. An empty string
		// indicates that they are active, otherwise the string is what they
		// set as their away message. May also be empty if unsupported by the
		// server/tracking is disabled.
		Away string
	}
}

// Lifetime represents the amount of time that has passed since we have first
// seen the user.
func (u *User) Lifetime() time.Duration {
	return time.Since(u.FirstSeen)
}

// Active represents the the amount of time that has passed since we have
// last seen the user.
func (u *User) Active() time.Duration {
	return time.Since(u.LastActive)
}

// IsActive returns true if they were active within the last 30 minutes.
func (u *User) IsActive() bool {
	return u.Active() < (time.Minute * 30)
}

// Channel represents an IRC channel and the state attached to it.
type Channel struct {
	// Name of the channel. Must be rfc compliant. Always represented as
	// lower-case, to ensure that the channel is only being tracked once.
	Name string
	// Topic of the channel.
	Topic string
	// users represents the users that we can currently see within the
	// channel.
	users map[string]*User
	// Joined represents the first time that the client joined the channel.
	Joined time.Time
	// Modes are the known channel modes that the bot has captured.
	Modes CModes
}

// Copy returns a deep copy of a given channel.
func (c *Channel) Copy() *Channel {
	nc := &Channel{}
	*nc = *c

	// Copy the users.
	nc.users = make(map[string]*User)
	for k, v := range c.users {
		nc.users[k] = v
	}

	// And modes.
	nc.Modes = c.Modes.Copy()

	return nc
}

// Users returns a list of users in a given channel.
func (c *Channel) Users() []*User {
	out := make([]*User, len(c.users))

	var index int
	for _, u := range c.users {
		out[index] = u

		index++
	}

	return out
}

// NickList returns a list of nicknames in a given channel.
func (c *Channel) NickList() []string {
	out := make([]string, len(c.users))

	var index int
	for k := range c.users {
		out[index] = k

		index++
	}

	return out
}

// Len returns the count of users in a given channel.
func (c *Channel) Len() int {
	return len(c.users)
}

// Lookup looks up a user in a channel based on a given nickname. If the
// user wasn't found, user is nil.
func (c *Channel) Lookup(nick string) *User {
	for k, v := range c.users {
		if ToRFC1459(k) == ToRFC1459(nick) {
			// No need to have a copy, as if one has access to a channel,
			// should already have a full copy.
			return v
		}
	}

	return nil
}

// Lifetime represents the amount of time that has passed since we have first
// joined the channel.
func (c *Channel) Lifetime() time.Duration {
	return time.Since(c.Joined)
}

// createChanIfNotExists creates the channel in state, if not already done.
// Always use state.mu for transaction.
func (s *state) createChanIfNotExists(name string) (channel *Channel) {
	// Not a valid channel.
	if !IsValidChannel(name) {
		return nil
	}

	supported := s.chanModes()
	prefixes, _ := parsePrefixes(s.userPrefixes())

	name = strings.ToLower(name)
	if _, ok := s.channels[name]; !ok {
		channel = &Channel{
			Name:   name,
			users:  make(map[string]*User),
			Joined: time.Now(),
			Modes:  NewCModes(supported, prefixes),
		}
		s.channels[name] = channel
	} else {
		channel = s.channels[name]
	}

	return channel
}

// deleteChannel removes the channel from state, if not already done. Always
// use state.mu for transaction.
func (s *state) deleteChannel(name string) {
	channel := s.createChanIfNotExists(name)
	if channel == nil {
		return
	}

	if _, ok := s.channels[channel.Name]; ok {
		delete(s.channels, channel.Name)
	}
}

// lookupChannel returns a reference to a channel with a given case-insensitive
// name. nil returned if no results found.
func (s *state) lookupChannel(name string) *Channel {
	if !IsValidChannel(name) {
		return nil
	}

	return s.channels[strings.ToLower(name)]
}

// createUserIfNotExists creates the channel and user in state, if not already
// done. Always use state.mu for transaction.
func (s *state) createUserIfNotExists(channelName, nick string) (user *User) {
	if !IsValidNick(nick) {
		return nil
	}

	channel := s.createChanIfNotExists(channelName)
	if channel == nil {
		return nil
	}

	if _, ok := channel.users[nick]; ok {
		channel.users[nick].LastActive = time.Now()
		return channel.users[nick]
	}

	user = &User{Nick: nick, FirstSeen: time.Now(), LastActive: time.Now()}
	channel.users[nick] = user

	return user
}

// deleteUser removes the user from channel state. Always use state.mu for
// transaction.
func (s *state) deleteUser(nick string) {
	if !IsValidNick(nick) {
		return
	}

	for k := range s.channels {
		// Check to see if they're in this channel.
		if _, ok := s.channels[k].users[nick]; !ok {
			continue
		}

		delete(s.channels[k].users, nick)
	}
}

// renameUser renames the user in state, in all locations where relevant.
// Always use state.mu for transaction.
func (s *state) renameUser(from, to string) {
	if !IsValidNick(from) || !IsValidNick(to) {
		return
	}

	// Update our nickname.
	if from == s.nick {
		s.nick = to
	}

	for k := range s.channels {
		// Check to see if they're in this channel.
		if _, ok := s.channels[k].users[from]; !ok {
			continue
		}

		// Take the actual reference to the pointer.
		source := *s.channels[k].users[from]

		// Update the nick field (as we not only have a key, but a matching
		// struct field).
		source.Nick = to
		source.LastActive = time.Now()

		// Delete the old reference.
		delete(s.channels[k].users, from)

		// In with the new.
		s.channels[k].users[to] = &source
	}
}

// lookupUsers returns a slice of references to users matching a given
// query. mathType is of "nick", "name", "ident" or "account".
func (s *state) lookupUsers(matchType, toMatch string) []*User {
	var users []*User

	for c := range s.channels {
		for u := range s.channels[c].users {
			switch matchType {
			case "nick":
				if ToRFC1459(s.channels[c].users[u].Nick) == ToRFC1459(toMatch) {
					users = append(users, s.channels[c].users[u])
					continue
				}
			case "ident":
				if ToRFC1459(s.channels[c].users[u].Ident) == ToRFC1459(toMatch) {
					users = append(users, s.channels[c].users[u])
					continue
				}
			case "account":
				if ToRFC1459(s.channels[c].users[u].Extras.Account) == ToRFC1459(toMatch) {
					users = append(users, s.channels[c].users[u])
					continue
				}
			}
		}
	}

	return users
}
