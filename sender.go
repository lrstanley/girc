// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

import "time"

// Sender is an interface for sending IRC messages.
type Sender interface {
	// Send sends the given message and returns any errors.
	Send(*Event) error
}

// serverSender is a barebones writer used as the default sender for all
// callbacks.
type serverSender struct {
	writer *ircEncoder
}

// Send write the specified event.
func (s serverSender) Send(event *Event) error {
	return s.writer.Encode(event)
}

// EventLimiter is a custom ticker which lets you rate limit sending events
// to a function (e.g. Client.Send()), with optional burst support. See
// NewEventLimiter() for more information.
type EventLimiter struct {
	tick     *time.Ticker
	throttle chan time.Time
	fn       func(*Event) error
}

// loop is used to read events from the internal time.Ticker.
func (el *EventLimiter) loop() {
	// This should exit itself once el.Stop() is called.
	for t := range el.tick.C {
		select {
		case el.throttle <- t:
		default:
		}
	}
}

// Stop closes the ticker, and prevents re-use of the EventLimiter. Use this
// to prevent EventLimiter from keeping unnecessary pointers in memory.
func (el *EventLimiter) Stop() {
	el.tick.Stop()
	el.fn = nil
}

// Send is the subtitute function used to send the event the the previously
// specified send function.
//
// This WILL panic if Stop() was already called on the EventLimiter.
func (el *EventLimiter) Send(event *Event) error {
	// Ensure nobody is sending to it once it's closed.
	if el.fn == nil {
		panic("attempted send on closed EventLimiter")
	}

	<-el.throttle
	return el.fn(event)
}

// SendAll sends a list of events to Send(). SendAll will return the first
// error it gets when attempting to Send() to the predefined Send function.
// It will not attempt to continue processing the list of events.
func (el *EventLimiter) SendAll(events ...*Event) error {
	for i := 0; i < len(events); i++ {
		if err := el.Send(events[i]); err != nil {
			return err
		}
	}

	return nil
}

// NewEventLimiter returns a NewEventLimiter which can be used to rate limit
// events being sent to a Send function. This does support bursting a
// certain amount of messages if there are less than burstCount.
//
// Ensure that Stop() is called on the returned EventLimiter, otherwise
// the limiter may keep unwanted pointers to data in memory.
func NewEventLimiter(burstCount int, rate time.Duration, eventFunc func(event *Event) error) *EventLimiter {
	limiter := &EventLimiter{
		tick:     time.NewTicker(rate),
		throttle: make(chan time.Time, burstCount),
		fn:       eventFunc,
	}

	// Push the ticket into the background. If you want to stop this, simply
	// use EventLimiter.Stop().
	go limiter.loop()

	return limiter
}
