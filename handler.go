// Copyright (c) Liam Stanley <me@liamstanley.io>. All rights reserved. Use
// of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package girc

import (
	"fmt"
	"log"
	"math/rand"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/orcaman/concurrent-map"
)

// RunHandlers manually runs handlers for a given event.
func (c *Client) RunHandlers(event *Event) {
	if event == nil {
		c.debug.Print("nil event")
		return
	}

	// Log the event.
	prefix := "< "
	if event.Echo {
		prefix += "[echo-message] "
	}
	c.debug.Print(prefix + StripRaw(event.String()))
	if c.Config.Out != nil {
		if pretty, ok := event.Pretty(); ok {
			fmt.Fprintln(c.Config.Out, StripRaw(pretty))
		}
	}

	// Background handlers first. If the event is an echo-message, then only
	// send the echo version to ALL_EVENTS.
	c.Handlers.exec(ALL_EVENTS, true, c, event.Copy())
	if !event.Echo {
		c.Handlers.exec(event.Command, true, c, event.Copy())
	}

	c.Handlers.exec(ALL_EVENTS, false, c, event.Copy())

	if !event.Echo {
		c.Handlers.exec(event.Command, false, c, event.Copy())
	}
	// Check if it's a CTCP.
	if ctcp := DecodeCTCP(event.Copy()); ctcp != nil {
		// Execute it.
		c.CTCP.call(c, ctcp)
	}

}

// Handler is lower level implementation of a handler. See
// Caller.AddHandler()
type Handler interface {
	Execute(*Client, Event)
}

// HandlerFunc is a type that represents the function necessary to
// implement Handler.
type HandlerFunc func(client *Client, event Event)

// Execute calls the HandlerFunc with the sender and irc message.
func (f HandlerFunc) Execute(client *Client, event Event) {
	f(client, event)
}

// nestedHandlers consists of a nested concurrent map.
//
// ( cmap.ConcurrentMap[command]cmap.ConcurrentMap[cuid]Handler )
//
// command and cuid are both strings.
type nestedHandlers struct {
	cm cmap.ConcurrentMap
}

type handlerTuple struct {
	cuid    string
	handler Handler
}

func newNestedHandlers() *nestedHandlers {
	return &nestedHandlers{cm: cmap.New()}
}

func (nest *nestedHandlers) len() (total int) {
	for hs := range nest.cm.IterBuffered() {
		hndlrs := hs.Val.(cmap.ConcurrentMap)
		total += len(hndlrs.Keys())
	}
	return
}

func (nest *nestedHandlers) lenFor(cmd string) (total int) {
	cmd = strings.ToUpper(cmd)
	hs, ok := nest.cm.Get(cmd)
	if !ok {
		return 0
	}
	hndlrs := hs.(cmap.ConcurrentMap)
	return len(hndlrs.Keys())
}

func (nest *nestedHandlers) getAllHandlersFor(s string) (handlers chan handlerTuple, ok bool) {
	var h interface{}
	h, ok = nest.cm.Get(s)
	if !ok {
		return
	}
	hm := h.(cmap.ConcurrentMap)
	handlers = make(chan handlerTuple, 5)
	go func() {
		for hi := range hm.IterBuffered() {
			ht := handlerTuple{
				hi.Key,
				hi.Val.(Handler),
			}
			handlers <- ht
		}
	}()
	return
}

// Caller manages internal and external (user facing) handlers.
type Caller struct {
	// mu is the mutex that should be used when accessing handlers.
	mu sync.RWMutex

	parent *Client

	// external/internal keys are of structure:
	//   map[COMMAND][CUID]Handler
	// Also of note: "COMMAND" should always be uppercase for normalization.

	// external is a map of user facing handlers.
	external *nestedHandlers
	// external map[string]map[string]Handler
	// internal is a map of internally used handlers for the client.
	internal *nestedHandlers
	// debug is the clients logger used for debugging.
	debug *log.Logger
}

// newCaller creates and initializes a new handler.
func newCaller(parent *Client, debugOut *log.Logger) *Caller {
	c := &Caller{
		external: newNestedHandlers(),
		internal: newNestedHandlers(),
		debug:    debugOut,
		parent:   parent,
	}

	return c
}

// Len returns the total amount of user-entered registered handlers.
func (c *Caller) Len() int {
	return c.external.len()
}

// Count is much like Caller.Len(), however it counts the number of
// registered handlers for a given command.
func (c *Caller) Count(cmd string) int {
	return c.external.lenFor(cmd)
}

func (c *Caller) String() string {
	return fmt.Sprintf("<Caller external:%d internal:%d>", c.Len(), c.internal.len())
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// cuid generates a unique UID string for each handler for ease of removal.
func (c *Caller) cuid(cmd string, n int) (cuid, uid string) {
	b := make([]byte, n)

	for i := range b {
		b[i] = letterBytes[rand.Int63()%int64(len(letterBytes))]
	}

	return cmd + ":" + string(b), string(b)
}

// cuidToID allows easy mapping between a generated cuid and the caller
// external/internal handler maps.
func (c *Caller) cuidToID(input string) (cmd, uid string) {
	i := strings.IndexByte(input, ':')
	if i < 0 {
		return "", ""
	}

	return input[:i], input[i+1:]
}

// exec executes all handlers pertaining to specified event. Internal first,
// then external.
//
// Please note that there is no specific order/priority for which the handlers
// are executed.
func (c *Caller) exec(command string, bg bool, client *Client, event *Event) {
	handle := func(wgr *sync.WaitGroup, h handlerTuple) {

		c.debug.Printf("(%s) exec %s => %s", c.parent.Config.Nick, command, h.cuid)
		start := time.Now()

		if bg {
			go func() {
				defer wgr.Done()
				if client.Config.RecoverFunc != nil {
					defer recoverHandlerPanic(client, event, h.cuid, 3)
				}
				h.handler.Execute(client, *event)
				c.debug.Printf("(%s) done %s == %s", c.parent.Config.Nick,
					h.cuid, time.Since(start))
			}()
			return
		}

		if client.Config.RecoverFunc != nil {
			defer recoverHandlerPanic(client, event, h.cuid, 3)
		}

		h.handler.Execute(client, *event)
		c.debug.Printf("(%s) done %s == %s", c.parent.Config.Nick, h.cuid, time.Since(start))
		wgr.Done()
	}

	// Run all handlers concurrently across the same event. This should
	// still help prevent mis-ordered events, while speeding up the
	// execution speed.
	var wg sync.WaitGroup

	internals, iok := c.internal.getAllHandlersFor(command)
	if iok {
		for h := range internals {
			wg.Add(1)
			go handle(&wg, h)
		}
	}
	externals, eok := c.external.getAllHandlersFor(command)
	if eok {
		for h := range externals {
			wg.Add(1)
			go handle(&wg, h)
		}
	}

	// Wait for all of the handlers to complete. Not doing this may cause
	// new events from becoming ahead of older handlers.
	c.debug.Printf("(%s) wg.Wait()", c.parent.Config.Nick)
	wg.Wait()
}

// ClearAll clears all external handlers currently setup within the client.
// This ignores internal handlers.
func (c *Caller) ClearAll() {
	c.external.cm.Clear()
	c.debug.Print("cleared all external handlers")
}

// clearInternal clears all internal handlers currently setup within the
// client.
func (c *Caller) clearInternal() {
	c.internal.cm.Clear()
	c.debug.Print("cleared all internal handlers")
}

// Clear clears all of the handlers for the given event.
// This ignores internal handlers.
func (c *Caller) Clear(cmd string) {
	cmd = strings.ToUpper(cmd)
	c.external.cm.Remove(cmd)
	c.debug.Printf("(%s) cleared external handlers for %s", c.parent.Config.Nick, cmd)
}

// Remove removes the handler with cuid from the handler stack. success
// indicates that it existed, and has been removed. If not success, it
// wasn't a registered handler.
func (c *Caller) Remove(cuid string) (success bool) {
	c.mu.Lock()
	success = c.remove(cuid)
	c.mu.Unlock()

	return success
}

// remove is much like Remove, however is NOT concurrency safe. Lock Caller.mu
// on your own.
func (c *Caller) remove(cuid string) (ok bool) {
	cmd, uid := c.cuidToID(cuid)
	if len(cmd) == 0 || len(uid) == 0 {
		return false
	}

	// Check if the irc command/event has any handlers on it.
	var h interface{}
	h, ok = c.external.cm.Get(cmd)
	if !ok {
		return
	}

	hs := h.(cmap.ConcurrentMap)

	// Check to see if it's actually a registered handler.
	if _, ok = hs.Get(cuid); !ok {
		return
	}

	hs.Remove(uid)
	c.debug.Printf("removed handler %s", cuid)

	// Assume success.
	return true
}

// sregister is much like Caller.register(), except that it safely locks
// the Caller mutex.
func (c *Caller) sregister(internal, bg bool, cmd string, handler Handler) (cuid string) {
	cuid = c.register(internal, bg, cmd, handler)
	return cuid
}

// register will register a handler in the internal tracker. Unsafe (you
// must lock c.mu yourself!)
func (c *Caller) register(internal, bg bool, cmd string, handler Handler) (cuid string) {
	var uid string

	cmd = strings.ToUpper(cmd)

	cuid, uid = c.cuid(cmd, 20)
	if bg {
		uid += ":bg"
		cuid += ":bg"
	}

	var (
		parent    *nestedHandlers
		chandlers cmap.ConcurrentMap
		ei        interface{}
		ok        bool
	)

	if internal {
		parent = c.internal
	} else {
		parent = c.external
	}

	ei, ok = parent.cm.Get(cmd)

	if ok {
		chandlers = ei.(cmap.ConcurrentMap)
	} else {
		chandlers = cmap.New()
	}
	parent.cm.SetIfAbsent(cmd, chandlers)

	chandlers.Set(uid, handler)

	_, file, line, _ := runtime.Caller(2)

	c.debug.Printf("reg %q => %s [int:%t bg:%t] %s:%d", uid, cmd, internal, bg, file, line)

	return cuid
}

// AddHandler registers a handler (matching the handler interface) for the
// given event. cuid is the handler uid which can be used to remove the
// handler with Caller.Remove().
func (c *Caller) AddHandler(cmd string, handler Handler) (cuid string) {
	return c.sregister(false, false, cmd, handler)
}

// Add registers the handler function for the given event. cuid is the
// handler uid which can be used to remove the handler with Caller.Remove().
func (c *Caller) Add(cmd string, handler func(client *Client, event Event)) (cuid string) {
	return c.sregister(false, false, cmd, HandlerFunc(handler))
}

// AddBg registers the handler function for the given event and executes it
// in a go-routine. cuid is the handler uid which can be used to remove the
// handler with Caller.Remove().
func (c *Caller) AddBg(cmd string, handler func(client *Client, event Event)) (cuid string) {
	return c.sregister(false, true, cmd, HandlerFunc(handler))
}

// AddTmp adds a "temporary" handler, which is good for one-time or few-time
// uses. This supports a deadline and/or manual removal, as this differs
// much from how normal handlers work. An example of a good use for this
// would be to capture the entire output of a multi-response query to the
// server. (e.g. LIST, WHOIS, etc)
//
// The supplied handler is able to return a boolean, which if true, will
// remove the handler from the handler stack.
//
// Additionally, AddTmp has a useful option, deadline. When set to greater
// than 0, deadline will be the amount of time that passes before the handler
// is removed from the stack, regardless of if the handler returns true or not.
// This is useful in that it ensures that the handler is cleaned up if the
// server does not respond appropriately, or takes too long to respond.
//
// Note that handlers supplied with AddTmp are executed in a goroutine to
// ensure that they are not blocking other handlers. However, if you are
// creating a temporary handler from another handler, it should be a
// background handler.
//
// Use cuid with Caller.Remove() to prematurely remove the handler from the
// stack, bypassing the timeout or waiting for the handler to return that it
// wants to be removed from the stack.
func (c *Caller) AddTmp(cmd string, deadline time.Duration, handler func(client *Client, event Event) bool) (cuid string, done chan struct{}) {
	done = make(chan struct{})

	cuid = c.sregister(false, true, cmd, HandlerFunc(func(client *Client, event Event) {
		remove := handler(client, event)
		if remove {
			if ok := c.Remove(cuid); ok {
				close(done)
			}
		}
	}))

	if deadline > 0 {
		go func() {
			select {
			case <-time.After(deadline):
			case <-done:
			}

			if ok := c.Remove(cuid); ok {
				close(done)
			}
		}()
	}

	return cuid, done
}

// recoverHandlerPanic is used to catch all handler panics, and re-route
// them if necessary.
func recoverHandlerPanic(client *Client, event *Event, id string, skip int) {
	perr := recover()
	if perr == nil {
		return
	}

	var file, function string
	var line int
	var ok bool

	var pcs [10]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(skip, pcs[:])])
	for {
		frame, _ := frames.Next()
		file = frame.File
		line = frame.Line
		function = frame.Function

		break
	}

	err := &HandlerError{
		Event:  *event,
		ID:     id,
		File:   file,
		Line:   line,
		Func:   function,
		Panic:  perr,
		Stack:  debug.Stack(),
		callOk: ok,
	}

	client.Config.RecoverFunc(client, err)
}

// HandlerError is the error returned when a panic is intentionally recovered
// from. It contains useful information like the handler identifier (if
// applicable), filename, line in file where panic occurred, the call
// trace, and original event.
type HandlerError struct {
	Event  Event       // Event is the event that caused the error.
	ID     string      // ID is the CUID of the handler.
	File   string      // File is the file from where the panic originated.
	Line   int         // Line number where panic originated.
	Func   string      // Function name where panic originated.
	Panic  interface{} // Panic is the error that was passed to panic().
	Stack  []byte      // Stack is the call stack. Note you may have to skip 1 or 2 due to debug functions.
	callOk bool
}

// Error returns a prettified version of HandlerError, containing ID, file,
// line, and basic error string.
func (e *HandlerError) Error() string {
	if e.callOk {
		return fmt.Sprintf("panic during handler [%s] execution in %s:%d: %s", e.ID, e.File, e.Line, e.Panic)
	}

	return fmt.Sprintf("panic during handler [%s] execution in unknown: %s", e.ID, e.Panic)
}

// String returns the error that panic returned, as well as the entire call
// trace of where it originated.
func (e *HandlerError) String() string {
	return fmt.Sprintf("panic: %s\n\n%s", e.Panic, string(e.Stack))
}

// DefaultRecoverHandler can be used with Config.RecoverFunc as a default
// catch-all for panics. This will log the error, and the call trace to the
// debug log (see Config.Debug), or os.Stdout if Config.Debug is unset.
func DefaultRecoverHandler(client *Client, err *HandlerError) {
	if client.Config.Debug == nil {
		fmt.Println(err.Error())
		fmt.Println(err.String())
		return
	}

	client.debug.Println(err.Error())
	client.debug.Println(err.String())
}
