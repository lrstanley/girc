package girc

import (
	"strings"
	"sync"
	"time"
)

// TODO: conntime, uptime

// State represents the actively-changing variables within the client runtime
type State struct {
	m         sync.RWMutex        // lock, primarily used for writing things in state
	connected bool                // if we're connected to the server or not
	nick      string              // internal tracker for our nickname
	channels  map[string]*Channel // map of channels that the client is in
}

// User represents an IRC user and the state attached to them
type User struct {
	Nick      string    // nickname of the user
	Ident     string    // ident (often referred to as "user") of the user
	Host      string    // host that server is providing for the user, may not always be accurate
	FirstSeen time.Time // the first time they were seen by the client
}

// Channel represents an IRC channel and the state attached to it
type Channel struct {
	// TODO: users needs to be exposed
	Name   string // name of the channel, always lowercase
	users  map[string]*User
	Joined time.Time // when the channel was joined
}

// NewState returns a clean state
func NewState() *State {
	s := &State{}

	s.channels = make(map[string]*Channel)
	s.connected = false

	return s
}

// createChanIfNotExists creates the channel in state, if not already done
func (s *State) createChanIfNotExists(channel string) {
	channel = strings.ToLower(channel)

	// not a valid channel
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

// deleteChannel removes the channel from state, if not already done
func (s *State) deleteChannel(channel string) {
	channel = strings.ToLower(channel)
	s.createChanIfNotExists(channel)

	s.m.Lock()
	if _, ok := s.channels[channel]; ok {
		delete(s.channels, channel)
	}
	s.m.Unlock()
}

// createUserIfNotExists creates the channel and user in state,
// if not already done
func (s *State) createUserIfNotExists(channel, nick, ident, host string) {
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

// deleteUser removes the user from channel state
func (s *State) deleteUser(nick string) {
	s.m.Lock()
	for k := range s.channels {
		// check to see if they're in this channel
		if _, ok := s.channels[k].users[nick]; !ok {
			continue
		}

		delete(s.channels[k].users, nick)
	}
	s.m.Unlock()
}

// renameUser renames the user in state, in all locations where
// relevant
func (s *State) renameUser(from, to string) {
	s.m.Lock()
	defer s.m.Unlock()

	for k := range s.channels {
		// check to see if they're in this channel
		if _, ok := s.channels[k].users[from]; !ok {
			continue
		}

		// take the actual reference to the pointer
		source := *s.channels[k].users[from]

		// update the nick field (as we not only have a key, but a
		// matching struct field)
		source.Nick = to

		// delete the old
		delete(s.channels[k].users, from)

		// in with the new
		s.channels[k].users[to] = &source
	}
}
