// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"fmt"
	"strconv"
	"strings"
)

// handleEvent runs the necessary callbacks for the incoming event
func (c *Client) handleEvent(event *Event) {
	// Log the event.
	c.log.Print("<-- " + event.Raw())

	// Wildcard callbacks first.
	if callbacks, ok := c.callbacks[ALLEVENTS]; ok {
		for i := 0; i < len(callbacks); i++ {
			callbacks[i].Execute(c, *event)
		}
	}

	// Regular non-threaded callbacks.
	if callbacks, ok := c.callbacks[event.Command]; ok {
		for i := 0; i < len(callbacks); i++ {
			callbacks[i].Execute(c, *event)
		}
	}

	// Callbacks that should be ran concurrently.
	//
	// Callbacks which should be ran in a go-routine should be prefixed
	// with "routine:". E.g. "routine:JOIN".
	if callbacks, ok := c.callbacks["routine:"+event.Command]; ok {
		for i := 0; i < len(callbacks); i++ {
			go callbacks[i].Execute(c, *event)
		}
	}
}

// ClearAllCallbacks clears all callbacks currently setup within the
// client.
//
// This ignores internal callbacks for the client.
func (c *Client) ClearAllCallbacks() {
	// registerHelpers should clean all callbacks and setup internal
	// ones as necessary.
	c.registerHelpers()
}

// ClearCallbacks clears all of the callbacks for the given event.
//
// This ignores internal callbacks for the client.
func (c *Client) ClearCallbacks(cmd string) {
	for i := 0; i < len(c.callbacks[cmd]); i++ {
		isin := false
		for _, cb := range c.internalCallbacks {
			if fmt.Sprintf("%s:%d", cmd, i) == cb {
				isin = true
				break
			}
		}

		if !isin {
			c.RemoveCallback(fmt.Sprintf("%s:%d", cmd, i))
		}
	}

	for i := 0; i < len(c.callbacks["routine:"+cmd]); i++ {
		isin := false
		for _, cb := range c.internalCallbacks {
			if fmt.Sprintf("routine:%s:%d", cmd, i) == cb {
				isin = true
				break
			}
		}

		if !isin {
			c.RemoveCallback(fmt.Sprintf("routine:%s:%d", cmd, i))
		}
	}
}

// RemoveCallback removes the callback with id from the callback stack.
func (c *Client) RemoveCallback(id string) {
	var cmd string
	var index int

	parts := strings.Split(id, ":")
	if len(parts) < 2 {
		// needs to be at least CMD:INDEX
		return
	}

	cmd = strings.Join(parts[0:len(parts)-1], ":")
	index, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return
	}

	if len(c.callbacks[cmd]) > (index + 1) {
		// index doesn't look to exist
		return
	}

	c.cbMux.Lock()
	c.callbacks[cmd] = append(c.callbacks[cmd][:index], c.callbacks[cmd][index+1:]...)
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

// RunCallbacks manually runs callbacks for a given event.
func (c *Client) RunCallbacks(event *Event) {
	c.handleEvent(event)
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
