// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
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
	// hasQuit is used to determine if we've finished quitting/cleaning up.
	hasQuit bool
	// reconnecting lets the internal state know a reconnect is occurring.
	reconnecting bool
	// nick is the tracker for our nickname on the server.
	nick string
	// channels represents all channels we're active in.
	channels map[string]*Channel
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
	// Name is the users "realname" or full name. Commonly contains links
	// to the IRC client being used, or something of non-importance. May also
	// be empty.
	Name string
	// FirstSeen represents the first time that the user was seen by the
	// client for the given channel.
	FirstSeen time.Time
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

// newState returns a clean client state.
func newState() *state {
	s := &state{}

	s.channels = make(map[string]*Channel)
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
		return channel.users[nick]
	}

	user = &User{Nick: nick, FirstSeen: time.Now()}
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

		// Delete the old reference.
		delete(s.channels[k].users, from)

		// In with the new.
		s.channels[k].users[to] = &source
	}
}
