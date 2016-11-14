// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

// handleEvent runs the necessary callbacks for the incoming event
func (c *Client) handleEvent(event *Event) {
	// Log the event.
	c.log.Print("<-- " + event.String())

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
	// with "routine_". E.g. "routine_JOIN".
	if callbacks, ok := c.callbacks["routine_"+event.Command]; ok {
		for i := 0; i < len(callbacks); i++ {
			go callbacks[i].Execute(c, *event)
		}
	}
}

// ClearCallbacks clears all callbacks currently setup within the
// client.
func (c *Client) ClearCallbacks() {
	// registerHelpers should clean all callbacks and setup internal
	// ones as necessary.
	c.registerHelpers()
}

// RunCallbacks manually runs callbacks for a given event.
func (c *Client) RunCallbacks(event *Event) {
	c.handleEvent(event)
}

// AddCallbackHandler registers a callback (matching the Callback
// interface) for the given command.
func (c *Client) AddCallbackHandler(cmd string, callback Callback) {
	c.callbacks[cmd] = append(c.callbacks[cmd], callback)
}

// AddCallback registers the callback function for the given command.
func (c *Client) AddCallback(cmd string, callback func(c *Client, e Event)) {
	c.callbacks[cmd] = append(c.callbacks[cmd], CallbackFunc(callback))
}

// AddBgCallback registers the callback function for the given command
// and executes it in a go-routine.
//
// Runs after all other callbacks have been ran.
func (c *Client) AddBgCallback(cmd string, callback func(c *Client, e Event)) {
	c.callbacks["routine_"+cmd] = append(c.callbacks["routine_"+cmd], CallbackFunc(callback))
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
