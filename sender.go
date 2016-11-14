// Copyright 2016 Liam Stanley <me@liamstanley.io>. All rights reserved.
// Use of this source code is governed by the MIT license that can be
// found in the LICENSE file.

package girc

// Sender is an interface for sending IRC messages.
type Sender interface {
	// Send sends the given message and returns any errors.
	Send(*Event) error
}

// serverSender is a barebones writer used as the default sender for all
// callbacks.
type serverSender struct {
	writer *Encoder
}

// Send sends the specified event.
func (s serverSender) Send(event *Event) error {
	return s.writer.Encode(event)
}
