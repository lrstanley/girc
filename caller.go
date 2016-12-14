// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
)

// RunCallbacks manually runs callbacks for a given event.
func (c *Client) RunCallbacks(event *Event) {
	// Log the event.
	c.log.Print("<-- " + event.Raw())

	// Regular wildcard callbacks.
	c.Callbacks.exec(ALLEVENTS, c, event)

	// Then regular non-threaded callbacks.
	c.Callbacks.exec(event.Command, c, event)
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
//   map[CALLBACK_TYPE][COMMAND][CUID]Callback
//
// Also of note: "COMMAND" should always be uppercase for normalization.
type Caller struct {
	// mu is the mutex that should be used when accessing callbacks.
	mu sync.RWMutex
	// external is a map of user facing callbacks.
	external map[string]map[string]map[string]Callback
	// internal is a map of internally used callbacks for the client.
	internal map[string]map[string]map[string]Callback
}

// newCaller creates and initializes a new callback handler.
func newCaller() *Caller {
	c := &Caller{}

	c.external = map[string]map[string]map[string]Callback{}
	c.external["routine"] = map[string]map[string]Callback{}
	c.external["std"] = map[string]map[string]Callback{}
	c.internal = map[string]map[string]map[string]Callback{}
	c.internal["routine"] = map[string]map[string]Callback{}
	c.internal["std"] = map[string]map[string]Callback{}

	return c
}

// Len returns the total amount of user-entered registered callbacks.
func (c *Caller) Len() int {
	var total int

	c.mu.RLock()
	for ctype := range c.external {
		for command := range c.external[ctype] {
			total += len(c.external[ctype][command])
		}
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
	for ctype := range c.external {
		for command := range c.external[ctype] {
			if command == cmd {
				total += len(c.external[ctype][command])
			}
		}
	}
	c.mu.RUnlock()

	return total
}

func (c *Caller) String() string {
	var total int
	var ctypes []string

	c.mu.RLock()
	for ctype := range c.internal {
		ctypes = append(ctypes, ctype)
		for cmd := range c.internal[ctype] {
			total += len(c.internal[ctype][cmd])
		}
	}
	c.mu.RUnlock()

	return fmt.Sprintf(
		"<Caller() types[%d]:[%s] client:%d internal:%d>",
		len(c.external), strings.Join(ctypes, ","), c.Len(), len(c.internal),
	)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// cuid generates a unique UID string for each callback for ease of removal.
func (c *Caller) cuid(ctype, cmd string, n int) (cuid, uid string) {
	b := make([]byte, n)

	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}

	return ctype + ":" + cmd + ":" + string(b), string(b)
}

// cuidToID allows easy mapping between a generated cuid and the caller``
// external/internal callback maps.
func (c *Caller) cuidToID(input string) (ctype, cmd, uid string) {
	// Ignore the errors because the strings will default to empty anyway.
	_, _ = fmt.Sscanf(input, "%s:%s:%s", &ctype, &cmd, &uid)
	return ctype, cmd, uid
}

// exec executes all callbacks pertaining to specified event. Internal first,
// then external.
//
// Please note that there is no specific order/priority for which the
// callback types themselves or the callbacks are executed.
func (c *Caller) exec(command string, client *Client, event *Event) {
	var execstack []Callback
	var routinestack []Callback

	c.mu.RLock()
	// Execute internal callbacks first.
	for callbackType := range c.internal {
		if _, ok := c.internal[callbackType][command]; !ok {
			continue
		}

		for cuid := range c.internal[callbackType][command] {
			switch callbackType {
			case "routine":
				routinestack = append(routinestack, c.internal[callbackType][command][cuid])
			default:
				execstack = append(execstack, c.internal[callbackType][command][cuid])
			}
		}
	}

	// Aaand then external callbacks.
	for callbackType := range c.external {
		if _, ok := c.external[callbackType][command]; !ok {
			continue
		}

		for cuid := range c.external[callbackType][command] {
			switch callbackType {
			case "routine":
				routinestack = append(routinestack, c.external[callbackType][command][cuid])
			default:
				execstack = append(execstack, c.external[callbackType][command][cuid])
			}
		}
	}
	c.mu.RUnlock()

	for i := 0; i < len(routinestack); i++ {
		routinestack[i].Execute(client, *event)
	}

	for i := 0; i < len(execstack); i++ {
		execstack[i].Execute(client, *event)
	}
}

// ClearAll clears all external callbacks currently setup within the client.
// This ignores internal callbacks.
func (c *Caller) ClearAll() {
	c.mu.Lock()
	c.external = map[string]map[string]map[string]Callback{}
	c.external["routine"] = map[string]map[string]Callback{}
	c.external["std"] = map[string]map[string]Callback{}
	c.mu.Unlock()
}

// Clear clears all of the callbacks for the given event.
// This ignores internal callbacks.
func (c *Caller) Clear(cmd string) {
	cmd = strings.ToUpper(cmd)

	c.mu.Lock()
	for ctype := range c.external {
		if _, ok := c.external[ctype][cmd]; ok {
			delete(c.external[ctype], cmd)
		}
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
	ctype, cmd, uid := c.cuidToID(cuid)
	if len(ctype) == 0 || len(cmd) == 0 || len(uid) == 0 {
		return false
	}

	// Check if the callback type exists.
	if _, ok := c.external[ctype]; !ok {
		return false
	}

	// Check if the irc command/event has any callbacks on it.
	if _, ok := c.external[ctype][cmd]; !ok {
		return false
	}

	// Check to see if it's actually a registered callback.
	if _, ok := c.external[ctype][cmd][cuid]; !ok {
		return false
	}

	delete(c.external[ctype][cmd], uid)

	// Assume success.
	return true
}

// sregister is much like Caller.register(), except that it safely locks
// the Caller mutex.
func (c *Caller) sregister(internal bool, ctype, cmd string, callback Callback) (cuid string) {
	c.mu.Lock()
	cuid = c.register(internal, ctype, cmd, callback)
	c.mu.Unlock()

	return cuid
}

// register will register a callback in the internal tracker. Unsafe (you
// must lock c.mu yourself!)
func (c *Caller) register(internal bool, ctype, cmd string, callback Callback) (cuid string) {
	var uid string

	cmd = strings.ToUpper(cmd)

	if internal {
		if _, ok := c.internal[ctype]; !ok {
			panic(errors.New("callback type does not exist: " + ctype))
		}

		if _, ok := c.internal[ctype][cmd]; !ok {
			c.internal[ctype][cmd] = map[string]Callback{}
		}

		cuid, uid = c.cuid(ctype, cmd, 20)

		c.internal[ctype][cmd][uid] = callback
	} else {
		if _, ok := c.external[ctype]; !ok {
			panic(errors.New("callback type does not exist: " + ctype))
		}

		if _, ok := c.external[ctype][cmd]; !ok {
			c.external[ctype][cmd] = map[string]Callback{}
		}

		cuid, uid = c.cuid(ctype, cmd, 20)

		c.external[ctype][cmd][uid] = callback
	}

	return cuid
}

// AddHandler registers a callback (matching the Callback interface) for the
// given event. cuid is the callback uid which can be used to remove the
// callback with Caller.Remove().
func (c *Caller) AddHandler(cmd string, callback Callback) (cuid string) {
	return c.sregister(false, "std", cmd, callback)
}

// AddBgHandler registers a callback (matching the Callback interface) for
// the given event and executes it in a go-routine. cuid is the callback uid
// which can be used to remove the callback with Caller.Remove().
func (c *Caller) AddBgHandler(cmd string, callback Callback) (cuid string) {
	return c.sregister(false, "routine", cmd, callback)
}

// Add registers the callback function for the given event. cuid is the
// callback uid which can be used to remove the callback with Caller.Remove().
func (c *Caller) Add(cmd string, callback func(c *Client, e Event)) (cuid string) {
	return c.sregister(false, "std", cmd, CallbackFunc(callback))
}

// AddBg registers the callback function for the given event and executes it
// in a go-routine. cuid is the callback uid which can be used to remove the
// callback with Caller.Remove().
func (c *Caller) AddBg(cmd string, callback func(c *Client, e Event)) (cuid string) {
	return c.sregister(false, "routine", cmd, CallbackFunc(callback))
}
