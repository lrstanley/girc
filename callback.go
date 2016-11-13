package girc

// TODO: ClearCallback(code string)

// handleEvent runs the necessary callbacks for the incoming event
func (c *Client) handleEvent(event *Event) {
	// log the event
	c.log.Print(event.String())

	// wildcard callbacks first
	if callbacks, ok := c.callbacks[ALLEVENTS]; ok {
		for i := 0; i < len(callbacks); i++ {
			callbacks[i].Execute(c, *event)
		}
	}

	// regular non-threaded callbacks
	if callbacks, ok := c.callbacks[event.Command]; ok {
		for i := 0; i < len(callbacks); i++ {
			callbacks[i].Execute(c, *event)
		}
	}

	// callbacks that should be ran concurrently
	// callbacks which should be ran in a go-routine should be prefixed with
	// "routine_". e.g. "routine_JOIN".
	if callbacks, ok := c.callbacks["routine_"+event.Command]; ok {
		for i := 0; i < len(callbacks); i++ {
			go callbacks[i].Execute(c, *event)
		}
	}
}

// ClearCallbacks clears all callbacks currently setup within the client
func (c *Client) ClearCallbacks() {
	// registerHelpers should clean all callbacks and setup internal ones
	// as necessary.
	c.registerHelpers()
}

// AddCallbackHandler registers the callback for the given command
func (c *Client) AddCallbackHandler(cmd string, callback Callback) {
	c.callbacks[cmd] = append(c.callbacks[cmd], callback)
}

// AddCallback registers the callback function for the given command
func (c *Client) AddCallback(cmd string, callback func(c *Client, e Event)) {
	c.callbacks[cmd] = append(c.callbacks[cmd], CallbackFunc(callback))
}

// AddBgCallback registers the callback function for the given command
// and executes it in a go-routine, after all other callbacks have been ran
func (c *Client) AddBgCallback(cmd string, callback func(c *Client, e Event)) {
	c.callbacks["routine_"+cmd] = append(c.callbacks["routine_"+cmd], CallbackFunc(callback))
}

// Callback is an interface to handle IRC events
type Callback interface {
	Execute(*Client, Event)
}

// CallbackFunc is a type that represents the function necessary to implement Callback
type CallbackFunc func(c *Client, e Event)

// Execute calls the CallbackFunc with the sender and irc message
func (f CallbackFunc) Execute(c *Client, e Event) {
	f(c, e)
}
