// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

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
	m sync.RWMutex
	// connected is true if we're actively connected to a server.
	connected bool
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
	// FirstSeen represents the first time that the user was seen by the
	// client for the given channel.
	FirstSeen time.Time
}

// Channel represents an IRC channel and the state attached to it.
type Channel struct {
	// Name of the channel. Must be rfc compliant. Always represented as
	// lower-case, to ensure that the channel is only being tracked once.
	Name string
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
func (s *state) createChanIfNotExists(channel string) {
	channel = strings.ToLower(channel)

	// Not a valid channel.
	if !IsValidChannel(channel) {
		return
	}

	s.m.Lock()
	if _, ok := s.channels[channel]; !ok {
		s.channels[channel] = &Channel{
			Name:   channel,
			users:  make(map[string]*User),
			Joined: time.Now(),
		}
	}
	s.m.Unlock()
}

// deleteChannel removes the channel from state, if not already done.
func (s *state) deleteChannel(channel string) {
	channel = strings.ToLower(channel)
	s.createChanIfNotExists(channel)

	s.m.Lock()
	if _, ok := s.channels[channel]; ok {
		delete(s.channels, channel)
	}
	s.m.Unlock()
}

// createUserIfNotExists creates the channel and user in state, if not already
// done.
func (s *state) createUserIfNotExists(channel, nick, ident, host string) {
	channel = strings.ToLower(channel)
	s.createChanIfNotExists(channel)

	s.m.Lock()
	if _, ok := s.channels[channel].users[nick]; !ok {
		s.channels[channel].users[nick] = &User{
			Nick:      nick,
			Ident:     ident,
			Host:      host,
			FirstSeen: time.Now(),
		}
	}
	s.m.Unlock()
}

// deleteUser removes the user from channel state.
func (s *state) deleteUser(nick string) {
	s.m.Lock()
	for k := range s.channels {
		// Check to see if they're in this channel.
		if _, ok := s.channels[k].users[nick]; !ok {
			continue
		}

		delete(s.channels[k].users, nick)
	}
	s.m.Unlock()
}

// renameUser renames the user in state, in all locations where relevant.
func (s *state) renameUser(from, to string) {
	s.m.Lock()
	defer s.m.Unlock()

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
