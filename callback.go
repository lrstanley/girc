// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"fmt"
	"strconv"
	"strings"
)

func (c *Client) execCallbacks(command string, event *Event) {
	if callbacks, ok := c.callbacks["routine:"+command]; ok {
		for i := 0; i < len(callbacks); i++ {
			if callbacks[i] == nil {
				continue
			}

			go callbacks[i].Execute(c, *event)
		}
	}

	if callbacks, ok := c.callbacks[command]; ok {
		for i := 0; i < len(callbacks); i++ {
			if callbacks[i] == nil {
				continue
			}

			callbacks[i].Execute(c, *event)
		}
	}
}

// RunCallbacks manually runs callbacks for a given event.
func (c *Client) RunCallbacks(event *Event) {
	// Log the event.
	c.log.Print("<-- " + event.Raw())

	// Regular wildcard callbacks.
	c.execCallbacks(ALLEVENTS, event)

	// Then regular non-threaded callbacks.
	c.execCallbacks(event.Command, event)
}

// ClearAllCallbacks clears all callbacks currently setup within the client.
//
// This ignores internal callbacks for the client.
func (c *Client) ClearAllCallbacks() {
	// registerHelpers should clean all callbacks and setup internal ones as
	// necessary.
	c.registerHelpers()
}

// ClearCallbacks clears all of the callbacks for the given event.
//
// This ignores internal callbacks for the client.
func (c *Client) ClearCallbacks(cmd string) {
	for i := 0; i < len(c.callbacks[cmd]); i++ {
		c.RemoveCallback(fmt.Sprintf("%s:%d", cmd, i))
	}

	for i := 0; i < len(c.callbacks["routine:"+cmd]); i++ {
		c.RemoveCallback(fmt.Sprintf("routine:%s:%d", cmd, i))
	}
}

// RemoveCallback removes the callback with id from the callback stack.
func (c *Client) RemoveCallback(id string) {
	// Check to see if it's an internal callback.
	for i := 0; i < len(c.internalCallbacks); i++ {
		if id == c.internalCallbacks[i] {
			return
		}
	}

	var cmd string
	var index int

	parts := strings.Split(id, ":")
	if len(parts) < 2 {
		// Needs to be at least CMD:INDEX.
		return
	}

	cmd = strings.Join(parts[0:len(parts)-1], ":")
	index, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return
	}

	if len(c.callbacks[cmd]) > (index + 1) {
		// Index doesn't look to exist.
		return
	}

	// Ignore if it's already been disabled.
	if c.callbacks[cmd][index] == nil {
		return
	}

	c.cbMux.Lock()
	c.callbacks[cmd][index] = nil
	c.cbMux.Unlock()
}

// trackIntCallback keeps track of all internally used callbacks.
func (c *Client) trackIntCallback(id string) {
	c.cbMux.Lock()
	c.internalCallbacks = append(c.internalCallbacks, id)
	c.cbMux.Unlock()
}

// untrackIntCallback keeps track of all internally used callbacks.
func (c *Client) untrackIntCallback(id string) {
	c.cbMux.Lock()
	for i := 0; i < len(c.internalCallbacks); i++ {
		if c.internalCallbacks[i] == id {
			c.internalCallbacks = append(c.internalCallbacks[:i], c.internalCallbacks[i+1:]...)
		}
	}
	c.cbMux.Unlock()
}

// AddCallbackHandler registers a callback (matching the Callback
// interface) for the given command.
func (c *Client) AddCallbackHandler(cmd string, callback Callback) (id string) {
	c.cbMux.Lock()
	c.callbacks[cmd] = append(c.callbacks[cmd], callback)
	id = fmt.Sprintf("%s:%d", cmd, len(c.callbacks[cmd])-1)
	c.cbMux.Unlock()

	return id
}

// AddCallback registers the callback function for the given command.
func (c *Client) AddCallback(cmd string, callback func(c *Client, e Event)) (id string) {
	c.cbMux.Lock()
	c.callbacks[cmd] = append(c.callbacks[cmd], CallbackFunc(callback))
	id = fmt.Sprintf("%s:%d", cmd, len(c.callbacks[cmd])-1)
	c.cbMux.Unlock()

	return id
}

// AddBgCallback registers the callback function for the given command
// and executes it in a go-routine.
//
// Runs after all other callbacks have been ran.
func (c *Client) AddBgCallback(cmd string, callback func(c *Client, e Event)) (id string) {
	return c.AddCallback("routine:"+cmd, callback)
}

// Callback is lower level implementation of Client.AddCallback().
type Callback interface {
	Execute(*Client, Event)
}

// CallbackFunc is a type that represents the function necessary to
// implement Callback.
type CallbackFunc func(c *Client, e Event)

// Execute calls the CallbackFunc with the sender and irc message.
func (f CallbackFunc) Execute(c *Client, e Event) {
	f(c, e)
}
