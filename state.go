// Copyright 2016-2017 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"fmt"
	"net"
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

	// reader is the socket buffer reader from the IRC server.
	reader *ircDecoder
	// reader is the socket buffer write to the IRC server.
	writer *ircEncoder
	// conn is a net.Conn reference to the IRC server.
	conn net.Conn

	// connected is true if we're actively connected to a server.
	connected bool
	// connTime is the time at which the client has connected to a server.
	connTime *time.Time
	// quitting is used to determine if we've finished quitting/cleaning up.
	quitting bool
	// reconnecting lets the internal state know a reconnect is occurring.
	reconnecting bool
	// nick is the tracker for our nickname on the server.
	nick string
	// channels represents all channels we're active in.
	channels map[string]*Channel
	// enabledCap are the capabilities which are enabled for this connection.
	enabledCap []string
	// tmpCap are the capabilties which we share with the server during the
	// last capability check. These will get sent once we have received the
	// last capability list command from the server.
	tmpCap []string
	// serverOptions are the standard capabilities and configurations
	// supported by the server at connection time. This also includes ISUPPORT
	// entries.
	serverOptions map[string]string
	// motd is the servers message of the day.
	motd string
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

// Message returns an event which can be used to send a response to the user
// as a private message.
func (u *User) Message(message string) *Event {
	return &Event{Command: PRIVMSG, Params: []string{u.Nick}, Trailing: message}
}

// Messagef returns an event which can be used to send a response to the user
// as a private message. format is a printf format string, which a's
// arbitrary arguments will be passed to.
func (u *User) Messagef(format string, a ...interface{}) *Event {
	return u.Message(fmt.Sprintf(format, a...))
}

// MessageTo returns an event which can be used to send a response to the
// user in a channel as a private message.
func (u *User) MessageTo(channel, message string) *Event {
	return &Event{Command: PRIVMSG, Params: []string{u.Nick}, Trailing: channel + ": " + message}
}

// MessageTof returns an event which can be used to send a response to the
// channel. format is a printf format string, which a's arbitrary arguments
// will be passed to.
func (u *User) MessageTof(channel, format string, a ...interface{}) *Event {
	return u.MessageTo(channel, fmt.Sprintf(format, a...))
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
}

// Message returns an event which can be used to send a response to the channel.
func (c *Channel) Message(message string) *Event {
	return &Event{Command: PRIVMSG, Params: []string{c.Name}, Trailing: message}
}

// Messagef returns an event which can be used to send a response to the
// channel. format is a printf format string, which a's arbitrary arguments
// will be passed to.
func (c *Channel) Messagef(format string, a ...interface{}) *Event {
	return c.Message(fmt.Sprintf(format, a...))
}

// Lifetime represents the amount of time that has passed since we have first
// joined the channel.
func (c *Channel) Lifetime() time.Duration {
	return time.Since(c.Joined)
}

// newState returns a clean client state.
func newState() *state {
	s := &state{}

	s.channels = make(map[string]*Channel)
	s.serverOptions = make(map[string]string)
	s.connected = false

	return s
}

// createChanIfNotExists creates the channel in state, if not already done.
// Always use state.mu for transaction.
func (s *state) createChanIfNotExists(name string) (channel *Channel) {
	// Not a valid channel.
	if !IsValidChannel(name) {
		return nil
	}

	name = strings.ToLower(name)
	if _, ok := s.channels[name]; !ok {
		channel = &Channel{
			Name:   name,
			users:  make(map[string]*User),
			Joined: time.Now(),
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

func (s *state) getUsers(matchType, toMatch string) []*User {
	var users []*User

	for c := range s.channels {
		for u := range s.channels[c].users {
			switch matchType {
			case "nick":
				if s.channels[c].users[u].Nick == toMatch {
					users = append(users, s.channels[c].users[u])
					continue
				}
			case "name":
				if s.channels[c].users[u].Extras.Name == toMatch {
					users = append(users, s.channels[c].users[u])
					continue
				}
			case "ident":
				if s.channels[c].users[u].Ident == toMatch {
					users = append(users, s.channels[c].users[u])
					continue
				}
			case "account":
				if s.channels[c].users[u].Extras.Account == toMatch {
					users = append(users, s.channels[c].users[u])
					continue
				}
			}
		}
	}

	return users
}
