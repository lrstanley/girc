// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"sort"
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
	// users represents all of users that we're tracking.
	users map[string]*User
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

// reset resets the state back to it's original form.
func (s *state) reset() {
	s.mu.Lock()
	s.nick = ""
	s.ident = ""
	s.host = ""
	s.channels = make(map[string]*Channel)
	s.users = make(map[string]*User)
	s.serverOptions = make(map[string]string)
	s.enabledCap = []string{}
	s.motd = ""
	s.mu.Unlock()
}

// User represents an IRC user and the state attached to them.
type User struct {
	// Nick is the users current nickname. rfc1459 compliant.
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

	// Channels is a sorted list of all channels that we are currently tracking
	// the user in. Each channel name is rfc1459 compliant.
	Channels []string

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

// Copy returns a deep copy of the user which can be modified without making
// changes to the actual state.
func (u *User) Copy() *User {
	nu := &User{}
	*nu = *u

	_ = copy(nu.Channels, u.Channels)

	return nu
}

func (u *User) deleteChannel(name string) {
	name = ToRFC1459(name)

	j := -1
	for i := 0; i < len(u.Channels); i++ {
		if u.Channels[i] == name {
			j = i
			break
		}
	}

	if j != -1 {
		u.Channels = append(u.Channels[:j], u.Channels[j+1:]...)
	}
}

// InChannel checks to see if a user is in the given channel.
func (u *User) InChannel(name string) bool {
	name = ToRFC1459(name)

	for i := 0; i < len(u.Channels); i++ {
		if u.Channels[i] == name {
			return true
		}
	}

	return false
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
	// Name of the channel. Must be rfc1459 compliant.
	Name string
	// Topic of the channel.
	Topic string

	// Users is a sorted list of all users we are currently tracking within
	// the channel. Each is the nickname, and is rfc1459 compliant.
	Users []string
	// Joined represents the first time that the client joined the channel.
	Joined time.Time
	// Modes are the known channel modes that the bot has captured.
	Modes CModes
}

func (c *Channel) deleteUser(nick string) {
	nick = ToRFC1459(nick)

	j := -1
	for i := 0; i < len(c.Users); i++ {
		if c.Users[i] == nick {
			j = i
			break
		}
	}

	if j != -1 {
		c.Users = append(c.Users[:j], c.Users[j+1:]...)
	}
}

// Copy returns a deep copy of a given channel.
func (c *Channel) Copy() *Channel {
	nc := &Channel{}
	*nc = *c

	_ = copy(nc.Users, c.Users)

	// And modes.
	nc.Modes = c.Modes.Copy()

	return nc
}

// Len returns the count of users in a given channel.
func (c *Channel) Len() int {
	return len(c.Users)
}

// UserIn checks to see if a given user is in a channel.
func (c *Channel) UserIn(name string) bool {
	name = ToRFC1459(name)

	for i := 0; i < len(c.Users); i++ {
		if c.Users[i] == name {
			return true
		}
	}

	return false
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

	if _, ok := s.channels[ToRFC1459(name)]; ok {
		return s.channels[ToRFC1459(name)]
	}

	channel = &Channel{
		Name:   name,
		Users:  []string{},
		Joined: time.Now(),
		Modes:  NewCModes(supported, prefixes),
	}
	s.channels[ToRFC1459(name)] = channel

	return channel
}

// deleteChannel removes the channel from state, if not already done. Always
// use state.mu for transaction.
func (s *state) deleteChannel(name string) {
	name = ToRFC1459(name)

	_, ok := s.channels[name]
	if !ok {
		return
	}

	for _, user := range s.channels[name].Users {
		s.users[user].deleteChannel(name)

		if len(s.users[user].Channels) == 0 {
			// Assume we were only tracking them in this channel, and they
			// should be removed from state.

			delete(s.users, user)
		}
	}

	delete(s.channels, name)
}

// lookupChannel returns a reference to a channel, nil returned if no results
// found. Always use state.mu for transaction.
func (s *state) lookupChannel(name string) *Channel {
	if !IsValidChannel(name) {
		return nil
	}

	return s.channels[ToRFC1459(name)]
}

// lookupUser returns a reference to a user, nil returned if no results
// found. Always use state.mu for transaction.
func (s *state) lookupUser(name string) *User {
	if !IsValidNick(name) {
		return nil
	}

	return s.users[ToRFC1459(name)]
}

// createUserIfNotExists creates the channel and user in state, if not already
// done. Always use state.mu for transaction.
func (s *state) createUserIfNotExists(channelName, nick string) (user *User) {
	if !IsValidNick(nick) {
		return nil
	}

	channel := s.createChanIfNotExists(channelName)
	if channel == nil {
		return
	}

	user = s.lookupUser(nick)
	if user != nil {
		if !user.InChannel(channelName) {
			user.Channels = append(user.Channels, ToRFC1459(channelName))
			sort.StringsAreSorted(user.Channels)
		}

		user.LastActive = time.Now()
		return user
	}

	user = &User{
		Nick:       nick,
		FirstSeen:  time.Now(),
		LastActive: time.Now(),
	}
	s.users[ToRFC1459(nick)] = user
	channel.Users = append(channel.Users, ToRFC1459(nick))
	sort.Strings(channel.Users)

	return user
}

// deleteUser removes the user from channel state. Always use state.mu for
// transaction.
func (s *state) deleteUser(channelName, nick string) {
	if !IsValidNick(nick) {
		return
	}

	user := s.lookupUser(nick)
	if user == nil {
		return
	}

	if channelName == "" {
		for i := 0; i < len(user.Channels); i++ {
			s.channels[user.Channels[i]].deleteUser(nick)
		}

		delete(s.users, ToRFC1459(nick))
		return
	}

	channel := s.lookupChannel(channelName)
	if channel == nil {
		return
	}

	user.deleteChannel(channelName)
	channel.deleteUser(nick)

	if len(user.Channels) == 0 {
		// This means they are no longer in any channels we track, delete
		// them from state.

		delete(s.users, ToRFC1459(nick))
	}
}

// renameUser renames the user in state, in all locations where relevant.
// Always use state.mu for transaction.
func (s *state) renameUser(from, to string) {
	if !IsValidNick(from) || !IsValidNick(to) {
		return
	}

	from = ToRFC1459(from)

	// Update our nickname.
	if from == ToRFC1459(s.nick) {
		s.nick = to
	}

	user := s.lookupUser(from)
	if user == nil {
		return
	}

	delete(s.users, from)

	user.Nick = to
	user.LastActive = time.Now()
	s.users[ToRFC1459(to)] = user

	for i := 0; i < len(user.Channels); i++ {
		for j := 0; j < len(s.channels[user.Channels[i]].Users); j++ {
			if s.channels[user.Channels[i]].Users[j] == from {
				s.channels[user.Channels[i]].Users[j] = ToRFC1459(to)
			}
		}
	}
}
