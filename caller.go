// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
)

// RunCallbacks manually runs callbacks for a given event.
func (c *Client) RunCallbacks(event *Event) {
	// Log the event.
	c.log.Print("<-- " + StripRaw(event.Raw()))

	// Regular wildcard callbacks.
	c.Callbacks.exec(ALLEVENTS, c, event)

	// Then regular callbacks.
	c.Callbacks.exec(event.Command, c, event)

	// Check if it's a CTCP.
	if ctcp := decodeCTCP(event); ctcp != nil {
		// Execute it.
		c.CTCP.call(ctcp, c)
	}
}

// Callback is lower level implementation of a callback. See
// Caller.AddHandler()
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

// Caller manages internal and external (user facing) callbacks.
//
// external/internal keys are of structure:
//   map[COMMAND][CUID]Callback
//
// Also of note: "COMMAND" should always be uppercase for normalization.
type Caller struct {
	// mu is the mutex that should be used when accessing callbacks.
	mu sync.RWMutex
	// wg is the waitgroup which is used to execute all callbacks concurrently.
	wg sync.WaitGroup
	// external is a map of user facing callbacks.
	external map[string]map[string]Callback
	// internal is a map of internally used callbacks for the client.
	internal map[string]map[string]Callback
}

// newCaller creates and initializes a new callback handler.
func newCaller() *Caller {
	c := &Caller{
		external: map[string]map[string]Callback{},
		internal: map[string]map[string]Callback{},
	}

	return c
}

// Len returns the total amount of user-entered registered callbacks.
func (c *Caller) Len() int {
	var total int

	c.mu.RLock()
	for command := range c.external {
		total += len(c.external[command])
	}
	c.mu.RUnlock()

	return total
}

// Count is much like Caller.Len(), however it counts the number of
// registered callbacks for a given command.
func (c *Caller) Count(cmd string) int {
	var total int

	cmd = strings.ToUpper(cmd)

	c.mu.RLock()
	for command := range c.external {
		if command == cmd {
			total += len(c.external[command])
		}
	}
	c.mu.RUnlock()

	return total
}

func (c *Caller) String() string {
	var total int

	c.mu.RLock()
	for cmd := range c.internal {
		total += len(c.internal[cmd])
	}
	c.mu.RUnlock()

	return fmt.Sprintf("<Caller() external:%d internal:%d>", c.Len(), total)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// cuid generates a unique UID string for each callback for ease of removal.
func (c *Caller) cuid(cmd string, n int) (cuid, uid string) {
	b := make([]byte, n)

	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}

	return cmd + ":" + string(b), string(b)
}

// cuidToID allows easy mapping between a generated cuid and the caller
// external/internal callback maps.
func (c *Caller) cuidToID(input string) (cmd, uid string) {
	// Ignore the errors because the strings will default to empty anyway.
	_, _ = fmt.Sscanf(input, "%s:%s", &cmd, &uid)
	return cmd, uid
}

// exec executes all callbacks pertaining to specified event. Internal first,
// then external.
//
// Please note that there is no specific order/priority for which the
// callback types themselves or the callbacks are executed.
func (c *Caller) exec(command string, client *Client, event *Event) {
	// Build a stack of callbacks which can be executed concurrently.
	var execstack []Callback

	c.mu.RLock()
	// Get internal callbacks first.
	if _, ok := c.internal[command]; ok {
		for cuid := range c.internal[command] {
			execstack = append(execstack, c.internal[command][cuid])
		}
	}

	// Aaand then external callbacks.
	if _, ok := c.external[command]; ok {
		for cuid := range c.external[command] {
			execstack = append(execstack, c.external[command][cuid])
		}
	}
	c.mu.RUnlock()

	// Run all callbacks concurrently across the same event. This should
	// still help prevent mis-ordered events, while speeding up the
	// execution speed.
	c.wg.Add(len(execstack))
	for i := 0; i < len(execstack); i++ {
		go func(index int) {
			execstack[index].Execute(client, *event)
			c.wg.Done()
		}(i)
	}

	// Wait for all of the callbacks to complete. Not doing this may cause
	// new events from becoming ahead of older callbacks.
	c.wg.Wait()
}

// ClearAll clears all external callbacks currently setup within the client.
// This ignores internal callbacks.
func (c *Caller) ClearAll() {
	c.mu.Lock()
	c.external = map[string]map[string]Callback{}
	c.mu.Unlock()
}

// Clear clears all of the callbacks for the given event.
// This ignores internal callbacks.
func (c *Caller) Clear(cmd string) {
	cmd = strings.ToUpper(cmd)

	c.mu.Lock()
	if _, ok := c.external[cmd]; ok {
		delete(c.external, cmd)
	}
	c.mu.Unlock()
}

// Remove removes the callback with cuid from the callback stack. success
// indicates that it existed, and has been removed. If not success, it
// wasn't a registered callback.
func (c *Caller) Remove(cuid string) (success bool) {
	c.mu.Lock()
	success = c.remove(cuid)
	c.mu.Unlock()

	return success
}

// remove is much like Remove, however is NOT concurrency safe. Lock Caller.mu
// on your own.
func (c *Caller) remove(cuid string) (success bool) {
	cmd, uid := c.cuidToID(cuid)
	if len(cmd) == 0 || len(uid) == 0 {
		return false
	}

	// Check if the irc command/event has any callbacks on it.
	if _, ok := c.external[cmd]; !ok {
		return false
	}

	// Check to see if it's actually a registered callback.
	if _, ok := c.external[cmd][cuid]; !ok {
		return false
	}

	delete(c.external[cmd], uid)

	// Assume success.
	return true
}

// sregister is much like Caller.register(), except that it safely locks
// the Caller mutex.
func (c *Caller) sregister(internal bool, cmd string, callback Callback) (cuid string) {
	c.mu.Lock()
	cuid = c.register(internal, cmd, callback)
	c.mu.Unlock()

	return cuid
}

// register will register a callback in the internal tracker. Unsafe (you
// must lock c.mu yourself!)
func (c *Caller) register(internal bool, cmd string, callback Callback) (cuid string) {
	var uid string

	cmd = strings.ToUpper(cmd)

	if internal {
		if _, ok := c.internal[cmd]; !ok {
			c.internal[cmd] = map[string]Callback{}
		}

		cuid, uid = c.cuid(cmd, 20)
		c.internal[cmd][uid] = callback
	} else {
		if _, ok := c.external[cmd]; !ok {
			c.external[cmd] = map[string]Callback{}
		}

		cuid, uid = c.cuid(cmd, 20)
		c.external[cmd][uid] = callback
	}

	return cuid
}

// AddHandler registers a callback (matching the Callback interface) for the
// given event. cuid is the callback uid which can be used to remove the
// callback with Caller.Remove().
func (c *Caller) AddHandler(cmd string, callback Callback) (cuid string) {
	return c.sregister(false, cmd, callback)
}

// Add registers the callback function for the given event. cuid is the
// callback uid which can be used to remove the callback with Caller.Remove().
func (c *Caller) Add(cmd string, callback func(c *Client, e Event)) (cuid string) {
	return c.sregister(false, cmd, CallbackFunc(callback))
}

// AddBg registers the callback function for the given event and executes it
// in a go-routine. cuid is the callback uid which can be used to remove the
// callback with Caller.Remove().
func (c *Caller) AddBg(cmd string, callback func(c *Client, e Event)) (cuid string) {
	return c.sregister(false, cmd, CallbackFunc(func(c *Client, e Event) {
		go callback(c, e)
	}))
}
